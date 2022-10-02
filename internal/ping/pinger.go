package ping

import (
	"fmt"
	"net"
	"sync"
	"time"

	rb "github.com/sector-f/failoverd/internal/ringbuffer"
	"github.com/vishvananda/netlink"
)

type Pinger struct {
	OnRecv func(gps GlobalProbeStats)

	pingFreqency time.Duration
	privileged   bool
	numSeconds   uint

	closeChan chan struct{}

	globalStats GlobalProbeStats
	probes      []Probe
	resolveMap  map[string]string // Maps client-specified destinations to resolved destinations

	stopProbeChan chan string
	stoppers      map[string]chan struct{} // Maps resolved destinations to channels which are used to stop running pings
	stopWG        sync.WaitGroup

	statTracker map[string]*rb.RingBuffer // Maps resolved destination address to ring buffer
	statCh      chan ProbeStats
	mu          sync.Mutex
}

func NewPinger(probes []Probe, options ...Option) (*Pinger, error) {
	resolvedProbes := make([]Probe, 0, len(probes))
	resolveMap := make(map[string]string)
	stoppers := make(map[string]chan struct{})

	for _, probe := range probes {
		// If specified source is not an address, treat it as a network interface name
		// and attempt to determine its address using netlink.
		if probe.Src != "" && net.ParseIP(probe.Src) == nil {
			link, err := netlink.LinkByName(probe.Src)
			if err != nil {
				return nil, fmt.Errorf("could not determine address of %s: %w", probe.Src, err)
			}

			addrs, err := netlink.AddrList(link, netlink.FAMILY_V4) // TODO: Handle IPv6?
			if err != nil {
				return nil, fmt.Errorf("could not determine address of %s: %w", probe.Src, err)
			}

			if len(addrs) == 0 {
				return nil, fmt.Errorf("interface has no addresses")
			}

			probe.Src = addrs[0].IP.String() // TODO: figure out if there's a better way to pick an address than just "use the first one"
		}

		addrs, err := net.LookupHost(probe.Dst)
		if err != nil {
			return nil, fmt.Errorf("could not resolve %s: %w", probe.Dst, err)
		}

		if len(addrs) == 0 {
			return nil, fmt.Errorf("no addresses returned for %s", probe.Dst)
		}

		resolvedProbes = append(resolvedProbes, Probe{Src: probe.Src, Dst: addrs[0]})
		stoppers[addrs[0]] = make(chan struct{})
		resolveMap[probe.Dst] = addrs[0]
	}

	gps := GlobalProbeStats{
		Stats: make(map[string]ProbeStats),

		probes:     resolvedProbes,
		resolveMap: resolveMap,
	}

	p := &Pinger{
		probes: resolvedProbes,

		closeChan:  make(chan struct{}),
		resolveMap: resolveMap,

		stopProbeChan: make(chan string),
		stoppers:      stoppers,
		stopWG:        sync.WaitGroup{},

		globalStats: gps,

		statTracker: make(map[string]*rb.RingBuffer),
		statCh:      make(chan ProbeStats),
		mu:          sync.Mutex{},
	}

	return p, nil
}

func (p *Pinger) Run() {
	if p.pingFreqency <= 0 {
		p.pingFreqency = 1 * time.Second
	}

	if p.numSeconds <= 0 {
		p.numSeconds = 10
	}

	for i := range p.probes {
		p.statTracker[p.probes[i].Dst] = rb.New(p.numSeconds)
		go p.probes[i].run(p.pingFreqency, p.privileged, p.statCh, p.stoppers[p.probes[i].Dst], &p.stopWG)
	}

	for {
		select {
		case dst := <-p.stopProbeChan:
			p.mu.Lock()

			resolvedAddr, ok := p.resolveMap[dst]
			if !ok {
				break
			}

			stopper, ok := p.stoppers[resolvedAddr]
			if !ok {
				break
			}

			stopper <- struct{}{}

			delete(p.resolveMap, dst)
			delete(p.stoppers, resolvedAddr)

			idx := 0
			for i, probe := range p.probes {
				if resolvedAddr == probe.Dst {
					idx = i
					break
				}
			}
			p.probes = append(p.probes[:idx], p.probes[idx+1:]...)

			p.globalStats.Remove(dst)

			p.mu.Unlock()
		case msg := <-p.statCh:
			p.mu.Lock()

			statTracker := p.statTracker[msg.Dst]
			statTracker.Insert(msg.Loss)

			stats := ProbeStats{
				Src:  msg.Src,
				Dst:  msg.Dst,
				Loss: statTracker.Average(),
			}

			p.globalStats.Stats[msg.Dst] = stats

			p.mu.Unlock()

			if p.OnRecv != nil {
				p.OnRecv(p.globalStats)
			}
		case <-p.closeChan:
			for _, ch := range p.stoppers {
				ch <- struct{}{}
			}

			return
		}
	}
}

func (p *Pinger) Stats() GlobalProbeStats {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.globalStats
}

func (p *Pinger) Stop() {
	p.closeChan <- struct{}{}
	p.stopWG.Wait()
}

func (p *Pinger) StopProbe(dst string) {
	p.stopProbeChan <- dst
}

type Option func(p *Pinger)

func WithPingFrequency(t time.Duration) Option {
	return func(p *Pinger) {
		p.pingFreqency = t
	}
}

func WithPrivileged(privileged bool) Option {
	return func(p *Pinger) {
		p.privileged = privileged
	}
}

func WithNumSeconds(n uint) Option {
	return func(p *Pinger) {
		p.numSeconds = n
	}
}

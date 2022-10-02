package ping

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	probing "github.com/prometheus-community/pro-bing"
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
	resolveMap  map[string]string        // Maps client-specified destinations to resolved destinations
	stoppers    map[string]chan struct{} // Maps resolved destinations to channels which are used to stop running pings
	stopWG      sync.WaitGroup

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
		stoppers[probe.Dst] = make(chan struct{})
		resolveMap[probe.Dst] = addrs[0]
	}

	gps := GlobalProbeStats{
		Stats: make(map[string]ProbeStats),

		probes:     resolvedProbes,
		resolveMap: resolveMap,
	}

	p := &Pinger{
		probes: resolvedProbes,

		closeChan: make(chan struct{}),
		stoppers:  stoppers,
		stopWG:    sync.WaitGroup{},

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

	for i, _ := range p.probes {
		p.statTracker[p.probes[i].Dst] = rb.New(p.numSeconds)
		go p.probes[i].run(p.pingFreqency, p.privileged, p.statCh, p.stoppers[p.probes[i].Dst], &p.stopWG)
	}

	for {
		select {
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

func (probe *Probe) run(pingFrequency time.Duration, privileged bool, statCh chan ProbeStats, stopChan chan struct{}, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()

	for {
		select {
		case <-stopChan:
			return
		default:
			pinger, err := probing.NewPinger(probe.Dst)
			if err != nil {
				log.Println(err)
				return
			}
			pinger.Source = probe.Src

			// This is all working around Pinger.Run() not taking a context.
			//
			// The desired behavior is as follows:
			//   * Send a ping request at a fixed period, e.g. once per second, and wait for a response.
			//
			//   * If we are still waiting for a response when it's time to send the next request,
			//     then stop waiting for a response to the current request, and send the next request.
			//
			//   * If we _do_ get a response before it's time to send another request, then
			//     we still want to let the full time period elapse (starting from when we sent the request).
			//     E.g. if we are sending one request per second, and we receive a response after 100ms, then
			//     we still want to wait the remaining 900ms before sending the next request.

			finishedChan := make(chan *probing.Statistics)
			pinger.OnFinish = func(stats *probing.Statistics) {
				finishedChan <- stats
			}

			// Note that we never set a timeout on the pinger itself
			pinger.SetPrivileged(privileged)
			pinger.Count = 1
			go func() {
				pinger.Run() // Blocks until it has dealt with a packet
			}()

			// Create a context that is canceled once we want to send the next ping
			ctx, cancelFunc := context.WithTimeout(context.Background(), pingFrequency)

			// At this point, three things can happen:
			//   * We get a response to the ping request in time
			//   * We _don't_ get a response to the ping request in time, and therefore time out
			//   * Stop() is called, so we want to abandon the running ping
			select {
			case stats := <-finishedChan:
				statCh <- ProbeStats{
					Src:  probe.Src,
					Dst:  probe.Dst,
					Loss: stats.PacketLoss,
				}
			case <-ctx.Done():
				// Timed out
				pinger.Stop()
				statCh <- ProbeStats{
					Src:  probe.Src,
					Dst:  probe.Dst,
					Loss: 100.0, // We're only sending one ping at a time, so a timeout means 100% packet loss
				}
			case <-stopChan:
				pinger.Stop()
				cancelFunc()
				return
			}

			<-ctx.Done()
			cancelFunc()
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

type GlobalProbeStats struct {
	Stats map[string]ProbeStats

	probes     []Probe
	resolveMap map[string]string
}

func (gps GlobalProbeStats) Get(dst string) ProbeStats {
	return gps.Stats[gps.resolveMap[dst]]
}

func (gps GlobalProbeStats) LowestLoss() ProbeStats {
	var lowestProbeStats ProbeStats

	for i, p := range gps.probes {
		stats := gps.Stats[p.Dst]

		if i == 0 {
			lowestProbeStats = stats
			continue
		}

		if stats.Loss < lowestProbeStats.Loss {
			lowestProbeStats = stats
		}
	}

	return lowestProbeStats
}

type Probe struct {
	Src string
	Dst string
}

type ProbeStats struct {
	Src  string
	Dst  string
	Loss float64
}

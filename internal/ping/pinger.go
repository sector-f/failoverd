package ping

import (
	"sync"
	"time"

	rb "github.com/sector-f/failoverd/internal/ringbuffer"
)

type Pinger struct {
	OnRecv func(ps ProbeStats)

	pingFreqency time.Duration
	privileged   bool
	numSeconds   uint

	closeChan chan struct{}

	probes           []Probe
	globalProbeStats map[string]ProbeStats

	createProbeChan chan Probe
	stopProbeChan   chan string
	stoppers        map[string]chan struct{} // Maps destinations to channels which are used to stop running pings
	stopWG          sync.WaitGroup

	statTracker map[string]*rb.RingBuffer // Maps destination addresses to ring buffers
	statCh      chan ProbeStats
	mu          sync.Mutex
}

func NewPinger(probes []Probe, options ...Option) (*Pinger, error) {
	stoppers := make(map[string]chan struct{}, len(probes))

	for i, probe := range probes {
		validatedProbe, err := newProbe(probe)
		if err != nil {
			return nil, err
		}

		probes[i] = validatedProbe
		stoppers[probe.Dst] = make(chan struct{})
	}

	p := &Pinger{
		probes: probes,

		closeChan: make(chan struct{}),

		createProbeChan: make(chan Probe),
		stopProbeChan:   make(chan string),
		stoppers:        stoppers,
		stopWG:          sync.WaitGroup{},

		globalProbeStats: make(map[string]ProbeStats),

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
		case probe := <-p.createProbeChan:
			p.mu.Lock()

			stopper := make(chan struct{})
			p.probes = append(p.probes, probe)
			p.stoppers[probe.Dst] = stopper
			p.statTracker[probe.Dst] = rb.New(p.numSeconds)
			go probe.run(p.pingFreqency, p.privileged, p.statCh, stopper, &p.stopWG)

			p.mu.Unlock()
		case dst := <-p.stopProbeChan:
			p.mu.Lock()

			stopper, ok := p.stoppers[dst]
			if !ok {
				break
			}
			stopper <- struct{}{}
			delete(p.stoppers, dst)

			idx := 0
			for i, probe := range p.probes {
				if dst == probe.Dst {
					idx = i
					break
				}
			}
			p.probes = append(p.probes[:idx], p.probes[idx+1:]...)
			delete(p.globalProbeStats, dst)
			delete(p.statTracker, dst)

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

			p.globalProbeStats[msg.Dst] = stats

			p.mu.Unlock()

			if p.OnRecv != nil {
				p.OnRecv(stats)
			}
		case <-p.closeChan:
			for _, ch := range p.stoppers {
				ch <- struct{}{}
			}

			return
		}
	}
}

func (p *Pinger) GetProbeStats(dst string) ProbeStats {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.globalProbeStats[dst]
}

func (p *Pinger) Stats() map[string]ProbeStats {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.globalProbeStats
}

func (p *Pinger) Stop() {
	p.closeChan <- struct{}{}
	p.stopWG.Wait()
}

func (p *Pinger) AddProbe(probe Probe) error {
	validated, err := newProbe(probe)
	if err != nil {
		return err
	}

	p.createProbeChan <- validated
	return nil
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

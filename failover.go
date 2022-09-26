package failover

import (
	"log"
	"sync"
	"time"

	probing "github.com/prometheus-community/pro-bing"
	rb "github.com/sector-f/failover/internal/ringbuffer"
)

type Failover struct {
	OnRecv func(p ProbeStats)

	pingFreqency time.Duration
	privileged   bool
	numSeconds   uint

	probes      []Probe
	statTracker map[string]*rb.RingBuffer // Map destination address to ring buffer
	statCh      chan ProbeStats
	mu          sync.Mutex
}

func NewFailover() *Failover {
	f := &Failover{
		// Placeholder probe values
		probes: []Probe{
			{
				Dst: "192.168.0.1",
			},
			{
				Dst: "192.168.0.2",
			},
			{
				Dst: "192.168.0.3",
			},
		},
		statTracker: make(map[string]*rb.RingBuffer),
		statCh:      make(chan ProbeStats),
		mu:          sync.Mutex{},
	}

	if f.pingFreqency == 0 {
		f.pingFreqency = 1 * time.Second
	}

	if f.numSeconds == 0 {
		f.numSeconds = 10
	}

	for _, probe := range f.probes {
		f.statTracker[probe.Dst] = rb.New(f.numSeconds)
	}

	return f
}

func (f *Failover) Run() {
	for _, probe := range f.probes {
		go func(src, dst string) {
			for {
				// TODO: resolve DNS once at creation and then use value here
				pinger, err := probing.NewPinger(dst)
				if err != nil {
					log.Println(err)
					return
				}

				pinger.SetPrivileged(true)
				pinger.Count = 1
				pinger.Timeout = f.pingFreqency
				timer := time.NewTimer(f.pingFreqency)
				pinger.Run()

				f.statCh <- ProbeStats{
					Src:  src,
					Dst:  dst,
					Loss: pinger.Statistics().PacketLoss,
				}

				<-timer.C
			}
		}(probe.Src, probe.Dst)
	}

	for msg := range f.statCh {
		statTracker := f.statTracker[msg.Dst]
		statTracker.Insert(msg.Loss)

		if f.OnRecv != nil {
			f.OnRecv(ProbeStats{
				Src:  msg.Src,
				Dst:  msg.Dst,
				Loss: statTracker.Average(),
			})
		}
	}
}

type Option func(f *Failover)

func WithProbes(probes []Probe) Option {
	return func(f *Failover) {
		f.probes = probes
	}
}

func WithPingFrequency(t time.Duration) Option {
	return func(f *Failover) {
		f.pingFreqency = t
	}
}

func WithPrivileged(p bool) Option {
	return func(f *Failover) {
		f.privileged = p
	}
}

func WithNumSeconds(n uint) Option {
	return func(f *Failover) {
		f.numSeconds = n
	}
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

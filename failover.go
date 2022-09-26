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

	probes      []probe
	statTracker map[string]*rb.RingBuffer // Map destination address to ring buffer
	statCh      chan ProbeStats
	mu          sync.Mutex
}

func NewFailover() *Failover {
	f := &Failover{
		// Placeholder probe values
		probes: []probe{
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

	for _, probe := range f.probes {
		f.statTracker[probe.Dst] = rb.New(10) // TODO: make value configurable
	}

	return f
}

func (f *Failover) Run() {
	for _, probe := range f.probes {
		go func(src, dst string) {
			for {
				pinger, err := probing.NewPinger(dst)
				if err != nil {
					log.Println(err)
					return
				}

				pinger.SetPrivileged(true)
				pinger.Count = 1
				pinger.Timeout = 1 * time.Second
				timer := time.NewTimer(1 * time.Second)
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

type probe struct {
	Src string
	Dst string
}

type ProbeStats struct {
	Src  string
	Dst  string
	Loss float64
}

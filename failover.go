package failover

import (
	"fmt"
	"log"
	"sync"
	"time"

	probing "github.com/prometheus-community/pro-bing"
	rb "github.com/sector-f/failover/internal/ringbuffer"
)

type Failover struct {
	probes []probe
	mu     sync.Mutex
}

func NewFailover() *Failover {
	// Placeholder values
	probes := []probe{
		{
			Dst: "192.168.0.1",
		},
		{
			Dst: "192.168.0.2",
		},
		{
			Dst: "192.168.0.3",
		},
	}

	statTracker := make(map[string]*rb.RingBuffer) // Map destination address to ring buffer
	for _, probe := range probes {
		statTracker[probe.Dst] = rb.New(10) // TODO: make value configurable
	}

	statCh := make(chan probeStats)

	// TOOD: This should be in a "Run" function (or similar)
	for _, probe := range probes {
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

				statCh <- probeStats{
					Src:  src,
					Dst:  dst,
					Loss: pinger.Statistics().PacketLoss,
				}

				<-timer.C
			}
		}(probe.Src, probe.Dst)
	}

	go func() {
		for msg := range statCh {
			statTracker := statTracker[msg.Dst]
			statTracker.Insert(msg.Loss)
			fmt.Printf("%s: %.2f%%\n", msg.Dst, statTracker.Average())
		}
	}()

	f := &Failover{
		mu: sync.Mutex{},
	}

	return f
}

type probe struct {
	Src string
	Dst string
}

type probeStats struct {
	Src  string
	Dst  string
	Loss float64
}

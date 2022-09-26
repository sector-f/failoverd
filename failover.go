package failover

import (
	"fmt"
	"log"
	"sync"

	"github.com/sector-f/failover/internal/ping"
	rb "github.com/sector-f/failover/internal/ringbuffer"
)

type Failover struct {
	probes []Probe
	mu     sync.Mutex
}

func NewFailover() *Failover {
	// Placeholder values
	probes := []Probe{
		{
			Dst: "192.168.0.2",
		},
	}

	probeStats := make(map[string]*rb.RingBuffer)
	for _, probe := range probes {
		probeStats[probe.Dst] = rb.New(10) // TODO: make value configurable
	}

	statCh := make(chan ProbeStats)

	// TOOD: This should be in a "Run" function (or similar)
	for _, probe := range probes {
		pinger, err := ping.NewPinger(probe.Dst)
		if err != nil {
			log.Fatalln(err)
		}
		pinger.SetPrivileged(true)

		go func(p *ping.Pinger, src, dst string) {
			go p.Run()
			for {
				select {
				case stats := <-p.StatsChan:
					statCh <- ProbeStats{
						Src:  src,
						Dst:  dst,
						Loss: stats.PacketLoss,
					}
				}
			}
		}(pinger, probe.Src, probe.Dst)
	}

	go func() {
		for msg := range statCh {
			stats := probeStats[msg.Dst]
			stats.Insert(msg.Loss)
			fmt.Printf("%.2f%%\n", stats.Average())

			// fmt.Printf("%s: %.2f%%\n", msg.Dst, stats.Average())
		}
	}()

	f := &Failover{
		mu: sync.Mutex{},
	}

	return f
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

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

	probeStats := make(map[string]*StatTracker)
	for _, probe := range probes {
		probeStats[probe.Dst] = &StatTracker{
			buffer: rb.New(10), // TODO: make value configurable
		}
	}

	statCh := make(chan ProbeStats)

	// TOOD: This should be in a "Run" function (or similar)
	for _, probe := range probes {
		pinger, err := probing.NewPinger(probe.Dst)
		if err != nil {
			log.Fatalln(err)
		}
		pinger.SetPrivileged(true)

		go func(p *probing.Pinger, src, dst string) {
			go p.Run()
			ticker := time.NewTicker(1 * time.Second) // TODO: make configurable
			for {
				select {
				case <-ticker.C:
					tracker := probeStats[dst]
					stats := p.Statistics()

					lastSeenSent := tracker.lastSeenSent
					lastSeenRcvd := tracker.lastSeenRcvd
					sent := stats.PacketsSent - lastSeenSent
					rcvd := stats.PacketsRecv - lastSeenRcvd

					var loss float64
					if sent > 0 {
						loss = float64(sent-rcvd) / float64(sent) * 100
					}

					tracker.lastSeenSent = stats.PacketsSent
					tracker.lastSeenRcvd = stats.PacketsRecv

					statCh <- ProbeStats{
						Src:  src,
						Dst:  dst,
						Loss: loss,
					}
				}
			}
		}(pinger, probe.Src, probe.Dst)
	}

	go func() {
		for msg := range statCh {
			statTracker := probeStats[msg.Dst]
			statTracker.buffer.Insert(msg.Loss)
			fmt.Printf("%.2f%%\n", statTracker.buffer.Average())
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

type StatTracker struct {
	lastSeenSent int
	lastSeenRcvd int
	buffer       *rb.RingBuffer
}

type ProbeStats struct {
	Src  string
	Dst  string
	Loss float64
}

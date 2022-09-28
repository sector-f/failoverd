package failover

import (
	"context"
	"fmt"
	"log"
	"net"
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

	closeChan    chan struct{}
	isClosedChan chan struct{}

	probes      []Probe
	statTracker map[string]*rb.RingBuffer // Map destination address to ring buffer
	statCh      chan ProbeStats
	mu          sync.Mutex
}

func NewFailover(probes []Probe, options ...Option) (*Failover, error) {
	resolvedProbes := make([]Probe, 0, len(probes))
	for _, p := range probes {
		addrs, err := net.LookupHost(p.Dst)
		if err != nil {
			return nil, fmt.Errorf("could not resolve %s: %w", p.Dst, err)
		}

		if len(addrs) == 0 {
			return nil, fmt.Errorf("no addresses returned for %s", p.Dst)
		}

		resolvedProbes = append(resolvedProbes, Probe{Src: p.Src, Dst: addrs[0]})
	}

	f := &Failover{
		probes: resolvedProbes,

		closeChan:    make(chan struct{}),
		isClosedChan: make(chan struct{}),

		statTracker: make(map[string]*rb.RingBuffer),
		statCh:      make(chan ProbeStats),
		mu:          sync.Mutex{},
	}

	return f, nil
}

func (f *Failover) Run() {
	if f.pingFreqency <= 0 {
		f.pingFreqency = 1 * time.Second
	}

	if f.numSeconds <= 0 {
		f.numSeconds = 10
	}

	for _, probe := range f.probes {
		f.statTracker[probe.Dst] = rb.New(f.numSeconds)

		go func(src, dst string) {
			for {
				select {
				case <-f.isClosedChan:
					return
				default:
					pinger, err := probing.NewPinger(dst)
					if err != nil {
						log.Println(err)
						return
					}
					finishedChan := make(chan *probing.Statistics)
					pinger.OnFinish = func(stats *probing.Statistics) {
						finishedChan <- stats
					}

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

					// Note that we never set a timeout on the pinger itself
					pinger.SetPrivileged(f.privileged)
					pinger.Count = 1
					go func() {
						pinger.Run() // Blocks until it has dealt with a packet
					}()

					// Create a context that is canceled once we want to send the next ping
					ctx, cancelFunc := context.WithTimeout(context.Background(), f.pingFreqency)

					// At this point, three things can happen:
					//   * We get a response to the ping request in time
					//   * We _don't_ get a response to the ping request in time, and therefore time out
					//   * Stop() is called, so we want to abandon the running ping
					select {
					case stats := <-finishedChan:
						f.statCh <- ProbeStats{
							Src:  src,
							Dst:  dst,
							Loss: stats.PacketLoss,
						}
					case <-ctx.Done():
						// Timed out
						pinger.Stop()
						f.statCh <- ProbeStats{
							Src:  src,
							Dst:  dst,
							Loss: 100.0, // We're only sending one ping at a time, so a timeout means 100% packet loss
						}
					case <-f.isClosedChan:
						pinger.Stop()
						cancelFunc()
						return
					}

					<-ctx.Done()
					cancelFunc()
				}
			}
		}(probe.Src, probe.Dst)
	}

	for {
		select {
		case msg := <-f.statCh:
			f.mu.Lock()

			statTracker := f.statTracker[msg.Dst]
			statTracker.Insert(msg.Loss)

			if f.OnRecv != nil {
				f.OnRecv(ProbeStats{
					Src:  msg.Src,
					Dst:  msg.Dst,
					Loss: statTracker.Average(),
				})
			}

			f.mu.Unlock()
		case <-f.closeChan:
			close(f.isClosedChan)
			return
		}

	}
}

func (f *Failover) Stats() map[string]ProbeStats {
	f.mu.Lock()
	defer f.mu.Unlock()

	m := make(map[string]ProbeStats)
	for _, probe := range f.probes {
		m[probe.Dst] = ProbeStats{
			Src:  probe.Src,
			Dst:  probe.Dst,
			Loss: f.statTracker[probe.Dst].Average(),
		}
	}

	return m
}

func (f *Failover) Stop() {
	f.closeChan <- struct{}{}
}

type Option func(f *Failover)

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

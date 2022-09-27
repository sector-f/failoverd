package failover

import (
	"context"
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

	closeChan    chan struct{}
	isClosedChan chan struct{}

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
		closeChan:    make(chan struct{}),
		isClosedChan: make(chan struct{}),
		statTracker:  make(map[string]*rb.RingBuffer),
		statCh:       make(chan ProbeStats),
		mu:           sync.Mutex{},
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
				select {
				case <-f.isClosedChan:
					return
				default:
					// TODO: resolve DNS once at creation and then use value here
					pinger, err := probing.NewPinger(dst)
					if err != nil {
						log.Println(err)
						return
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
					pinger.SetPrivileged(true)
					pinger.Count = 1

					// Start running the pinger in its own goroutine; Run() blocks until it has dealt with pinger.Count packets
					go func() {
						pinger.Run()
					}()

					// Create a context that is canceled once we want to send the next ping
					ctx, cancelFunc := context.WithTimeout(context.Background(), f.pingFreqency)

					// At this point, three things can happen:
					//   * We get a response to the ping request in time
					//   * We _don't_ get a response to the ping request in time, and therefore time out
					//   * Stop() is called, so we want to abandon the running ping
					select {
					case <-ctx.Done():
						pinger.Stop()
						f.statCh <- ProbeStats{
							Src:  src,
							Dst:  dst,
							Loss: pinger.Statistics().PacketLoss,
						}
						break
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
		case <-f.closeChan:
			close(f.isClosedChan)
			return
		case msg := <-f.statCh:
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
}

func (f *Failover) Stop() {
	f.closeChan <- struct{}{}
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

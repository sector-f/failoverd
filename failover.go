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
	"github.com/vishvananda/netlink"
)

type Failover struct {
	OnRecv func(gps GlobalProbeStats)

	pingFreqency time.Duration
	privileged   bool
	numSeconds   uint

	closeChan    chan struct{}
	isClosedChan chan struct{}

	globalStats     GlobalProbeStats
	globalStatsChan chan GlobalProbeStats

	probes     []Probe
	resolveMap map[string]string // Maps client-specified destinations to resolved destinations

	statTracker map[string]*rb.RingBuffer // Map destination address to ring buffer
	statCh      chan ProbeStats
	mu          sync.Mutex
}

func NewFailover(probes []Probe, options ...Option) (*Failover, error) {
	resolvedProbes := make([]Probe, 0, len(probes))
	resolveMap := make(map[string]string)

	for _, p := range probes {
		// If specified source is not an address, treat it as a network interface name
		// and attempt to determine its address using netlink.
		if net.ParseIP(p.Src) == nil {
			link, err := netlink.LinkByName(p.Src)
			if err != nil {
				return nil, fmt.Errorf("could not determine address: %w", err)
			}

			addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
			if err != nil {
				return nil, fmt.Errorf("could not determine address: %w", err)
			}

			if len(addrs) == 0 {
				return nil, fmt.Errorf("interface has no addresses")
			}

			p.Src = addrs[0].IP.String() // TODO: figure out if there's a better way to pick an address than just "use the first one"
		}

		addrs, err := net.LookupHost(p.Dst)
		if err != nil {
			return nil, fmt.Errorf("could not resolve %s: %w", p.Dst, err)
		}

		if len(addrs) == 0 {
			return nil, fmt.Errorf("no addresses returned for %s", p.Dst)
		}

		resolvedProbes = append(resolvedProbes, Probe{Src: p.Src, Dst: addrs[0]})
		resolveMap[p.Dst] = addrs[0]
	}

	gps := GlobalProbeStats{
		Stats: make(map[string]ProbeStats),

		probes:     resolvedProbes,
		resolveMap: resolveMap,
	}

	f := &Failover{
		probes: resolvedProbes,

		closeChan:    make(chan struct{}),
		isClosedChan: make(chan struct{}),

		globalStats:     gps,
		globalStatsChan: make(chan GlobalProbeStats),

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
					pinger.Source = src

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
		case f.globalStatsChan <- f.globalStats:
		case msg := <-f.statCh:
			f.mu.Lock()

			statTracker := f.statTracker[msg.Dst]
			statTracker.Insert(msg.Loss)

			stats := ProbeStats{
				Src:  msg.Src,
				Dst:  msg.Dst,
				Loss: statTracker.Average(),
			}

			f.globalStats.Stats[msg.Dst] = stats

			if f.OnRecv != nil {
				f.OnRecv(f.globalStats)
			}

			f.mu.Unlock()
		case <-f.closeChan:
			close(f.isClosedChan)
			return
		}
	}
}

func (f *Failover) Stats() GlobalProbeStats {
	return <-f.globalStatsChan
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

package ping

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	probing "github.com/prometheus-community/pro-bing"
	"github.com/vishvananda/netlink"
)

type Probe struct {
	Src string
	Dst string
}

// newProbe takes in a Probe and validates its addresses
func newProbe(probe Probe) (Probe, error) {
	// Verify destination is valid IP address
	if net.ParseIP(probe.Dst) == nil {
		return Probe{}, fmt.Errorf("%s is not a valid IP address", probe.Dst)
	}

	validated := probe

	// If specified source is not an address, treat it as a network interface name
	// and attempt to determine its address using netlink.
	if probe.Src != "" && net.ParseIP(probe.Src) == nil {
		link, err := netlink.LinkByName(probe.Src)
		if err != nil {
			return Probe{}, fmt.Errorf("could not determine address of %s: %w", probe.Src, err)
		}

		addrs, err := netlink.AddrList(link, netlink.FAMILY_V4) // TODO: Handle IPv6?
		if err != nil {
			return Probe{}, fmt.Errorf("could not determine address of %s: %w", probe.Src, err)
		}

		if len(addrs) == 0 {
			return Probe{}, fmt.Errorf("interface has no addresses")
		}

		validated.Src = addrs[0].IP.String() // TODO: figure out if there's a better way to pick an address than just "use the first one"
	}

	return validated, nil
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

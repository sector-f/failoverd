// Package ping implements a wrapper around the github.com/prometheus-community/pro-bing package.
// It provides the ability to calculate packet loss while taking into account the fact that responses to a ping are not instantaneous.

package ping

import (
	"fmt"
	"sync"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

type packetState uint

const (
	sentState packetState = iota
	waitingState
	receivedState
)

type Pinger struct {
	StatsChan      chan *probing.Statistics
	packetsTracker map[int]map[int]packetState // Map of ID -> sequence number -> packet state

	packetsSent    int
	packetsWaiting int
	packetsRecv    int

	addr string // Destination address
	mu   sync.Mutex
	*probing.Pinger
}

func NewPinger(addr string) (*Pinger, error) {
	pinger, err := probing.NewPinger(addr)
	if err != nil {
		return nil, err
	}

	p := Pinger{
		Pinger:         pinger,
		StatsChan:      make(chan *probing.Statistics),
		packetsTracker: make(map[int]map[int]packetState),
		addr:           addr,
		mu:             sync.Mutex{},
	}

	pinger.OnSend = func(packet *probing.Packet) {
		p.mu.Lock()

		if _, ok := p.packetsTracker[packet.ID]; !ok {
			p.packetsTracker[packet.ID] = make(map[int]packetState)
		}
		fmt.Println("Sending packet", packet.Seq)
		p.packetsTracker[packet.ID][packet.Seq] = sentState
		p.packetsSent++

		p.mu.Unlock()

		go func(packet *probing.Packet) {
			time.Sleep(1 * time.Second) // TODO: Make this configurable?

			p.mu.Lock()
			defer p.mu.Unlock()

			if _, ok := p.packetsTracker[packet.ID]; !ok {
				p.packetsTracker[packet.ID] = make(map[int]packetState)
			}

			// Ignore packets that  we've already received a response for
			currentState := p.packetsTracker[packet.ID][packet.Seq]
			fmt.Printf("Current state of %v is %v\n", packet.Seq, currentState)
			if currentState == receivedState {
				return
			}

			fmt.Println("Waiting on packet", packet.Seq)
			p.packetsTracker[packet.ID][packet.Seq] = waitingState
			p.packetsWaiting++
		}(packet)
	}

	pinger.OnRecv = func(packet *probing.Packet) {
		p.mu.Lock()
		if _, ok := p.packetsTracker[packet.ID][packet.Seq]; ok {
			fmt.Println("Received packet", packet.Seq)
			p.packetsTracker[packet.ID][packet.Seq] = receivedState
			p.packetsRecv++
			p.packetsWaiting--
		}
		p.mu.Unlock()
	}

	go func() {
		ticker := time.NewTicker(1 * time.Second) // TODO: make configurable
		for {
			select {
			case <-ticker.C:
				p.StatsChan <- p.getAndResetStats()
			}
		}
	}()

	return &p, nil
}

func (p *Pinger) getAndResetStats() *probing.Statistics {
	p.mu.Lock()
	defer p.mu.Unlock()

	sent := p.packetsWaiting
	recv := p.packetsRecv

	fmt.Println(p.packetsSent, p.packetsWaiting, p.packetsRecv)

	// We don't increment PacketsSent until some time has elapsed.
	// So, if we've received more packets than we've sent, then loss is 0% (and we're currently waiting for a response to arrive)
	var loss float64 = 0
	if recv < sent {
		loss = float64(sent-recv) / float64(sent) * 100
	}

	stats := p.Pinger.Statistics()
	stats.PacketsSent = sent
	stats.PacketsRecv = recv
	stats.PacketLoss = loss

	/*
			for _, seqMap := range p.packetsTracker {
				for seq := range seqMap {
					delete(seqMap, seq)
				}
			}


		p.packetsSent = 0
		p.packetsRecv = 0
		p.packetsWaiting = 0
	*/

	return stats
}

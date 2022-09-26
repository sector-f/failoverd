// Package ringbuffer implements a ring buffer that tracks recent values.

package ringbuffer

import (
	"sync"
	"time"
)

type bufferElement struct {
	val         float64
	insertCount uint
}

type RingBuffer struct {
	pointer uint
	buffer  []bufferElement

	sum         float64
	insertCount uint

	mu sync.Mutex
}

func New(seconds uint) *RingBuffer {
	ticker := time.NewTicker(1 * time.Second) // TODO: make configurable?
	return newWithChannel(seconds, ticker.C)
}

func (rb *RingBuffer) Insert(n float64) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.buffer[rb.pointer].val += n
	rb.buffer[rb.pointer].insertCount++

	rb.sum += n
	rb.insertCount++
}

func (rb *RingBuffer) Average() float64 {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	return rb.sum / float64(rb.insertCount)
}

func newWithChannel(seconds uint, c <-chan time.Time) *RingBuffer {
	rb := RingBuffer{
		buffer: make([]bufferElement, seconds),
	}

	go rb.run(c)

	return &rb
}

// Method to facilitate testing by allowing manual control of time steps
func (rb *RingBuffer) run(c <-chan time.Time) {
	for {
		select {
		case <-c:
			rb.mu.Lock()

			rb.pointer += 1
			if rb.pointer == uint(len(rb.buffer)) {
				rb.pointer = 0
			}

			rb.sum -= rb.buffer[rb.pointer].val
			rb.insertCount -= rb.buffer[rb.pointer].insertCount

			rb.buffer[rb.pointer].val = 0
			rb.buffer[rb.pointer].insertCount = 0

			rb.mu.Unlock()
		}
	}
}

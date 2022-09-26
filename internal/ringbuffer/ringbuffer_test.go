package ringbuffer

import (
	"testing"
	"time"
)

func TestSingleInsert(t *testing.T) {
	ch := make(chan time.Time)
	rb := newWithChannel(10, ch)

	rb.Insert(5)

	avg := rb.Average()
	if avg != 5 {
		t.Fatalf("Expected 5, got %v", avg)
	}
}

func TestManyInserts(t *testing.T) {
	ch := make(chan time.Time)
	rb := newWithChannel(10, ch)

	rb.Insert(5)
	rb.Insert(5)
	rb.Insert(5)

	avg := rb.Average()
	if avg != 5 {
		t.Fatalf("Expected 5, got %v", avg)
	}
}

func TestManyInsertsIntoMultipleCells(t *testing.T) {
	ch := make(chan time.Time)
	rb := newWithChannel(10, ch)

	rb.Insert(5)
	ch <- time.Now()

	rb.Insert(5)
	ch <- time.Now()

	rb.Insert(5)
	ch <- time.Now()

	avg := rb.Average()
	if avg != 5 {
		t.Fatalf("Expected 5, got %v", avg)
	}
}

func TestCircularOverwrite(t *testing.T) {
	ch := make(chan time.Time)
	rb := newWithChannel(1, ch)

	rb.Insert(1)
	ch <- time.Now()
	rb.Insert(2)

	avg := rb.Average()
	if avg != 2 {
		t.Fatalf("Expected 2, got %v", avg)
	}
}

func TestAvg(t *testing.T) {
	ch := make(chan time.Time)
	rb := newWithChannel(2, ch)

	rb.Insert(0)
	ch <- time.Now()
	rb.Insert(1)

	avg := rb.Average()
	if avg != 0.5 {
		t.Fatalf("Expected 0.5, got %v", avg)
	}
}

package main

import (
	"time"

	"github.com/sector-f/failover"
)

func main() {
	failover.NewFailover()
	time.Sleep(600 * time.Second)
}

package main

import (
	"fmt"

	"github.com/sector-f/failover"
)

func main() {
	f := failover.NewFailover()
	f.OnRecv = func(p failover.ProbeStats) {
		fmt.Printf("%s: %.02f\n", p.Dst, p.Loss)
	}

	f.Run()
}

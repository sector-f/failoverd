package main

import (
	"fmt"

	"github.com/sector-f/failover"
)

func main() {
	probes := []failover.Probe{
		{
			Dst: "192.168.0.1",
		},
		{
			Dst: "192.168.0.2",
		},
		{
			Dst: "192.168.0.3",
		},
	}

	f := failover.NewFailover(probes, failover.WithPrivileged(true))
	f.OnRecv = func(p failover.ProbeStats) {
		fmt.Printf("%s: %.02f\n", p.Dst, p.Loss)
	}

	f.Run()
}

package main

import (
	"fmt"
	"os"

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

	f, err := failover.NewFailover(probes, failover.WithPrivileged(true))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	f.OnRecv = func(p failover.ProbeStats) {
		fmt.Printf("%s: %.02f\n", p.Dst, p.Loss)
	}

	f.Run()
}

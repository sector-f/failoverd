package main

import (
	"fmt"
	"os"
	"time"

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
		{
			Dst: "google.com",
		},
	}

	f, err := failover.NewFailover(probes, failover.WithPrivileged(true))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	go f.Run()

	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ticker.C:
			statMap := f.Stats()
			for _, p := range probes {
				stats := statMap[p.Dst]
				fmt.Printf("%s: %.02f\n", stats.Dst, stats.Loss)
			}
		}
	}
}

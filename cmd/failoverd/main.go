package main

import (
	"fmt"
	"log"
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
	}

	f, err := failover.NewFailover(probes, failover.WithPrivileged(true))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	go f.Run()

	var (
		lowestDst  string
		lowestLoss float64
	)

	for {
		time.Sleep(10 * time.Second)

		statMap := f.Stats()
		for i, p := range probes {
			stats := statMap[p.Dst]

			if i == 0 {
				lowestDst = stats.Dst
				lowestLoss = stats.Loss
				continue
			}

			if stats.Loss < lowestLoss {
				lowestDst = stats.Dst
				lowestLoss = stats.Loss
			}
		}

		log.Printf("%s: %.02f%%\n", lowestDst, lowestLoss)
	}
}

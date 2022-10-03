package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/sector-f/failoverd/internal/lua"
	"github.com/sector-f/failoverd/internal/ping"
)

func main() {
	configFilename := flag.String("c", "config.lua", "Path to configuration Lua script")
	flag.Parse()

	luaEngine, err := lua.New(*configFilename)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer luaEngine.Close()

	config := luaEngine.Config
	p, err := ping.NewPinger(
		config.Probes,
		ping.WithPingFrequency(config.PingFrequency),
		ping.WithNumSeconds(config.NumSeconds),
		ping.WithPrivileged(config.Privileged),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	p.OnRecv = func(ps ping.ProbeStats) {
		err := luaEngine.OnRecv(p.Stats(), ps)
		if err != nil {
			log.Println(err)
		}
	}

	go p.Run()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	ticker := time.NewTicker(config.UpdateFrequency)
	for {
		select {
		case <-ticker.C:
			err := luaEngine.OnUpdate(p.Stats())
			if err != nil {
				log.Println(err)
			}
		case <-sigChan:
			err := luaEngine.OnQuit(p.Stats())
			if err != nil {
				log.Println(err)
			}

			p.Stop()
			return
		}
	}
}

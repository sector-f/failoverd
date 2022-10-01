package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/sector-f/failover"
	lua "github.com/yuin/gopher-lua"
)

func main() {
	luaState := lua.NewState()
	defer luaState.Close()

	configFilename := "config.lua" // TODO: make configurable
	luaState.DoFile(configFilename)
	config, err := configFromLua(luaState)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
		os.Exit(1)
	}

	registerProbeStatsType(luaState)

	probes := config.Probes

	f, err := failover.NewFailover(probes, failover.WithPrivileged(config.Privileged))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	go f.Run()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	ticker := time.NewTicker(config.UpdateFrequency)
	var lowestProbeStats failover.ProbeStats
	for {
		select {
		case <-ticker.C:
			statMap := f.Stats()
			for i, p := range probes {
				stats := statMap[p.Dst]

				if i == 0 {
					lowestProbeStats = stats
					continue
				}

				if stats.Loss < lowestProbeStats.Loss {
					lowestProbeStats = stats
				}
			}

			ud := &lua.LUserData{
				Value:     &lowestProbeStats,
				Metatable: luaState.GetTypeMetatable(luaProbeStatsTypeName),
			}

			err := luaState.CallByParam(
				lua.P{
					Fn:      config.OnUpdateFunc, // I suppose this name could be hardcoded in?
					NRet:    0,
					Protect: true,
				},
				ud,
			)

			if err != nil {
				log.Printf("Error calling on_update function: %v\n", err)
			}
		case <-sigChan:
			err := luaState.CallByParam(
				lua.P{
					Fn:      config.OnQuitFunc,
					NRet:    0,
					Protect: true,
				},
			)

			if err != nil {
				log.Printf("Error calling on_quit function: %v\n", err)
			}

			f.Stop()
			return
		}
	}
}

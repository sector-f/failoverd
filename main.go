package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/sector-f/failoverd/internal/ping"
	lua "github.com/yuin/gopher-lua"
)

func main() {
	configFilename := flag.String("c", "config.lua", "Path to configuration Lua script")
	flag.Parse()

	luaState := lua.NewState()
	defer luaState.Close()

	registerTypes(luaState)
	err := luaState.DoFile(*configFilename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
		os.Exit(1)
	}

	config, err := configFromLua(luaState)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
		os.Exit(1)
	}

	probes := config.Probes

	p, err := ping.NewPinger(
		probes,
		ping.WithPingFrequency(config.PingFrequency),
		ping.WithNumSeconds(config.NumSeconds),
		ping.WithPrivileged(config.Privileged),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	p.OnRecv = func(ps ping.ProbeStats) {
		globalProbeStatsUD := &lua.LUserData{
			Value:     p.Stats(),
			Metatable: luaState.GetTypeMetatable(luaGlobalProbeStatsTypeName),
		}

		probeStatsUD := &lua.LUserData{
			Value:     &ps,
			Metatable: luaState.GetTypeMetatable(luaProbeStatsTypeName),
		}

		if config.OnRecvFunc.Type() != lua.LTNil {
			err := luaState.CallByParam(
				lua.P{
					Fn:      config.OnRecvFunc,
					NRet:    0,
					Protect: true,
				},
				globalProbeStatsUD,
				probeStatsUD,
			)

			if err != nil {
				log.Printf("Error calling on_recv function: %v\n", err)
			}
		}
	}

	go p.Run()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	ticker := time.NewTicker(config.UpdateFrequency)
	for {
		select {
		case <-ticker.C:
			stats := p.Stats()

			ud := &lua.LUserData{
				Value:     stats,
				Metatable: luaState.GetTypeMetatable(luaGlobalProbeStatsTypeName),
			}

			if config.OnUpdateFunc.Type() != lua.LTNil {
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
			}
		case <-sigChan:
			if config.OnQuitFunc.Type() != lua.LTNil {
				stats := p.Stats()

				ud := &lua.LUserData{
					Value:     stats,
					Metatable: luaState.GetTypeMetatable(luaGlobalProbeStatsTypeName),
				}

				err := luaState.CallByParam(
					lua.P{
						Fn:      config.OnQuitFunc,
						NRet:    0,
						Protect: true,
					},
					ud,
				)

				if err != nil {
					log.Printf("Error calling on_quit function: %v\n", err)
				}
			}

			p.Stop()
			return
		}
	}
}

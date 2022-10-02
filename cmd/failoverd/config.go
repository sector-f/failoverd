package main

import (
	"fmt"
	"time"

	"github.com/sector-f/failover/internal/ping"
	lua "github.com/yuin/gopher-lua"
)

type config struct {
	PingFrequency   time.Duration
	UpdateFrequency time.Duration
	Privileged      bool
	NumSeconds      uint
	Probes          []ping.Probe

	OnRecvFunc   lua.LValue
	OnUpdateFunc lua.LValue
	OnQuitFunc   lua.LValue
}

func configFromLua(l *lua.LState) (config, error) {
	c := config{}

	switch pingFreq := l.GetGlobal("ping_frequency").(type) {
	case lua.LNumber:
		c.PingFrequency = time.Duration(float64(pingFreq) * float64(time.Second))
	default:
		return c, fmt.Errorf("`ping_frequency` must be a number, not a %s", pingFreq.Type())
	}

	switch updateFreq := l.GetGlobal("update_frequency").(type) {
	case lua.LNumber:
		c.UpdateFrequency = time.Duration(float64(updateFreq) * float64(time.Second))
	default:
		return c, fmt.Errorf("`update_frequency` must be a number, not a %s", updateFreq.Type())
	}

	switch privileged := l.GetGlobal("privileged").(type) {
	case lua.LBool:
		c.Privileged = bool(privileged)
	default:
		return c, fmt.Errorf("`privileged` must be a bool, not a %s", privileged.Type())
	}

	switch numSeconds := l.GetGlobal("num_seconds").(type) {
	case lua.LNumber:
		c.NumSeconds = uint(numSeconds)
	default:
		return c, fmt.Errorf("`num_seconds` must be a number, not a %s", numSeconds.Type())
	}

	switch probes := l.GetGlobal("probes").(type) {
	case *lua.LTable:
		p := []ping.Probe{}

		var err error = nil
		probes.ForEach(
			func(_ lua.LValue, val lua.LValue) {
				switch probeItem := val.(type) {
				case *lua.LUserData:
					switch probe := probeItem.Value.(type) {
					case ping.Probe:
						p = append(p, probe)
					default:
						err = fmt.Errorf("`probes` item must be a probe")
					}
				default:
					err = fmt.Errorf("`probes` item must be a probe, not a %s", probeItem.Type())
				}
			},
		)
		if err != nil {
			return c, err
		}

		c.Probes = p
	default:
		return c, fmt.Errorf("`probes` must be a table, not a %s", probes.Type())
	}

	switch onRecvFunc := l.GetGlobal("on_recv").(type) {
	case *lua.LFunction, *lua.LNilType:
		c.OnRecvFunc = onRecvFunc
	default:
		return c, fmt.Errorf("`on_recv` must be a function, not a %s", onRecvFunc.Type())
	}

	switch onUpdateFunc := l.GetGlobal("on_update").(type) {
	case *lua.LFunction, *lua.LNilType:
		c.OnUpdateFunc = onUpdateFunc
	default:
		return c, fmt.Errorf("`on_update` must be a function, not a %s", onUpdateFunc.Type())
	}

	switch onQuitFunc := l.GetGlobal("on_quit").(type) {
	case *lua.LFunction, *lua.LNilType:
		c.OnQuitFunc = onQuitFunc
	default:
		return c, fmt.Errorf("`on_quit` must be a function, not a %s", onQuitFunc.Type())
	}

	// Set defaults/overrides

	if c.PingFrequency < 1*time.Second {
		c.PingFrequency = 1 * time.Second
	}

	if c.UpdateFrequency < 1*time.Second {
		c.UpdateFrequency = 1 * time.Second
	}

	if c.NumSeconds < 1 {
		c.NumSeconds = 10
	}

	return c, nil
}

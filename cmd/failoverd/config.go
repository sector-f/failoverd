package main

import (
	"fmt"
	"time"

	"github.com/sector-f/failover"
	lua "github.com/yuin/gopher-lua"
)

type config struct {
	PingFrequency   time.Duration
	UpdateFrequency time.Duration
	Privileged      bool
	NumSeconds      uint
	Probes          []failover.Probe

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
		p := []failover.Probe{}

		var err error = nil
		probes.ForEach(
			func(_ lua.LValue, val lua.LValue) {
				switch dst := val.(type) {
				case lua.LString:
					p = append(p, failover.Probe{Dst: string(dst)})
				default:
					err = fmt.Errorf("`probe` item must be a string, not a %s", dst.Type())
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

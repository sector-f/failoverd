package main

import (
	"github.com/sector-f/failover/internal/ping"
	lua "github.com/yuin/gopher-lua"
)

const (
	luaProbeStatsTypeName       = "probe_stats"
	luaGlobalProbeStatsTypeName = "global_probe_stats"
	luaProbeTypeName            = "probe"
)

func registerTypes(l *lua.LState) {
	registerProbeStatsType(l)
	registerGlobalProbeStatsType(l)
	registerProbeType(l)
}

func registerGlobalProbeStatsType(l *lua.LState) {
	mt := l.NewTypeMetatable(luaGlobalProbeStatsTypeName)
	l.SetGlobal(luaGlobalProbeStatsTypeName, mt)

	methods := map[string]lua.LGFunction{
		"lowest_loss": globalProbeStatsLowestLoss,
		"get":         globalProbeStatsGet,
	}

	l.SetField(mt, "__index", l.SetFuncs(l.NewTable(), methods))
}

func checkGlobalProbeStats(l *lua.LState) ping.GlobalProbeStats {
	ud := l.CheckUserData(1)
	if v, ok := ud.Value.(ping.GlobalProbeStats); ok {
		return v
	}
	l.ArgError(1, "global_probe_stats expected")
	return ping.GlobalProbeStats{}
}

func globalProbeStatsGet(l *lua.LState) int {
	p := checkGlobalProbeStats(l)
	dst := l.CheckString(2)
	stats := p.Get(dst)

	l.Push(&lua.LUserData{
		Value:     &stats,
		Metatable: l.GetTypeMetatable(luaProbeStatsTypeName),
	})

	return 1
}

func globalProbeStatsLowestLoss(l *lua.LState) int {
	p := checkGlobalProbeStats(l)
	lowest := p.LowestLoss()

	l.Push(&lua.LUserData{
		Value:     &lowest,
		Metatable: l.GetTypeMetatable(luaProbeStatsTypeName),
	})

	return 1
}

func registerProbeStatsType(l *lua.LState) {
	mt := l.NewTypeMetatable(luaProbeStatsTypeName)
	l.SetGlobal(luaProbeStatsTypeName, mt)

	methods := map[string]lua.LGFunction{
		"src":  probeStatsGetSrc,
		"dst":  probeStatsGetDst,
		"loss": probeStatsGetLoss,
	}

	l.SetField(mt, "__index", l.SetFuncs(l.NewTable(), methods))
}

func checkProbeStats(l *lua.LState) *ping.ProbeStats {
	ud := l.CheckUserData(1)
	if v, ok := ud.Value.(*ping.ProbeStats); ok {
		return v
	}
	l.ArgError(1, "probe_stats expected")
	return nil
}

func probeStatsGetSrc(l *lua.LState) int {
	p := checkProbeStats(l)
	l.Push(lua.LString(p.Src))
	return 1
}

func probeStatsGetDst(l *lua.LState) int {
	p := checkProbeStats(l)
	l.Push(lua.LString(p.Dst))
	return 1
}

func probeStatsGetLoss(l *lua.LState) int {
	p := checkProbeStats(l)
	l.Push(lua.LNumber(p.Loss))
	return 1
}

func registerProbeType(l *lua.LState) {
	mt := l.NewTypeMetatable(luaProbeTypeName)
	l.SetGlobal(luaProbeTypeName, mt)
	l.SetField(mt, "new", l.NewFunction(newProbe))
	// l.SetField(mt, "__index", l.SetFuncs(l.NewTable(), nil))
}

func newProbe(l *lua.LState) int {
	p := ping.Probe{}

	switch l.GetTop() {
	case 1:
		p.Dst = l.CheckString(1)
	case 2:
		p.Dst = l.CheckString(1)
		p.Src = l.CheckString(2)
	default:
		l.ArgError(1, "no destination specified")
		return 0
	}

	l.Push(&lua.LUserData{
		Value:     p,
		Metatable: l.GetTypeMetatable(luaProbeTypeName),
	})

	return 1
}

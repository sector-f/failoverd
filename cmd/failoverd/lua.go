package main

import (
	"github.com/sector-f/failover"
	lua "github.com/yuin/gopher-lua"
)

const (
	luaProbeStatsTypeName       = "probe_stats"
	luaGlobalProbeStatsTypeName = "global_probe_stats"
)

func registerGlobalProbeStatsType(l *lua.LState) {
	mt := l.NewTypeMetatable(luaGlobalProbeStatsTypeName)
	l.SetGlobal(luaGlobalProbeStatsTypeName, mt)

	methods := map[string]lua.LGFunction{
		"lowest_loss": globalProbeStatsLowestLoss,
	}

	l.SetField(mt, "__index", l.SetFuncs(l.NewTable(), methods))
}

func checkGlobalProbeStats(l *lua.LState) failover.GlobalProbeStats {
	ud := l.CheckUserData(1)
	if v, ok := ud.Value.(failover.GlobalProbeStats); ok {
		return v
	}
	l.ArgError(1, "global_probe_stats expected")
	return failover.GlobalProbeStats{}
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
		"src":  probeGetSrc,
		"dst":  probeGetDst,
		"loss": probeGetLoss,
	}

	l.SetField(mt, "__index", l.SetFuncs(l.NewTable(), methods))
}

func checkProbeStats(l *lua.LState) *failover.ProbeStats {
	ud := l.CheckUserData(1)
	if v, ok := ud.Value.(*failover.ProbeStats); ok {
		return v
	}
	l.ArgError(1, "probe expected")
	return nil
}

func probeGetSrc(l *lua.LState) int {
	p := checkProbeStats(l)
	l.Push(lua.LString(p.Src))
	return 1
}

func probeGetDst(l *lua.LState) int {
	p := checkProbeStats(l)
	l.Push(lua.LString(p.Dst))
	return 1
}

func probeGetLoss(l *lua.LState) int {
	p := checkProbeStats(l)
	l.Push(lua.LNumber(p.Loss))
	return 1
}

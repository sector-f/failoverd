package lua

import (
	"github.com/sector-f/failoverd/internal/ping"
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

func checkGlobalProbeStats(l *lua.LState) map[string]ping.ProbeStats {
	ud := l.CheckUserData(1)
	if v, ok := ud.Value.(map[string]ping.ProbeStats); ok {
		return v
	}
	l.ArgError(1, "global_probe_stats expected")
	return nil
}

func globalProbeStatsGet(l *lua.LState) int {
	gps := checkGlobalProbeStats(l)
	dst := l.CheckString(2)
	stats, ok := gps[dst]
	if !ok {
		l.ArgError(2, "destination not found")
		return 0
	}

	l.Push(&lua.LUserData{
		Value:     &stats,
		Metatable: l.GetTypeMetatable(luaProbeStatsTypeName),
	})

	return 1
}

func globalProbeStatsLowestLoss(l *lua.LState) int {
	gps := checkGlobalProbeStats(l)

	var (
		lowestProbeStats ping.ProbeStats
		first            bool = true
	)

	for _, stats := range gps {
		if first {
			lowestProbeStats = stats
			first = false
			continue
		}

		if stats.Loss < lowestProbeStats.Loss {
			lowestProbeStats = stats
		}
	}

	l.Push(&lua.LUserData{
		Value:     &lowestProbeStats,
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

func checkProbe(l *lua.LState) ping.Probe {
	ud := l.CheckUserData(1)
	if v, ok := ud.Value.(ping.Probe); ok {
		return v
	}
	l.ArgError(1, "probe expected")
	return ping.Probe{}
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

func (e *Engine) registerProbePingerCommands(l *lua.LState) {
	mt := l.GetTypeMetatable(luaProbeTypeName)

	methods := map[string]lua.LGFunction{
		"add":  e.addProbe,
		"stop": e.stopProbe,
	}

	for m, fn := range methods {
		l.SetField(mt, m, l.NewFunction(fn))
	}
}

func (e *Engine) addProbe(l *lua.LState) int {
	if e.pinger == nil {
		l.ArgError(1, "pinger has not been initialized yet")
		return 0
	}

	probe := checkProbe(l)
	e.pinger.AddProbe(probe)

	return 0
}

func (e *Engine) stopProbe(l *lua.LState) int {
	if e.pinger == nil {
		l.ArgError(1, "pinger has not been initialized yet")
		return 0
	}

	dst := l.CheckString(1)
	e.pinger.StopProbe(dst)

	return 0
}

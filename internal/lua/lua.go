package lua

import (
	"fmt"

	"github.com/sector-f/failoverd/internal/ping"
	lua "github.com/yuin/gopher-lua"
)

type Engine struct {
	Config Config

	state  *lua.LState
	pinger *ping.Pinger
}

func New(configFile string) (*Engine, error) {
	lstate := lua.NewState()
	registerTypes(lstate)
	lstate.PreloadModule("dns", (&dnsModule{}).loader)

	err := lstate.DoFile(configFile)
	if err != nil {
		return nil, err
	}

	config, err := configFromLua(lstate)
	if err != nil {
		return nil, err
	}

	e := &Engine{
		Config: config,
		state:  lstate,
	}

	e.registerProbePingerCommands(lstate)

	return e, nil
}

func (e *Engine) SetPinger(p *ping.Pinger) {
	e.pinger = p
}

func (e *Engine) OnRecv(gps map[string]ping.ProbeStats, ps ping.ProbeStats) error {
	if e.Config.onRecvFunc.Type() != lua.LTNil {
		globalProbeStatsUD := &lua.LUserData{
			Value:     gps,
			Metatable: e.state.GetTypeMetatable(luaGlobalProbeStatsTypeName),
		}

		probeStatsUD := &lua.LUserData{
			Value:     &ps,
			Metatable: e.state.GetTypeMetatable(luaProbeStatsTypeName),
		}

		err := e.state.CallByParam(
			lua.P{
				Fn:      e.Config.onRecvFunc,
				NRet:    0,
				Protect: true,
			},
			globalProbeStatsUD,
			probeStatsUD,
		)

		if err != nil {
			return fmt.Errorf("error calling on_recv function: %w\n", err)
		}
	}

	return nil
}

func (e *Engine) OnUpdate(gps map[string]ping.ProbeStats) error {
	if e.Config.onUpdateFunc.Type() != lua.LTNil {
		ud := &lua.LUserData{
			Value:     gps,
			Metatable: e.state.GetTypeMetatable(luaGlobalProbeStatsTypeName),
		}

		err := e.state.CallByParam(
			lua.P{
				Fn:      e.Config.onUpdateFunc,
				NRet:    0,
				Protect: true,
			},
			ud,
		)

		if err != nil {
			return fmt.Errorf("error calling on_update function: %w\n", err)
		}
	}

	return nil
}

func (e *Engine) OnQuit(gps map[string]ping.ProbeStats) error {
	if e.Config.onQuitFunc.Type() != lua.LTNil {
		ud := &lua.LUserData{
			Value:     gps,
			Metatable: e.state.GetTypeMetatable(luaGlobalProbeStatsTypeName),
		}

		err := e.state.CallByParam(
			lua.P{
				Fn:      e.Config.onQuitFunc,
				NRet:    0,
				Protect: true,
			},
			ud,
		)

		if err != nil {
			return fmt.Errorf("error calling on_quit function: %w\n", err)
		}
	}

	return nil
}

func (e *Engine) Close() {
	e.state.Close()
}

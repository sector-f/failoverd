package lua

import (
	"context"
	"net"
	"time"

	lua "github.com/yuin/gopher-lua"
)

type dnsModule struct {
	timeout time.Duration
}

func (m *dnsModule) loader(l *lua.LState) int {
	module := l.SetFuncs(l.NewTable(), map[string]lua.LGFunction{
		"lookup":      m.dnsLookupFunc("ip"),
		"lookup_v4":   m.dnsLookupFunc("ip4"),
		"lookup_v6":   m.dnsLookupFunc("ip6"),
		"set_timeout": m.dnsSetTimeout,
	})
	l.Push(module)
	return 1
}

func (m *dnsModule) dnsSetTimeout(l *lua.LState) int {
	t := l.CheckInt(1)
	m.timeout = time.Duration(t) * time.Millisecond
	return 0
}

func (m *dnsModule) dnsLookupFunc(network string) lua.LGFunction {
	return func(l *lua.LState) int {
		var (
			ctx        context.Context
			cancelFunc func()
		)

		switch m.timeout {
		case 0:
			ctx = context.Background()
		default:
			ctx, cancelFunc = context.WithTimeout(context.Background(), m.timeout)
		}

		if cancelFunc != nil {
			defer cancelFunc()
		}

		host := l.CheckString(1)
		addrs, err := net.DefaultResolver.LookupIP(ctx, network, host)
		if err != nil {
			l.ArgError(1, err.Error())
			return 0
		}

		table := l.NewTable()
		for _, addr := range addrs {
			table.Append(lua.LString(addr.String()))
		}

		l.Push(table)
		return 1
	}
}

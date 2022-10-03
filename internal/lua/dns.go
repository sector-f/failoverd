package lua

import (
	"context"
	"net"

	lua "github.com/yuin/gopher-lua"
)

func newDNSModule(l *lua.LState) int {
	module := l.SetFuncs(l.NewTable(), map[string]lua.LGFunction{
		"lookup":    dnsLookupFunc("ip"),
		"lookup_v4": dnsLookupFunc("ip4"),
		"lookup_v6": dnsLookupFunc("ip6"),
	})
	l.Push(module)
	return 1
}

func dnsLookupFunc(network string) lua.LGFunction {
	return func(l *lua.LState) int {
		host := l.CheckString(1)
		addrs, err := net.DefaultResolver.LookupIP(context.Background(), network, host)
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

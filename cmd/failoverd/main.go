package main

import (
	"github.com/sector-f/failover"
)

func main() {
	f := failover.NewFailover()
	f.Run()
}

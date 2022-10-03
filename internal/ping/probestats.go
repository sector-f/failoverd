package ping

type ProbeStats struct {
	Src  string
	Dst  string
	Loss float64
}

type GlobalProbeStats struct {
	Stats map[string]ProbeStats
}

func (gps GlobalProbeStats) Get(dst string) ProbeStats {
	return gps.Stats[dst]
}

func (gps *GlobalProbeStats) Remove(dst string) {
	delete(gps.Stats, dst)
}

func (gps GlobalProbeStats) LowestLoss() ProbeStats {
	var (
		lowestProbeStats ProbeStats
		first            bool = true
	)

	for _, stats := range gps.Stats {
		if first {
			lowestProbeStats = stats
			first = false
			continue
		}

		if stats.Loss < lowestProbeStats.Loss {
			lowestProbeStats = stats
		}
	}

	return lowestProbeStats
}

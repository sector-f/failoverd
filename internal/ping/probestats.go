package ping

type ProbeStats struct {
	Src  string
	Dst  string
	Loss float64
}

/*
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
*/

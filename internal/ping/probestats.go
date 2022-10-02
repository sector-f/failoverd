package ping

type ProbeStats struct {
	Src  string
	Dst  string
	Loss float64
}

type GlobalProbeStats struct {
	Stats map[string]ProbeStats

	probes     []Probe
	resolveMap map[string]string
}

func (gps GlobalProbeStats) Get(dst string) ProbeStats {
	return gps.Stats[gps.resolveMap[dst]]
}

func (gps GlobalProbeStats) LowestLoss() ProbeStats {
	var lowestProbeStats ProbeStats

	for i, p := range gps.probes {
		stats := gps.Stats[p.Dst]

		if i == 0 {
			lowestProbeStats = stats
			continue
		}

		if stats.Loss < lowestProbeStats.Loss {
			lowestProbeStats = stats
		}
	}

	return lowestProbeStats
}

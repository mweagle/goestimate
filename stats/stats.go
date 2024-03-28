package stats

import (
	"sort"

	gonumstat "gonum.org/v1/gonum/stat"
)

type AggregatedStatistics struct {
	Mean        float64
	Median      float64
	StdDev      float64
	Percentiles []float64
}

func StatsForSequence(unsortedSamples []float64, percentiles []float64) *AggregatedStatistics {
	sortedSamples := make([]float64, len(unsortedSamples))
	copy(sortedSamples, unsortedSamples)
	sort.Float64s(sortedSamples)

	// Compute aggregates...
	mean, stddev := gonumstat.MeanStdDev(sortedSamples, nil)
	median := gonumstat.Quantile(0.5, gonumstat.Empirical, sortedSamples, nil)
	aggStats := &AggregatedStatistics{
		Mean:        mean,
		Median:      median,
		StdDev:      stddev,
		Percentiles: make([]float64, len(percentiles)),
	}

	for eachPercentileIndex := range percentiles {
		percentileValue := percentiles[eachPercentileIndex]
		if percentileValue > 1.00 {
			percentileValue = percentileValue / 100
		}
		aggStats.Percentiles[eachPercentileIndex] = gonumstat.Quantile(percentileValue,
			gonumstat.Empirical,
			sortedSamples,
			nil)
	}
	return aggStats
}

package stats

import (
	"sort"

	gonumstat "gonum.org/v1/gonum/stat"
)

type PercentilePair struct {
	P   float64
	Val float64
}
type AggregatedStatistics struct {
	Mean        float64
	Median      float64
	StdDev      float64
	Percentiles []*PercentilePair
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
		Percentiles: make([]*PercentilePair, len(percentiles)),
	}

	for eachPercentileIndex := range percentiles {
		percentileValue := percentiles[eachPercentileIndex]
		if percentileValue > 1.00 {
			percentileValue = percentileValue / 100
		}
		quantValue := gonumstat.Quantile(percentileValue,
			gonumstat.Empirical,
			sortedSamples,
			nil)
		aggStats.Percentiles[eachPercentileIndex] = &PercentilePair{
			P:   percentileValue,
			Val: quantValue,
		}
	}
	return aggStats
}

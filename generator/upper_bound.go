package generator

import (
	"fmt"
	"log/slog"
	"math"

	"golang.org/x/exp/rand"
)

// /////////////////////////////////////////////////////////////////////////////
type UpperBoundGenerator struct {
	BaseGenerator
}

func (ubg *UpperBoundGenerator) Name() string {
	return "UpperBoundGenerator"
}

func (ubg *UpperBoundGenerator) Unmarshal(typeParameter string, log *slog.Logger) error {
	return fmt.Errorf("unmarshal not supported for UpperBoundGenerator")
}

func (ubg *UpperBoundGenerator) Generate(priorSamples map[int64]*GenerationResults,
	percentiles []float64,
	src rand.Source,
	log *slog.Logger) (*GenerationResults, error) {

	if len(priorSamples) <= 0 {
		return nil, fmt.Errorf("no slices provided to UpperBoundGenerator")
	}
	priorSampleKeys := make([]int64, len(priorSamples))
	i := 0
	for eachKey := range priorSamples {
		priorSampleKeys[i] = eachKey
		i++
	}

	// Extract the upper bound, then run that through the aggregation function
	var maxValues []float64
	if len(priorSampleKeys) == 1 {
		maxValues = *priorSamples[priorSampleKeys[0]].RawValues
	} else {
		// Use the first sample to determine the sample size
		sampleSize := len(*priorSamples[priorSampleKeys[0]].RawValues)
		maxValues = make([]float64, sampleSize)
		for sampleIdx := 0; sampleIdx != sampleSize; sampleIdx++ {
			curMax := math.SmallestNonzeroFloat64
			for priorSeriesIdx := 0; priorSeriesIdx != len(priorSampleKeys); priorSeriesIdx++ {
				priorData := priorSamples[priorSampleKeys[priorSeriesIdx]]
				priorRawValues := *priorData.RawValues
				curMax = math.Max(curMax, priorRawValues[sampleIdx])
			}
			maxValues[sampleIdx] = curMax
		}
	}
	generatorValues := make([]float64, len(maxValues))
	return ubg.computeAggregates(generatorValues, maxValues, percentiles, log)
}

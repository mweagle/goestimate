package generator

import (
	"log/slog"

	"golang.org/x/exp/rand"
	"gonum.org/v1/gonum/stat/distuv"
)

type UniformDurationGenerator struct {
	BaseGenerator
	lowerBound float64
	upperBound float64
}

func (fdg *UniformDurationGenerator) Generate(priorSamples map[int64]*GenerationResults,
	percentiles []float64,
	src rand.Source,
	log *slog.Logger) (*GenerationResults, error) {
	generator := &distuv.Uniform{
		Min: fdg.lowerBound,
		Max: fdg.upperBound,
		Src: src}

	// Delegate to the Base generator
	return fdg.BaseGenerator.Generate(generator, priorSamples, percentiles, log)
}

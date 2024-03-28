package generator

import (
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"strings"

	"golang.org/x/exp/rand"
	"gonum.org/v1/gonum/stat/distuv"
)

type ParetoGenerator struct {
	BaseGenerator
	x        float64
	alpha    float64
	maxValue float64
}

func (pg *ParetoGenerator) Name() string {
	maxSuffix := ""
	if pg.maxValue != math.MaxFloat64 {
		maxSuffix = fmt.Sprintf(", max: %.2f", pg.maxValue)
	}
	return fmt.Sprintf("Pareto(Xmin = %.2f, α= %.2f%s)",
		pg.x,
		pg.alpha,
		maxSuffix)
}

func (pg *ParetoGenerator) Generate(priorSamples map[int64]*GenerationResults,
	percentiles []float64,
	src rand.Source,
	log *slog.Logger) (*GenerationResults, error) {
	generator := &distuv.Pareto{
		Xm:    pg.x,
		Alpha: pg.alpha,
		Src:   src}

	// Delegate to the Base generator
	filterFunc := func(genSample float64) float64 {
		return math.Min(pg.maxValue, genSample)
	}
	return pg.BaseGenerator.FilterGenerate(generator, filterFunc, priorSamples, percentiles, log)
}

func UnmarshalPareto(typeParameter string, log *slog.Logger) (DurationGenerator, error) {
	// Supported forms:
	// Pareto(Xmin, alphaShape)
	pg := &ParetoGenerator{}
	reParams := regexp.MustCompile(`[()]`)
	paretoParts := reParams.Split(typeParameter, -1)
	if len(paretoParts) < 2 {
		return nil, fmt.Errorf("invalid Pareto generator expression: %s", typeParameter)
	}
	paretoFloatParts := strings.Split(paretoParts[1], ",")

	if len(paretoFloatParts) >= 2 {
		err := pg.BaseGenerator.parseFloat(strings.TrimSpace(paretoFloatParts[0]), &pg.x)
		if err != nil {
			return nil, err
		}
		err = pg.BaseGenerator.parseFloat(strings.TrimSpace(paretoFloatParts[1]), &pg.alpha)
		if err != nil {
			return nil, err
		}
		pg.maxValue = math.MaxFloat64
		if len(paretoFloatParts) == 3 {
			err = pg.BaseGenerator.parseFloat(strings.TrimSpace(paretoFloatParts[2]), &pg.maxValue)
			if err != nil {
				return nil, err
			}
		}

	} else {
		return nil, fmt.Errorf("invalid pareto generator expression: %s", typeParameter)
	}
	return pg, nil
}

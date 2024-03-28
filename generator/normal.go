package generator

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"golang.org/x/exp/rand"
	"gonum.org/v1/gonum/stat/distuv"
)

// /////////////////////////////////////////////////////////////////////////////
// _  _                    _
// | \| |___ _ _ _ __  __ _| |
// | .` / _ \ '_| '  \/ _` | |
// |_|\_\___/_| |_|_|_\__,_|_|
//
// /////////////////////////////////////////////////////////////////////////////
type NormalGenerator struct {
	BaseGenerator
	mean   float64
	stddev float64
}

func (ng *NormalGenerator) Name() string {
	return fmt.Sprintf("Normal(μ = %.2f, σ= %.2f)",
		ng.mean,
		ng.stddev)
}

func (ng *NormalGenerator) Generate(priorSamples map[int64]*GenerationResults,
	percentiles []float64,
	src rand.Source,
	log *slog.Logger) (*GenerationResults, error) {
	generator := distuv.Normal{
		Mu:    ng.mean,
		Sigma: ng.stddev,
		Src:   src,
	}
	// Delegate to the Base generator
	return ng.BaseGenerator.Generate(generator, priorSamples, percentiles, log)
}

func UnmarshalNormal(typeParameter string, log *slog.Logger) (DurationGenerator, error) {
	// Supported forms:
	// Normal(mean, stddev)
	ng := &NormalGenerator{}
	reParams := regexp.MustCompile(`[()]`)
	normalParts := reParams.Split(typeParameter, -1)
	if len(normalParts) < 2 {
		return nil, fmt.Errorf("invalid Normal generator expression: %s", typeParameter)
	}
	normalFloatParts := strings.Split(normalParts[1], ",")

	if len(normalFloatParts) == 2 {
		err := ng.BaseGenerator.parseFloat(strings.TrimSpace(normalFloatParts[0]), &ng.mean)
		if err != nil {
			return nil, err
		}
		err = ng.BaseGenerator.parseFloat(strings.TrimSpace(normalFloatParts[1]), &ng.stddev)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("invalid normal generator expression: %s", typeParameter)
	}
	return ng, nil
}

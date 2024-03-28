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
//
// ___ _            _
// | __(_)_ _____ __| |
// | _|| \ \ / -_) _` |
// |_| |_/_\_\___\__,_|
//
// /////////////////////////////////////////////////////////////////////////////
type FixedGenerator struct {
	BaseGenerator
	value float64
}

func (ng *FixedGenerator) Name() string {
	return fmt.Sprintf("Fixed(v = %.2f)",
		ng.value)
}

func (ng *FixedGenerator) Generate(priorSamples map[int64]*GenerationResults,
	percentiles []float64,
	src rand.Source,
	log *slog.Logger) (*GenerationResults, error) {
	generator := distuv.Uniform{
		Min: ng.value,
		Max: ng.value,
		Src: src,
	}
	// Delegate to the Base generator
	return ng.BaseGenerator.Generate(generator, priorSamples, percentiles, log)
}

func UnmarshalFixed(typeParameter string, log *slog.Logger) (DurationGenerator, error) {
	// Supported forms:
	// Fixed(value)
	ng := &FixedGenerator{}
	reParams := regexp.MustCompile(`[()]`)
	normalParts := reParams.Split(typeParameter, -1)
	if len(normalParts) < 2 {
		return nil, fmt.Errorf("invalid Fixed generator expression: %s", typeParameter)
	}
	normalFloatParts := strings.Split(normalParts[1], ",")

	if len(normalFloatParts) == 1 {
		err := ng.BaseGenerator.parseFloat(strings.TrimSpace(normalFloatParts[0]), &ng.value)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("invalid normal generator expression: %s", typeParameter)
	}
	return ng, nil
}

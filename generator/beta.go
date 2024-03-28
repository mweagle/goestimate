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
// ___      _
// | _ ) ___| |_ __ _
// | _ \/ -_)  _/ _` |
// |___/\___|\__\__,_|
//
// /////////////////////////////////////////////////////////////////////////////
type BetaGenerator struct {
	BaseGenerator
	alpha float64
	beta  float64
}

func (bg *BetaGenerator) Name() string {
	return fmt.Sprintf("Beta(α=%.2f, β=%.2f)",
		bg.alpha,
		bg.beta)
}

func (bg *BetaGenerator) Generate(priorSamples map[int64]*GenerationResults,
	percentiles []float64,
	src rand.Source,
	log *slog.Logger) (*GenerationResults, error) {
	generator := distuv.Beta{
		Alpha: bg.alpha,
		Beta:  bg.beta,
	}

	// Delegate to the Base generator
	return bg.BaseGenerator.Generate(generator, priorSamples, percentiles, log)
}

func UnmarshalBeta(typeParameter string, log *slog.Logger) (DurationGenerator, error) {
	// Supported forms:
	// Bernoulli(prob)
	bg := &BetaGenerator{}
	reParams := regexp.MustCompile(`[()]`)
	bernoulliParts := reParams.Split(typeParameter, -1)
	if len(bernoulliParts) < 2 {
		return nil, fmt.Errorf("invalid Beta generator expression: %s", typeParameter)
	}
	bernoulliVals := strings.Split(bernoulliParts[1], ",")

	if len(bernoulliVals) == 2 {
		err := bg.BaseGenerator.parseFloat(strings.TrimSpace(bernoulliVals[0]), &bg.alpha)
		if err != nil {
			return nil, err
		}
		err = bg.BaseGenerator.parseFloat(strings.TrimSpace(bernoulliVals[1]), &bg.beta)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("invalid Beta generator expression: %s", typeParameter)
	}
	return bg, nil
}

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
// ___                        _ _ _
// | _ ) ___ _ _ _ _  ___ _  _| | (_)
// | _ \/ -_) '_| ' \/ _ \ || | | | |
// |___/\___|_| |_||_\___/\_,_|_|_|_|
//
// /////////////////////////////////////////////////////////////////////////////
type BernoulliGenerator struct {
	BaseGenerator
	prob float64
}

func (bg *BernoulliGenerator) Name() string {
	return fmt.Sprintf("Bernoulli(%.2f, )",
		bg.prob)
}

func (bg *BernoulliGenerator) Generate(priorSamples map[int64]*GenerationResults,
	percentiles []float64,
	src rand.Source,
	log *slog.Logger) (*GenerationResults, error) {
	generator := distuv.Bernoulli{
		P:   bg.prob,
		Src: src}

	// Delegate to the Base generator
	return bg.BaseGenerator.Generate(generator, priorSamples, percentiles, log)
}

func UnmarshalBernoulli(typeParameter string, log *slog.Logger) (DurationGenerator, error) {
	// Supported forms:
	// Bernoulli(prob)
	bg := &BernoulliGenerator{}
	reParams := regexp.MustCompile(`[()]`)
	bernoulliParts := reParams.Split(typeParameter, -1)
	if len(bernoulliParts) < 2 {
		return nil, fmt.Errorf("invalid Bernoulli generator expression: %s", typeParameter)
	}
	bernoulliVals := strings.Split(bernoulliParts[1], ",")

	if len(bernoulliVals) == 1 {
		err := bg.BaseGenerator.parseFloat(strings.TrimSpace(bernoulliVals[0]), &bg.prob)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("invalid Bernoulli generator expression: %s", typeParameter)
	}
	return bg, nil
}

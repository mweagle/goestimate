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
// ___ ___ ___ _____
// | _ \ __| _ \_   _|
// |  _/ _||   / | |
// |_| |___|_|_\ |_|
//
// /////////////////////////////////////////////////////////////////////////////
type PERTGenerator struct {
	BaseGenerator
	min  float64
	mode float64
	max  float64
}

func (pg *PERTGenerator) Validate() error {
	var validationError error
	if (pg.min >= pg.max) ||
		(pg.min > pg.mode) ||
		(pg.mode > pg.max) {
		validationError = fmt.Errorf("inavlid PERT distribution: (lower=%.2f, upper=%.2f, mode=%.2f). Distribution must satisfy: lower <= mode <= upper",
			pg.min,
			pg.max,
			pg.mode)
	}
	return validationError
}

func (pg *PERTGenerator) Name() string {
	return fmt.Sprintf("PERT(%.2f, %.2f, %.2f)",
		pg.min,
		pg.mode,
		pg.max)
}

func (pg *PERTGenerator) Generate(priorSamples map[int64]*GenerationResults,
	percentiles []float64,
	src rand.Source,
	log *slog.Logger) (*GenerationResults, error) {
	generator := distuv.NewTriangle(pg.min, pg.max, pg.mode, src)

	// Delegate to the Base generator
	return pg.BaseGenerator.Generate(generator, priorSamples, percentiles, log)
}

func UnmarshalPERT(typeParameter string, log *slog.Logger) (DurationGenerator, error) {
	// Supported forms:
	// PERT(n)
	// PERT(min, mode, max)
	pg := &PERTGenerator{}
	reParams := regexp.MustCompile(`[()]`)
	pertParts := reParams.Split(typeParameter, -1)
	if len(pertParts) < 2 {
		return nil, fmt.Errorf("invalid PERT generator expression: %s", typeParameter)
	}
	pertFloatParts := strings.Split(pertParts[1], ",")
	var floatErr error
	if len(pertFloatParts) == 1 {
		floatErr = pg.BaseGenerator.parseFloat(pertFloatParts[0], &pg.mode)
		if floatErr != nil {
			return nil, floatErr
		}
		pg.max = pg.mode
		pg.min = pg.mode
	} else if len(pertFloatParts) == 3 {
		floatErr = pg.BaseGenerator.parseFloat(pertFloatParts[0], &pg.min)
		if floatErr != nil {
			return nil, floatErr
		}
		floatErr = pg.BaseGenerator.parseFloat(pertFloatParts[1], &pg.mode)
		if floatErr != nil {
			return nil, floatErr
		}
		floatErr = pg.BaseGenerator.parseFloat(pertFloatParts[2], &pg.max)
		if floatErr != nil {
			return nil, floatErr
		}
	} else {
		return nil, fmt.Errorf("invalid PERT generator expression: %s", typeParameter)
	}
	// Check the values
	return pg, pg.Validate()
}

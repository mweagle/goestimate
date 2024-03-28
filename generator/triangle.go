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
/*
  _____    _                _
  |_   _| _(_)__ _ _ _  __ _| |___
  	| || '_| / _` | ' \/ _` | / -_)
  	|_||_| |_\__,_|_||_\__, |_\___|
  	                   |___/
*/
// /////////////////////////////////////////////////////////////////////////////
type TriangularGenerator struct {
	BaseGenerator
	min  float64
	mode float64
	max  float64
}

func (tg *TriangularGenerator) Validate() error {
	var validationError error
	if (tg.min >= tg.max) ||
		(tg.min > tg.mode) ||
		(tg.mode > tg.max) {
		validationError = fmt.Errorf("inavlid Triangle distribution: (lower=%.2f, upper=%.2f, mode=%.2f). Distribution must satisfy: lower <= mode <= upper",
			tg.min,
			tg.max,
			tg.mode)
	}
	return validationError
}

func (tg *TriangularGenerator) Name() string {
	return fmt.Sprintf("Triangle(%.2f, %.2f, %.2f)",
		tg.min,
		tg.mode,
		tg.max)
}

func (tg *TriangularGenerator) Generate(priorSamples map[int64]*GenerationResults,
	percentiles []float64,
	src rand.Source,
	log *slog.Logger) (*GenerationResults, error) {
	generator := distuv.NewTriangle(tg.min, tg.max, tg.mode, src)

	// Delegate to the Base generator
	return tg.BaseGenerator.Generate(generator, priorSamples, percentiles, log)
}

func UnmarshalTriangle(typeParameter string, log *slog.Logger) (DurationGenerator, error) {
	// Supported forms:
	// Triangle(n)
	// Triangle(min, mode, max)
	tg := &TriangularGenerator{}
	reParams := regexp.MustCompile(`[()]`)
	pertParts := reParams.Split(typeParameter, -1)
	if len(pertParts) < 2 {
		return nil, fmt.Errorf("invalid PERT generator expression: %s", typeParameter)
	}
	pertFloatParts := strings.Split(pertParts[1], ",")
	var floatErr error
	if len(pertFloatParts) == 1 {
		floatErr = tg.BaseGenerator.parseFloat(pertFloatParts[0], &tg.mode)
		if floatErr != nil {
			return nil, floatErr
		}
		tg.max = tg.mode
		tg.min = tg.mode
	} else if len(pertFloatParts) == 3 {
		floatErr = tg.BaseGenerator.parseFloat(pertFloatParts[0], &tg.min)
		if floatErr != nil {
			return nil, floatErr
		}
		floatErr = tg.BaseGenerator.parseFloat(pertFloatParts[1], &tg.mode)
		if floatErr != nil {
			return nil, floatErr
		}
		floatErr = tg.BaseGenerator.parseFloat(pertFloatParts[2], &tg.max)
		if floatErr != nil {
			return nil, floatErr
		}
	} else {
		return nil, fmt.Errorf("invalid PERT generator expression: %s", typeParameter)
	}
	// Check the values
	return tg, tg.Validate()
}

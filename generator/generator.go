package generator

import (
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/mweagle/goestimate/json"
	"github.com/mweagle/goestimate/stats"

	"golang.org/x/exp/rand"
	"gonum.org/v1/gonum/stat/distuv"
)

type GenerationResults struct {
	RawValues        *[]float64
	GeneratorStats   *stats.AggregatedStatistics
	CumulativeValues *[]float64
	CumulativeStats  *stats.AggregatedStatistics
}

type DurationGenerator interface {
	Name() string
	Generate(priorSamples map[int64]*GenerationResults,
		percentiles []float64,
		src rand.Source,
		log *slog.Logger) (*GenerationResults, error)
	GenerationResults() *GenerationResults
}

type generatorFilter func(in float64) float64

type unmarshalFunc func(string, *slog.Logger) (DurationGenerator, error)

var unmarshalMap map[string]unmarshalFunc

func init() {
	// The map of supported distributions. The upper bound generator doesn't
	// take any parameters and is implicitly created as part of the graph
	// creation
	unmarshalMap = map[string]unmarshalFunc{
		"PERT":      UnmarshalPERT,
		"Pareto":    UnmarshalPareto,
		"Normal":    UnmarshalNormal,
		"Fixed":     UnmarshalFixed,
		"Bernoulli": UnmarshalBernoulli,
		"Beta":      UnmarshalBeta,
		"Triangle":  UnmarshalTriangle,
	}
}

// /////////////////////////////////////////////////////////////////////////////
// ___                ___                       _
// | _ ) __ _ ___ ___ / __|___ _ _  ___ _ _ __ _| |_ ___ _ _
// | _ \/ _` (_-</ -_) (_ / -_) ' \/ -_) '_/ _` |  _/ _ \ '_|
// |___/\__,_/__/\___|\___\___|_||_\___|_| \__,_|\__\___/_|
//
// /////////////////////////////////////////////////////////////////////////////

type BaseGenerator struct {
	rawValues        []float64
	cumulativeValues []float64
	generatorStats   *stats.AggregatedStatistics
	cumulativeStats  *stats.AggregatedStatistics
}

func (bg *BaseGenerator) parseFloat(strVal string, target *float64) error {

	trimmedVal := strings.TrimSpace(strVal)
	parseVal, parseValErr := strconv.ParseFloat(trimmedVal, 64)
	if parseValErr != nil {
		return parseValErr
	}
	*target = parseVal
	return nil
}

func (bg *BaseGenerator) GenerationResults() *GenerationResults {
	return &GenerationResults{
		RawValues:        &bg.rawValues,
		CumulativeValues: &bg.cumulativeValues,
		GeneratorStats:   bg.generatorStats,
		CumulativeStats:  bg.cumulativeStats,
	}
}

func (bg *BaseGenerator) computeAggregates(generatorSamples []float64,
	incomingCumulativeValues []float64,
	percentiles []float64,
	_ *slog.Logger) (*GenerationResults, error) {

	bg.rawValues = generatorSamples
	bg.generatorStats = stats.StatsForSequence(generatorSamples, percentiles)
	bg.cumulativeValues = make([]float64, len(generatorSamples))
	for i := 0; i != len(generatorSamples); i++ {
		bg.cumulativeValues[i] = incomingCumulativeValues[i] + generatorSamples[i]
	}
	bg.cumulativeStats = stats.StatsForSequence(bg.cumulativeValues, percentiles)
	return bg.GenerationResults(), nil
}
func (bg *BaseGenerator) FilterGenerate(rander distuv.Rander,
	filter generatorFilter,
	priorSamples map[int64]*GenerationResults,
	percentiles []float64,
	log *slog.Logger) (*GenerationResults, error) {

	if len(priorSamples) != 1 {
		return nil, fmt.Errorf("invalid length for prior samples: %d", len(priorSamples))
	}

	var genResults *GenerationResults
	for _, val := range priorSamples {
		genResults = val
	}
	generatedSamples := make([]float64, len(*genResults.RawValues))
	var genValue float64
	for i := range generatedSamples {
		genValue = rander.Rand()
		if filter != nil {
			genValue = filter(genValue)
		}
		generatedSamples[i] = genValue
	}
	return bg.computeAggregates(generatedSamples, *genResults.CumulativeValues, percentiles, log)
}

func (bg *BaseGenerator) Generate(rander distuv.Rander,
	priorSamples map[int64]*GenerationResults,
	percentiles []float64,
	log *slog.Logger) (*GenerationResults, error) {

	return bg.FilterGenerate(rander, nil, priorSamples, percentiles, log)

}
func NewDurationGenerator(dictActivityParams map[string]interface{}, log *slog.Logger) (DurationGenerator, error) {
	generatorType := json.String("type", dictActivityParams)

	// All generators satisfy:
	// GENERATOR(...)
	reSplit := regexp.MustCompile(`[\(\)]`)
	generatorParts := reSplit.Split(generatorType, -1)
	generatorBasename := strings.TrimSpace(generatorParts[0])

	// The generator type is always the distribution name before the
	// opening parens
	unmarshalFunc, unmarshalFuncExists := unmarshalMap[generatorBasename]
	if !unmarshalFuncExists {
		// Get all the keys
		unmarshalTypes := []string{}
		for eachKey := range unmarshalMap {
			unmarshalTypes = append(unmarshalTypes, eachKey)
		}
		return nil, fmt.Errorf("unsupported generator function name: %s. Supported types: %v", generatorBasename, unmarshalTypes)
	}
	return unmarshalFunc(generatorType, log)
}

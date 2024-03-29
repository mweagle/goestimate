package app

import (
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mweagle/goestimate/generator"
	goejson "github.com/mweagle/goestimate/json"
	"github.com/mweagle/goestimate/stats"
	"github.com/rickar/cal/v2"
	"github.com/rickar/cal/v2/us"
	"golang.org/x/exp/rand"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/encoding/dot"
	"gonum.org/v1/gonum/graph/path"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/graph/topo"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

var businessCalendar *cal.BusinessCalendar
var nowTime time.Time
var ECD_TIME_FORMAT = "Mon, 02 Jan 2006 MST"

func init() {
	nowTime = time.Now()
}

func workdayWithOffset(offset float64) time.Time {
	var once sync.Once
	onceBody := func() {
		businessCalendar = cal.NewBusinessCalendar()
		businessCalendar.Name = "goestimate."
		businessCalendar.Description = "Default company calendar"
		// add holidays that the business observes
		businessCalendar.AddHoliday(
			us.NewYear,
			us.MemorialDay,
			us.IndependenceDay,
			us.LaborDay,
			us.ThanksgivingDay,
			us.ChristmasDay,
		)
	}
	once.Do(onceBody)
	return businessCalendar.WorkdaysFrom(nowTime, int(math.Ceil(offset)))
}

func aggregatedStatsFormatter(aggStats *stats.AggregatedStatistics) string {
	label := fmt.Sprintf("μ=%.2f, σ=%.2f", aggStats.Mean, aggStats.StdDev)
	if len(aggStats.Percentiles) != 0 {
		value := ""
		for i := 0; i != len(aggStats.Percentiles); i++ {
			percentilePair := aggStats.Percentiles[i]
			pVal := percentilePair.P
			if pVal < 1 {
				pVal *= 100
			}
			if math.Floor(pVal) == pVal {
				value += fmt.Sprintf("p%.0f=%.2f, ", pVal, percentilePair.Val)
			} else {
				value += fmt.Sprintf("p%.2f=%.2f, ", pVal, percentilePair.Val)
			}
		}
		value = strings.TrimSuffix(value, ", ")
		label = fmt.Sprintf("%s (%s)", label, value)
	}
	return label
}

// /////////////////////////////////////////////////////////////////////////////
//
// TYPES
//
// /////////////////////////////////////////////////////////////////////////////

type d2TableParams struct {
	Key   string
	Value interface{}
}

type D2Encoding struct {
	Name   string
	Params []*d2TableParams
}

// DurationGeneratorGraphNode is the interface that all flow graph nodes satisfy
// so that we can evaluate them.
type DurationGeneratorGraphNode interface {
	Generate(
		flowGraph *flowGraph,
		percentiles []float64,
		src rand.Source,
		log *slog.Logger) (*generator.GenerationResults, error)
	ID() int64
}

type AggregationOptions struct {
	workdays bool
}

// /////////////////////////////////////////////////////////////////////////////
// flowGraphNode
//
// # Base node for all gonum nodes that wrap a generator function
//
// /////////////////////////////////////////////////////////////////////////////
type flowGraphNode struct {
	id                 int64
	name               string
	aggregationOptions *AggregationOptions
	// complete       bool
	// comments       string
	parentFlowSubgraphs []*flowSubgraph
	generator           generator.DurationGenerator
}

func (fgn *flowGraphNode) AbsoluteNodePath() []int64 {
	absPath := make([]int64, 0)
	if fgn.parentFlowSubgraphs != nil {
		for i := 0; i != len(fgn.parentFlowSubgraphs); i++ {
			subgraphNode := fgn.parentFlowSubgraphs[i].inputNode
			absPath = append(absPath, subgraphNode.ID())
		}
	}

	if (len(absPath) <= 0) || (absPath[len(absPath)-1] != fgn.ID()) {
		absPath = append(absPath, fgn.id)
	}
	return absPath
}

func (fgn *flowGraphNode) GenerationResults() *generator.GenerationResults {
	if nil != fgn.generator {
		return fgn.generator.GenerationResults()
	}
	return &generator.GenerationResults{}
}

func (fgn *flowGraphNode) encodeD2MarkdownNode(heading string,
	params map[string]interface{},
	output io.StringWriter,
	log *slog.Logger) error {
	var writeErr error
	log.Debug("Markdown encoding node", "title", heading)
	_, writeErr = output.WriteString(fmt.Sprintf("%d : |md\n", fgn.ID()))
	if writeErr != nil {
		return writeErr
	}
	_, writeErr = output.WriteString(fmt.Sprintf("# %s\n", heading))
	if writeErr != nil {
		return writeErr
	}
	for eachKey, eachVal := range params {
		_, writeErr = output.WriteString(fmt.Sprintf("- **%s**: %v\n", eachKey, eachVal))
		if writeErr != nil {
			return writeErr
		}
	}
	_, writeErr = output.WriteString("|\n")
	if writeErr != nil {
		return writeErr
	}
	_, writeErr = output.WriteString("\n")
	if writeErr != nil {
		return writeErr
	}
	return nil
}

func (fgn *flowGraphNode) ID() int64 {
	return fgn.id
}

func (fgn *flowGraphNode) Generate(flowGraph *flowGraph,
	percentiles []float64,
	src rand.Source,
	log *slog.Logger) (*generator.GenerationResults, error) {
	// We'll compute the generator and cumulative stats at each node
	priorSamples, priorSamplesErr := flowGraph.PredecessorValues(fgn.ID())
	if priorSamplesErr != nil {
		return nil, priorSamplesErr
	}
	return fgn.generator.Generate(priorSamples, percentiles, src, log)
}

func (fgn *flowGraphNode) DOTID() string {
	subgraphPath := ""
	for i := 0; i != len(fgn.parentFlowSubgraphs); i++ {
		subgraphPath += fmt.Sprintf("%d.", fgn.parentFlowSubgraphs[i].inputNode.id)
	}
	subgraphPath = strings.TrimSuffix(subgraphPath, ".")
	return fmt.Sprintf("Node: %s\nID: %d\nGenerator Type: %T\nPARENT SUBGRAPHS: %v",
		fgn.name,
		fgn.ID(),
		fgn.generator,
		subgraphPath)
}

func (fgn *flowGraphNode) D2Encode(output io.StringWriter, indent string, log *slog.Logger) error {
	var writeErr error
	encodeParams, encodeParamsErr := fgn.D2Params(log)
	if encodeParamsErr != nil {
		return encodeParamsErr
	}
	shape := "sql_table"
	if len(encodeParams.Params) <= 0 {
		shape = "cloud"
	}
	_, writeErr = output.WriteString(fmt.Sprintf("%s%d : %s {\n", indent, fgn.ID(), encodeParams.Name))
	if writeErr != nil {
		return writeErr
	}
	_, writeErr = output.WriteString(fmt.Sprintf("%s\tshape: %s\n", indent, shape))
	if writeErr != nil {
		return writeErr
	}
	for _, eachParam := range encodeParams.Params {
		_, writeErr = output.WriteString(fmt.Sprintf("%s\t%s: %v\n", indent, eachParam.Key, eachParam.Value))
		if writeErr != nil {
			return writeErr
		}
	}
	_, writeErr = output.WriteString(fmt.Sprintf("%s}\n", indent))
	if writeErr != nil {
		return writeErr
	}
	return nil
}

func (fgn *flowGraphNode) D2Params(_ *slog.Logger) (*D2Encoding, error) {
	encoding := &D2Encoding{
		Name:   fgn.name,
		Params: []*d2TableParams{},
	}

	if fgn.generator != nil {

		genResults := fgn.generator.GenerationResults()
		incrementalValue := aggregatedStatsFormatter(genResults.GeneratorStats)

		encoding.Params = append(encoding.Params,
			&d2TableParams{
				Key:   "Type",
				Value: fgn.generator.Name(),
			},
			&d2TableParams{
				Key:   "+",
				Value: incrementalValue,
			},
		)
		if genResults.CumulativeStats != nil {
			encoding.Params = append(encoding.Params, &d2TableParams{
				Key:   "∑",
				Value: aggregatedStatsFormatter(genResults.CumulativeStats),
			})
		}

		if fgn.aggregationOptions != nil && fgn.aggregationOptions.workdays {
			encoding.Params = append(encoding.Params, &d2TableParams{
				Key:   "ECD",
				Value: workdayWithOffset(genResults.CumulativeStats.Mean).Format(ECD_TIME_FORMAT),
			})

		}
	}
	return encoding, nil
}

// /////////////////////////////////////////////////////////////////////////////
// flowGraphJoinMaxValueNode
//
// There are two other types of nodes. The input node, which is the
// primary entrypoint for a flowgraph and is a pass through duration generator
// and the join node, which takes the pairwise max of all elements in the array
//
// /////////////////////////////////////////////////////////////////////////////
type flowGraphJoinMaxValueNode struct {
	flowGraphNode
}

func (fgj *flowGraphJoinMaxValueNode) Generate(flowGraph *flowGraph,
	percentiles []float64,
	src rand.Source,
	log *slog.Logger) (*generator.GenerationResults, error) {

	priorSamples, priorSamplesErr := flowGraph.PredecessorValues(fgj.ID())
	if priorSamplesErr != nil {
		return nil, priorSamplesErr
	}

	// The upper bound is a logical operation that works on cumulative values
	// rather than generator values. Take the existing samples and mock this
	// behavior by promoting the cumulative values to the incremental raw values
	cumulativeSamples := make(map[int64]*generator.GenerationResults, len(priorSamples))
	for eachID, eachGenResult := range priorSamples {
		cumulativeSamples[eachID] = &generator.GenerationResults{
			RawValues:        eachGenResult.CumulativeValues,
			GeneratorStats:   eachGenResult.CumulativeStats,
			CumulativeValues: eachGenResult.CumulativeValues,
			CumulativeStats:  eachGenResult.CumulativeStats,
		}
	}
	return fgj.generator.Generate(cumulativeSamples, percentiles, src, log)
}

func (fgj *flowGraphJoinMaxValueNode) D2Encode(output io.StringWriter,
	_ string,
	log *slog.Logger) error {
	genStats := fgj.generator.GenerationResults()
	cumulativeValue := aggregatedStatsFormatter(genStats.CumulativeStats)

	markdownParams := map[string]interface{}{
		"Cumulative": cumulativeValue,
	}
	// Optional aggregation options
	if fgj.aggregationOptions != nil {
		if fgj.aggregationOptions.workdays {
			markdownParams["Estimated"] = workdayWithOffset(genStats.CumulativeStats.Mean).Format(ECD_TIME_FORMAT)
		}
	} else {
		log.Debug("No aggregation options for node", "id", fgj.flowGraphNode.id, "type", fmt.Sprintf("%T", fgj))
	}
	return fgj.encodeD2MarkdownNode(fgj.name,
		markdownParams,
		output,
		log)
}

func (fgj *flowGraphJoinMaxValueNode) DOTID() string {
	return fmt.Sprintf("Join Node - %d\n%s", fgj.ID(), fgj.flowGraphNode.DOTID())
}

// /////////////////////////////////////////////////////////////////////////////
// flowGraphStartNode
// /////////////////////////////////////////////////////////////////////////////
type flowGraphStartNode struct {
	runCount uint64
	flowGraphNode
}

func (fgsn *flowGraphStartNode) D2Params(_ *slog.Logger) (*D2Encoding, error) {
	return nil, nil
}

func (fgsn *flowGraphStartNode) encodeD2Markdown(output io.StringWriter, log *slog.Logger) error {
	currentTime := nowTime.Format(time.ANSIC)
	return fgsn.encodeD2MarkdownNode(fgsn.name,
		map[string]interface{}{
			"Runs":    fgsn.runCount,
			"Created": currentTime,
		}, output,
		log)
}

func (fgsn *flowGraphStartNode) Generate(_ *flowGraph,
	_ []float64,
	src rand.Source,
	log *slog.Logger) (*generator.GenerationResults, error) {
	if fgsn.runCount <= 0 {
		return nil, fmt.Errorf("invalid run count for flowGraphStartNode %d", fgsn.runCount)
	}
	defSequence := make([]float64, fgsn.runCount)
	generationResults := &generator.GenerationResults{
		RawValues:        &defSequence,
		CumulativeValues: &defSequence,
	}
	return generationResults, nil
}

func (fgsn *flowGraphStartNode) DOTID() string {
	return fmt.Sprintf("StartNode - %d.\nRun count: %d", fgsn.ID(), fgsn.runCount)
}

// /////////////////////////////////////////////////////////////////////////////
// flowGraphPassThroughNode
//
// A virtual node that denotes entering a logical subgraph. The subgraph
// is terminated by the first reachable flowGraphJoinMaxValueNode node. The
// pass through just copies the existing values through...
// /////////////////////////////////////////////////////////////////////////////

type flowGraphPassThroughNode struct {
	sourceInputNode *flowGraphNode
	flowGraphNode
}

func (fgpt *flowGraphPassThroughNode) Generate(flowGraph *flowGraph,
	percentiles []float64,
	src rand.Source,
	log *slog.Logger) (*generator.GenerationResults, error) {

	priorSamples, priorSamplesErr := flowGraph.PredecessorValues(fgpt.ID())
	if priorSamplesErr != nil {
		return nil, priorSamplesErr
	}
	// Verify there's only a single series in the dictionary
	if len(priorSamples) != 1 {
		return nil, fmt.Errorf("invalid priorSample dimension for passthrough: %d", len(priorSamples))
	}
	sampleKeys := make([]int64, len(priorSamples))
	i := 0
	for eachKey := range priorSamples {
		sampleKeys[i] = eachKey
		i++
	}
	return priorSamples[sampleKeys[0]], nil
}

func (fgpt *flowGraphPassThroughNode) DOTID() string {
	return fmt.Sprintf("Passthrough - %d\n%s", fgpt.ID(), fgpt.flowGraphNode.DOTID())
}

// /////////////////////////////////////////////////////////////////////////////
// flowSubgraph
//
// Base graph type for logically related nodes. Subgraphs can be recusively
// nested. The top level graph is also a subgraph.
//
// /////////////////////////////////////////////////////////////////////////////
type flowSubgraph struct {
	// This subgraph's depth. The root subgraph has depth zero.
	parentFlowSubgraphs []*flowSubgraph
	inputNode           *flowGraphPassThroughNode
	outputJoinNode      *flowGraphJoinMaxValueNode
	serialGenerators    []DurationGeneratorGraphNode
	aggregationOptions  *AggregationOptions
	*simple.WeightedDirectedGraph
}

func (fsg *flowSubgraph) Weight(xid, yid int64) (w float64, ok bool) {
	connectionCost := float64(0)
	fromNode := fsg.Node(xid)
	if fromNode != nil {
		flowGraphSrcNode, flowGraphSrcNodeOk := fromNode.(*flowGraphNode)
		if flowGraphSrcNodeOk && nil != flowGraphSrcNode {
			connectionCost = flowGraphSrcNode.generator.GenerationResults().GeneratorStats.Mean
		}
	}
	return -1 * connectionCost, fsg.HasEdgeBetween(xid, yid)
}

// AddSerialGeneratorNode adds a serial step to the current node
func (fsg *flowSubgraph) AddSerialGeneratorNode(gen *flowGraphNode) error {
	// When we add a new node, ensure it's joined to the start
	// and there is a join to the exit
	gen.parentFlowSubgraphs = append(fsg.parentFlowSubgraphs, fsg)
	gen.aggregationOptions = fsg.aggregationOptions
	var lastSerialGenerator DurationGeneratorGraphNode
	if len(fsg.serialGenerators) > 0 {
		// Remove the existing link from the serial node to the
		// Join upper bound node
		lastSerialGenerator = fsg.serialGenerators[len(fsg.serialGenerators)-1]
		fsg.WeightedDirectedGraph.RemoveEdge(lastSerialGenerator.ID(), fsg.outputJoinNode.ID())
	}
	// Add it to the graph...
	fsg.serialGenerators = append(fsg.serialGenerators, gen)
	fsg.WeightedDirectedGraph.AddNode(gen)

	// If it's the first, then add the head link
	if len(fsg.serialGenerators) == 1 {
		edge := fsg.WeightedDirectedGraph.NewWeightedEdge(fsg.inputNode, gen, 0)
		fsg.WeightedDirectedGraph.SetWeightedEdge(edge)
	} else if lastSerialGenerator != nil {
		// Add the link from the previous
		edge := fsg.WeightedDirectedGraph.NewWeightedEdge(lastSerialGenerator, gen, 0)
		fsg.WeightedDirectedGraph.SetWeightedEdge(edge)
	}
	// There's always a link from this to the join node...
	edge := fsg.WeightedDirectedGraph.NewWeightedEdge(gen, fsg.outputJoinNode, 0)
	fsg.WeightedDirectedGraph.SetWeightedEdge(edge)

	return nil
}

func (fsg *flowSubgraph) AddParallelGeneratorNode(gen *flowGraphNode) graph.Node {
	// Create the node, link it to the input and join nodes...
	gen.parentFlowSubgraphs = append(fsg.parentFlowSubgraphs, fsg)
	gen.aggregationOptions = fsg.aggregationOptions

	fsg.WeightedDirectedGraph.AddNode(gen)
	edge := fsg.WeightedDirectedGraph.NewWeightedEdge(fsg.inputNode, gen, 1)
	fsg.WeightedDirectedGraph.SetWeightedEdge(edge)

	// Link it to the Join node...
	edge = fsg.WeightedDirectedGraph.NewWeightedEdge(gen, fsg.outputJoinNode, 1)
	fsg.WeightedDirectedGraph.SetWeightedEdge(edge)
	return nil
}

func (fsg *flowSubgraph) AddSubgraph(name string) (*flowSubgraph, error) {
	subgraph := newFlowSubgraph(name, fsg)

	// Attach the subgraph to our input, and the subgraphs join output to our output
	edge := fsg.WeightedDirectedGraph.NewWeightedEdge(fsg.inputNode, subgraph.inputNode, 1)
	fsg.WeightedDirectedGraph.SetWeightedEdge(edge)

	edge = fsg.WeightedDirectedGraph.NewWeightedEdge(subgraph.outputJoinNode, fsg.outputJoinNode, 1)
	fsg.WeightedDirectedGraph.SetWeightedEdge(edge)
	return subgraph, nil
}

// /////////////////////////////////////////////////////////////////////////////
// newFlowSubgraph
//
// Constructor function for a new flowSubgraph instance. The subgraph is a logical
// entity. All nodes are created on the targetGraph
//
// /////////////////////////////////////////////////////////////////////////////

func newFlowSubgraph(name string, parentSubgraph *flowSubgraph) *flowSubgraph {

	var dirGraph *simple.WeightedDirectedGraph
	if parentSubgraph == nil {
		dirGraph = simple.NewWeightedDirectedGraph(1, 0)
	} else {
		dirGraph = parentSubgraph.WeightedDirectedGraph
	}
	// How will the subgraph know it's depth?
	subgraph := &flowSubgraph{
		parentFlowSubgraphs:   make([]*flowSubgraph, 0),
		serialGenerators:      make([]DurationGeneratorGraphNode, 0),
		WeightedDirectedGraph: dirGraph,
	}
	if parentSubgraph != nil {
		subgraph.parentFlowSubgraphs = parentSubgraph.parentFlowSubgraphs
		subgraph.parentFlowSubgraphs = append(subgraph.parentFlowSubgraphs, parentSubgraph)
		subgraph.aggregationOptions = parentSubgraph.aggregationOptions
	}
	// Create the input node that denotes the pass through entrypoint
	// of this subgraph
	subgraph.inputNode = &flowGraphPassThroughNode{
		flowGraphNode: flowGraphNode{
			name:                name,
			id:                  rand.Int63(),
			parentFlowSubgraphs: append(subgraph.parentFlowSubgraphs, subgraph),
		},
	}
	if parentSubgraph != nil {
		subgraph.inputNode.sourceInputNode = &parentSubgraph.outputJoinNode.flowGraphNode
		subgraph.inputNode.aggregationOptions = parentSubgraph.aggregationOptions
	}
	subgraph.WeightedDirectedGraph.AddNode(subgraph.inputNode)
	subgraph.outputJoinNode = &flowGraphJoinMaxValueNode{
		flowGraphNode: flowGraphNode{
			name:                "Summary",
			id:                  rand.Int63(),
			parentFlowSubgraphs: append(subgraph.parentFlowSubgraphs, subgraph),
			generator:           &generator.UpperBoundGenerator{},
		},
	}
	if parentSubgraph != nil {
		subgraph.outputJoinNode.aggregationOptions = parentSubgraph.aggregationOptions
	}
	subgraph.WeightedDirectedGraph.AddNode(subgraph.outputJoinNode)
	return subgraph
}

// /////////////////////////////////////////////////////////////////////////////
// flowGraph
//
// The root level graph.
//
// /////////////////////////////////////////////////////////////////////////////
type flowGraph struct {
	name              string
	percentiles       []float64
	startNode         *flowGraphStartNode
	criticalPathGraph *simple.DirectedGraph
	generatorResults  map[int64]*generator.GenerationResults
	*flowSubgraph
}

func (fg *flowGraph) PredecessorValues(nodeID int64) (map[int64]*generator.GenerationResults, error) {
	ancestorNodesIterator := fg.WeightedDirectedGraph.To(nodeID)
	predecessorMap := make(map[int64]*generator.GenerationResults)
	for ancestorNodesIterator.Next() {
		values, valuesExist := fg.generatorResults[ancestorNodesIterator.Node().ID()]
		if !valuesExist {
			return nil, fmt.Errorf("no predecessor values for nodeId: %d", ancestorNodesIterator.Node().ID())
		}
		predecessorMap[ancestorNodesIterator.Node().ID()] = values
	}
	return predecessorMap, nil
}

func (fg *flowGraph) PlotDistribution(histogramPath string, log *slog.Logger) error {
	// Make a plot and set its title.
	p := plot.New()
	p.X.Label.Text = "Total Duration"
	p.Y.Label.Text = "Probability"
	p.Title.Text = "Cumulative Estimate"
	p.Title.TextStyle.Font.Typeface = font.Typeface("Monoco")
	p.Title.TextStyle.Color = color.RGBA{B: 255, A: 255}

	// Calculate the CDF
	GenerationResults := fg.outputJoinNode.generator.GenerationResults()

	// First bin the data and graph that. We'll bin into 100 bins
	rawHist, rawHistError := plotter.NewHist(plotter.Values(*GenerationResults.CumulativeValues), 100)
	if rawHistError != nil {
		return rawHistError
	}
	rawHist.Normalize(1)
	p.Add(rawHist)

	// Then plot the CDF
	sortedSamples := make([]float64, len(*GenerationResults.CumulativeValues))
	copy(sortedSamples, *GenerationResults.CumulativeValues)
	sort.Float64s(sortedSamples)

	// Bin the data into 100 bins...
	hist, histErr := plotter.NewHist(plotter.Values(sortedSamples), 100)
	if histErr != nil {
		return histErr
	}
	cdfValues := make(plotter.XYs, len(hist.Bins))
	cumulativeWeight := float64(0)
	for i := 0; i != len(hist.Bins); i++ {
		activeBin := hist.Bins[i]
		cumulativeWeight += activeBin.Weight
		cdfValues[i].X = activeBin.Max
		cdfValues[i].Y = cumulativeWeight / float64(len(sortedSamples))
	}

	line, _ := plotter.NewLine(cdfValues)
	line.LineStyle.Width = vg.Points(2)
	line.LineStyle.Dashes = []vg.Length{vg.Points(3), vg.Points(3)}
	line.LineStyle.Color = color.RGBA{R: 255, G: 144, A: 255}
	p.Add(line)

	// Save the plot to a PNG file.
	saveErr := p.Save(12*vg.Inch, 12*vg.Inch, histogramPath)
	return saveErr
}

func (fg *flowGraph) recursiveUnmarshal(rootObj map[string]interface{},
	subgraphParent *flowSubgraph,
	log *slog.Logger) error {

	generatorUnmarshaller := func(rawData interface{}, defaultName string) (*flowGraphNode, error) {
		mapData, mapDataOk := rawData.(map[string]interface{})
		if !mapDataOk {
			return nil, fmt.Errorf("failed to type assert generator unmarshaller")
		}
		durGenerator, durGeneratorErr := generator.NewDurationGenerator(mapData, log)
		if durGeneratorErr != nil {
			return nil, durGeneratorErr
		}
		nodeName := goejson.String("name", mapData)
		if len(nodeName) <= 0 {
			nodeName = defaultName
		}
		return &flowGraphNode{
			id:        rand.Int63(),
			name:      nodeName,
			generator: durGenerator,
		}, nil
	}
	rootMap, rootMapOk := rootObj["activities"].(map[string]interface{})

	if !rootMapOk {
		return fmt.Errorf("failed to extract %s from map", "activities")
	}

	// Get all the keys
	for eachKey := range rootMap {
		log.Debug("Unmarshalling definition", "key", eachKey)

		val := rootMap[eachKey]
		switch typedVal := val.(type) {
		case []interface{}:
			for i := 0; i != len(typedVal); i++ {
				node, nodeErr := generatorUnmarshaller(typedVal[i], fmt.Sprintf("serial-%d", i))
				if nodeErr != nil {
					return nodeErr
				}
				addErr := subgraphParent.AddSerialGeneratorNode(node)
				if addErr != nil {
					return addErr
				}
			}
		case map[string]interface{}:
			// Is this an activities blob, or a set of parallel tasks?
			title := goejson.String("name", typedVal)
			_, activitiesOk := typedVal["activities"]
			if len(title) != 0 && activitiesOk {
				// This is a subgraph...Add a subgraph to the parent and then keep going...
				subgraph, subgraphAddErr := subgraphParent.AddSubgraph(title)
				if subgraphAddErr != nil {
					return subgraphAddErr
				}
				subgraphErr := fg.recursiveUnmarshal(typedVal, subgraph, log)
				if subgraphErr != nil {
					return subgraphErr
				}
			} else {
				// For each key, get the parallel node
				for eachKey, eachDict := range typedVal {
					node, nodeErr := generatorUnmarshaller(eachDict, eachKey)
					if nodeErr != nil {
						return nodeErr
					}
					subgraphParent.AddParallelGeneratorNode(node)

				}
			}
		default:
			return fmt.Errorf("unsupported type: %T", val)
		}
	}
	return nil
}

func (fg *flowGraph) Unmarshal(inputStream io.Reader, log *slog.Logger) error {
	// Read the root object, unmarshal the props...then hand off the "activities"
	// object to the recursive unmarshal with ourselves as the parent....
	inputBytes, inputBytesErr := io.ReadAll(inputStream)
	if inputBytesErr != nil {
		return inputBytesErr
	}
	rootMap := make(map[string]interface{})
	unmarshalErr := json.Unmarshal(inputBytes, &rootMap)
	if unmarshalErr != nil {
		return unmarshalErr
	}
	fg.name = goejson.String("name", rootMap)
	fg.startNode.runCount = goejson.Uint("runCount", rootMap)
	// Percentiles?
	fg.percentiles = []float64{50, 95}
	userPercentiles, userPercentilesExists := rootMap["percentiles"]
	if userPercentilesExists {
		switch typedVal := userPercentiles.(type) {
		case []interface{}:
			floatVals := make([]float64, 0)
			for i := 0; i != len(typedVal); i++ {
				castFloat, castFloatOK := typedVal[i].(float64)
				if !castFloatOK {
					return fmt.Errorf("invalid percentile specified: %v. Only arrays of float64 are supported", typedVal[i])
				}
				floatVals = append(floatVals, castFloat)
			}
			fg.percentiles = floatVals
		default:
			return fmt.Errorf("invalid percentiles specified: %v. Only arrays of float64 are supported", typedVal)
		}
	}
	// Set up the aggregation options
	fg.flowSubgraph.aggregationOptions = &AggregationOptions{}
	fg.flowSubgraph.aggregationOptions.workdays = goejson.Boolean("workdays", rootMap)
	return fg.recursiveUnmarshal(rootMap, fg.flowSubgraph, log)
}

func (fg *flowGraph) Evaluate(histogramPath string, log *slog.Logger) error {
	sortedNodes, sortedNodesErr := topo.Sort(fg)
	if sortedNodesErr != nil {
		return sortedNodesErr
	}

	// Topo sort, then evaluate all the nodes.
	percentiles := fg.percentiles
	randSrc := rand.NewSource(0)
	for _, val := range sortedNodes {
		switch typedVal := val.(type) {
		case DurationGeneratorGraphNode:
			values, valuesErr := typedVal.Generate(fg, percentiles, randSrc, log)
			if valuesErr != nil {
				return valuesErr
			}
			fg.generatorResults[val.ID()] = values
		default:
			return fmt.Errorf("invalid node type: %T", typedVal)
		}
	}
	// What's the critical path?
	srcPt, ok := path.BellmanFordFrom(fg.startNode, fg)
	if !ok {
		fmt.Println("no negative cycle present")
	} else {
		allNodes := fg.flowSubgraph.WeightedDirectedGraph.Nodes()
		for allNodes.Next() {
			curTerminalNode := allNodes.Node()
			shortestPath, shortestWeight := srcPt.To(curTerminalNode.ID())
			if math.IsInf(shortestWeight, -1) {
				return fmt.Errorf("negative cycle in path to %c path:%c", allNodes.Node().ID(), shortestPath)
			} else {
				nodePath := make([]int64, len(shortestPath))
				for i := 0; i != len(shortestPath); i++ {
					nodePath[i] = shortestPath[i].ID()
				}
				if curTerminalNode.ID() == fg.outputJoinNode.ID() {
					// Shortest nodePath
					log.Debug("shortest path to OUTPUT node",
						"source", fg.startNode.ID(),
						"end", allNodes.Node().ID(),
						"weight", shortestWeight,
						"path", fmt.Sprintf("%v", nodePath),
					)
					for i := 1; i != len(nodePath); i++ {
						srcNode := &flowGraphNode{
							id: nodePath[i-1],
						}
						destNode := &flowGraphNode{
							id: nodePath[i],
						}
						// Update the separate critical path graph with the edges
						// that are on the CPS.
						criticalEdge := fg.criticalPathGraph.NewEdge(srcNode, destNode)
						fg.criticalPathGraph.SetEdge(criticalEdge)
					}
				} else {
					// Shortest nodePath
					log.Debug("shortest path",
						"source", fg.startNode.ID(),
						"end", allNodes.Node().ID(),
						"weight", shortestWeight,
						"path", fmt.Sprintf("%v", nodePath),
					)
				}
			}
		}
	}
	// Graph the final distribution
	return fg.PlotDistribution(histogramPath, log)
}

func newFlowGraph(inputFile io.Reader, log *slog.Logger) (*flowGraph, error) {
	// Create the beginning and end nodes...
	fg := &flowGraph{
		flowSubgraph:      newFlowSubgraph("input", nil),
		criticalPathGraph: simple.NewDirectedGraph(),
		generatorResults:  make(map[int64]*generator.GenerationResults),
	}
	fg.startNode = &flowGraphStartNode{
		runCount: 0,
		flowGraphNode: flowGraphNode{
			name: "START",
			id:   rand.Int63(),
		},
	}
	fg.AddNode(fg.startNode)
	edge := fg.WeightedDirectedGraph.NewWeightedEdge(fg.startNode, fg.flowSubgraph.inputNode, 0)
	fg.WeightedDirectedGraph.SetWeightedEdge(edge)

	// Then try to unmarshal from the input stream
	unmarshalErr := fg.Unmarshal(inputFile, log)
	if unmarshalErr != nil {
		return nil, unmarshalErr
	}
	fg.startNode.flowGraphNode.name = fg.name
	return fg, nil
}

type ApplicationFlowGraphParams struct {
	InputFile       string
	OutputDirectory string
	CreateDot       bool
	LightThemeID    int64
	DarkThemeID     int64
}

func NewApplicationFlowGraph(params *ApplicationFlowGraphParams, log *slog.Logger) (*graph.Directed, error) {

	inputFile, _ := os.Open(params.InputFile)
	appGraph, appGraphErr := newFlowGraph(inputFile, log)
	if appGraphErr != nil {
		return nil, appGraphErr
	}
	outputFileName := filepath.Base(params.InputFile)
	outputFileBaseName := strings.TrimSuffix(outputFileName, filepath.Ext(outputFileName))

	if params.CreateDot {
		dotOutPath := filepath.Join(params.OutputDirectory, outputFileBaseName+".dot")
		dotBytes, dotBytesErr := dot.Marshal(appGraph, "Test", "", " ")
		if dotBytesErr != nil {
			return nil, dotBytesErr
		}
		writeErr := os.WriteFile(dotOutPath, dotBytes, 0644)
		if writeErr != nil {
			return nil, writeErr
		}
		log.Info("Created dot output file", "path", dotOutPath)
	}

	// Evaluate the graph and output the results...
	histogramPath := filepath.Join(params.OutputDirectory, outputFileBaseName+".png")
	evalErr := appGraph.Evaluate(histogramPath, log)
	if evalErr != nil {
		return nil, evalErr
	}
	d2File := filepath.Join(params.OutputDirectory, outputFileBaseName+".d2")
	f, _ := os.Create(d2File)

	encoder := D2EncodingVisitor{
		criticalPathGraph: simple.NewDirectedGraph(),
	}
	encodeErr := encoder.Encode(appGraph, histogramPath, f, log)
	closeErr := f.Close()
	if closeErr != nil {
		return nil, closeErr
	}
	if encodeErr != nil {
		log.Error("Failed to encode node", "err", encodeErr)
	}
	createErr := createD2Image(d2File,
		filepath.Join(params.OutputDirectory, outputFileBaseName+".svg"),
		params.LightThemeID,
		params.DarkThemeID,
		log)

	return nil, createErr
}

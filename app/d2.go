package app

import (
	"cmp"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"

	"gonum.org/v1/gonum/graph"
)

// /////////////////////////////////////////////////////////////////////////////
//
// TYPES
//
// /////////////////////////////////////////////////////////////////////////////

type D2Encoder interface {
	ID() int64
	D2Encode(output io.StringWriter, indent string, log *slog.Logger) error
	AbsoluteNodePath() []int64
}

// /////////////////////////////////////////////////////////////////////////////
// D2Connection
//
// D2 connections that are fully scoped (a.b.node -> a.node2) which will be
// written at the end of serializing all the nested graphs
//
// /////////////////////////////////////////////////////////////////////////////

type D2Connection struct {
	from         string
	to           string
	cost         float64
	criticalPath bool
}

// /////////////////////////////////////////////////////////////////////////////
// D2EncodingVisitor
//
// The stateful visitor that properly serializes nested graphs and caches
// D2Connections during traversal
//
// /////////////////////////////////////////////////////////////////////////////
type D2EncodingVisitor struct {
	visited           map[int64]int
	criticalPathGraph graph.Directed
	owningGraph       *flowGraph
	log               *slog.Logger
	connectionsList   []*D2Connection
}

func (d2enc *D2EncodingVisitor) encodeNode(output io.StringWriter, node D2Encoder) error {
	_, valExists := d2enc.visited[node.ID()]
	if valExists {
		return nil
	}
	d2enc.log.Debug("Encoding node", "type", fmt.Sprintf("%T", node), "id", node.ID())
	d2enc.visited[node.ID()] = 1
	indent := strings.Repeat("\t", len(node.AbsoluteNodePath())-1)
	return node.D2Encode(output, indent, d2enc.log)
}

func (d2enc *D2EncodingVisitor) fullConnectionPathForNode(node D2Encoder) string {
	rootGraphIDPrefix := fmt.Sprintf("%d.", d2enc.owningGraph.inputNode.id)
	fullIDPath := node.AbsoluteNodePath()
	idPath := ""
	for i := 0; i != len(fullIDPath); i++ {
		idPath += fmt.Sprintf("%d.", fullIDPath[i])
	}
	idPath = strings.TrimSuffix(idPath, ".")
	return strings.TrimPrefix(idPath, rootGraphIDPrefix)
}

func (d2enc *D2EncodingVisitor) createConnection(fromNode D2Encoder, toNode D2Encoder) {
	// No self-connections
	if fromNode.ID() == toNode.ID() {
		return
	}
	connectionCost := float64(0)
	flowGraphSrcNode, flowGraphSrcNodeOk := fromNode.(*flowGraphNode)
	if flowGraphSrcNodeOk && nil != flowGraphSrcNode {
		connectionCost = float64(flowGraphSrcNode.generator.GenerationResults().GeneratorStats.Mean)
	}
	criticalPathEdge := d2enc.criticalPathGraph.HasEdgeBetween(fromNode.ID(), toNode.ID())
	fromPath := d2enc.fullConnectionPathForNode(fromNode)
	toPath := d2enc.fullConnectionPathForNode(toNode)

	d2enc.log.Debug("Creating connection", "from", fromPath, "to", toPath)
	d2enc.connectionsList = append(d2enc.connectionsList, &D2Connection{
		from:         fromPath,
		to:           toPath,
		cost:         connectionCost,
		criticalPath: criticalPathEdge,
	})
}

func (d2enc *D2EncodingVisitor) encodeDirectChildren(output io.StringWriter,
	fromNode D2Encoder,
	children []D2Encoder,
	outputJoinNode D2Encoder) error {

	// Regular children...
	for i := 0; i != len(children); i++ {
		successorNode := children[i]
		encError := d2enc.encodeNode(output, successorNode)
		if encError != nil {
			return encError
		}
		d2enc.createConnection(fromNode, successorNode)

		flowGraphJoinNode, flowGraphJoinNodeOk := successorNode.(*flowGraphJoinMaxValueNode)
		if flowGraphJoinNodeOk {
			// We need to write out the full path to the connections slice
			// so that the critical path styling will escape the subgraph.
			d2enc.createConnection(flowGraphJoinNode, outputJoinNode)

		} else {
			recursiveErr := d2enc.recursiveEncode(output, successorNode, outputJoinNode)
			if recursiveErr != nil {
				return recursiveErr
			}
		}
	}
	return nil
}

func (d2enc *D2EncodingVisitor) recursiveEncode(output io.StringWriter,
	fromNode D2Encoder,
	outputJoinNode D2Encoder) error {

	subgraphDepth := func() int {
		return (len(fromNode.AbsoluteNodePath()))
	}
	autoindent := func() string {
		return strings.Repeat("\t", subgraphDepth())
	}

	// Write out the Join node before descending...
	if outputJoinNode != nil {
		encodeErr := d2enc.encodeNode(output, outputJoinNode)
		if encodeErr != nil {
			return nil
		}
	}

	// Get successors
	// Split them into subgraphs that need stack management and plain children
	subgraphNodes := make([]*flowGraphPassThroughNode, 0)
	atomicSuccessorNodes := make([]D2Encoder, 0)

	successorIter := d2enc.owningGraph.From(fromNode.ID())
	for successorIter.Next() {
		switch typedNode := successorIter.Node().(type) {
		case *flowGraphPassThroughNode: // Denotes entering a subgraph...
			subgraphNodes = append(subgraphNodes, typedNode)
		case D2Encoder:
			atomicSuccessorNodes = append(atomicSuccessorNodes, typedNode)
		}
	}

	// Sort them by name
	slices.SortFunc(subgraphNodes, func(a, b *flowGraphPassThroughNode) int {
		return cmp.Compare(strings.ToLower(a.name), strings.ToLower(b.name))
	})

	d2enc.log.Debug("recursiveEncode Node",
		"depth", subgraphDepth(),
		"ID", fromNode.ID(),
		"type", fmt.Sprintf("%T", fromNode))

	// Subgraphs are denoted by a passthrough node. They logically close with a
	// join node. That join node is connected to the incoming join
	for i := 0; i != len(subgraphNodes); i++ {
		subgraphNode := subgraphNodes[i]

		// What's the join node for this subgraph?
		parentSubgraph := subgraphNode.parentFlowSubgraphs[len(subgraphNode.parentFlowSubgraphs)-1]
		subgraphJoinNode := parentSubgraph.outputJoinNode
		d2enc.createConnection(fromNode, subgraphNode)
		d2enc.createConnection(subgraphJoinNode, outputJoinNode)

		if subgraphDepth() > 0 {
			output.WriteString(fmt.Sprintf(`%s%d: %s {
				style: {
					border-radius: 20
				}
	`,
				autoindent(),
				subgraphNode.ID(),
				subgraphNode.name))

		} else {
			// Just write out the node at existing depth
			encodeErr := d2enc.encodeNode(output, subgraphNode)
			if encodeErr != nil {
				return nil
			}
		}

		// Recurse into this subgraph...
		recursiveErr := d2enc.recursiveEncode(output,
			subgraphNode,
			subgraphJoinNode)
		if recursiveErr != nil {
			return recursiveErr
		}
		// Close out the subgraph if we're not writing the virtual top level subgraph
		if subgraphDepth() > 0 {
			output.WriteString(fmt.Sprintf("%s}\n", autoindent()))
		}
	}
	encError := d2enc.encodeDirectChildren(output, fromNode, atomicSuccessorNodes, outputJoinNode)
	if encError != nil {
		return encError
	}
	return nil
}
func (d2enc *D2EncodingVisitor) Encode(graph *flowGraph, histogramPath string, output io.StringWriter, log *slog.Logger) error {
	d2enc.owningGraph = graph
	d2enc.criticalPathGraph = graph.criticalPathGraph
	d2enc.log = log
	d2enc.visited = make(map[int64]int, 0)
	d2enc.connectionsList = make([]*D2Connection, 0)

	// Output the primary join node...
	output.WriteString(`
# Nodes
# ------------------------------------------------------------------------------

`)
	// Start at the start node, which is the virtual root that has the
	// run parameters
	encodeErr := graph.startNode.encodeD2Markdown(output, log)
	if encodeErr != nil {
		return nil
	}

	// Then write out the histogram node and add a link to the output join node
	histogramNodeName := "histogram_summary"
	output.WriteString(fmt.Sprintf(`%s: Estimated Completion {
shape: image
icon: %s
width: 768
height: 768
}
`,
		histogramNodeName,
		histogramPath))

	d2enc.connectionsList = append(d2enc.connectionsList, &D2Connection{
		from:         fmt.Sprintf("%d", graph.outputJoinNode.ID()),
		to:           histogramNodeName,
		cost:         0,
		criticalPath: false,
	})

	// At this point we have the flowInputNode which is the top level subgraph
	successorNodesIter := graph.WeightedDirectedGraph.From(graph.startNode.ID())
	for successorNodesIter.Next() {
		targetNode := successorNodesIter.Node()
		d2enc.createConnection(graph.startNode, targetNode.(D2Encoder))

		// Encode this node
		encodeErr := d2enc.encodeNode(output, targetNode.(D2Encoder))
		if encodeErr != nil {
			return nil
		}
		encodeErr = d2enc.recursiveEncode(output, targetNode.(D2Encoder), graph.outputJoinNode)
		if encodeErr != nil {
			return nil
		}
	}
	output.WriteString(`

# Connections
# ------------------------------------------------------------------------------
`)
	for i := 0; i != len(d2enc.connectionsList); i++ {
		connection := d2enc.connectionsList[i]
		costSuffix := ""
		if connection.cost != 0 {
			costSuffix = fmt.Sprintf(" : %.2f", connection.cost)
		}
		styleSuffix := ""
		if connection.criticalPath {
			styleSuffix = ` {
	style: {
		stroke: crimson
		stroke-width: 4
		stroke-dash: 2
		}
	}`
		}
		output.WriteString(fmt.Sprintf("%s -> %s%s%s\n", connection.from, connection.to, costSuffix, styleSuffix))
	}
	return nil
}

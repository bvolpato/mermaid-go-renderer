// Ported from cytoscape-fcose 2.2.0 (MIT license): src/fcose/spectral.js and
// the functions connectComponents, getTopMostNodes and calcBoundingBox of
// src/fcose/auxiliary.js. The spectral layout is the first part of fcose
// with the option randomize.

package fcose

import (
	"math"
	"strconv"

	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/internal/bail"
)

// spectralResult is the result of the spectral layout: the index of each
// node without children, and of each dummy node, in the two coordinate
// lists.
type spectralResult struct {
	nodeIndexes map[string]int
	xCoords     []float64
	yCoords     []float64
}

// maxPowerIterations bounds the loops of the power iteration. The JavaScript
// has no bound: it does not end when the iteration does not converge.
const maxPowerIterations = 200000

// descendants is node.descendants(): the nodes inside the node, each one
// before the nodes inside it.
func descendants(node *NodeInput) []*NodeInput {
	var elements []*NodeInput
	var add func(eles []*NodeInput)
	add = func(eles []*NodeInput) {
		for _, ele := range eles {
			elements = append(elements, ele)
			if len(ele.Children) > 0 {
				add(ele.Children)
			}
		}
	}
	add(node.Children)
	return elements
}

func hasNode(list []*NodeInput, node *NodeInput) bool {
	for _, n := range list {
		if n == node {
			return true
		}
	}
	return false
}

// getTopMostNodes is aux.getTopMostNodes: the nodes of the list that are not
// inside another node of the list.
func getTopMostNodes(nodes []*NodeInput) []*NodeInput {
	nodesMap := make(map[*NodeInput]bool, len(nodes))
	for _, n := range nodes {
		nodesMap[n] = true
	}
	var roots []*NodeInput
	for _, ele := range nodes {
		top := true
		for parent := ele.Parent; parent != nil; parent = parent.Parent {
			if nodesMap[parent] {
				top = false
				break
			}
		}
		if top {
			roots = append(roots, ele)
		}
	}
	return roots
}

// dummyNodes is the Map of the JavaScript that keeps the dummy nodes and the
// nodes that each one connects, in the order of insertion.
type dummyNodes struct {
	keys  []string
	lists map[string][]string
}

// connectComponents is aux.connectComponents: it finds the components of the
// graph that are not connected, and makes a dummy node that connects them.
func connectComponents(topMostNodes []*NodeInput, dummy *dummyNodes) {
	visited := map[*NodeInput]bool{}
	var visitedTopMostNodes []*NodeInput

	isConnected := false
	count := 1
	var nodesConnectedToDummy []string

	if len(topMostNodes) == 0 {
		// The JavaScript reads a member of undefined, and throws.
		bail.Out("fcose: the components of a graph without nodes")
	}
	for {
		currentNode := topMostNodes[0]
		childrenOfCurrentNode := append([]*NodeInput{currentNode}, descendants(currentNode)...)
		visitedTopMostNodes = append(visitedTopMostNodes, currentNode)

		var queue []*NodeInput
		for _, node := range childrenOfCurrentNode {
			queue = append(queue, node)
			visited[node] = true
		}

		for head := 0; head < len(queue); head++ {
			currentNode = queue[head]

			// Traverse all neighbors of this node
			for _, neighborNode := range currentNode.Neighbors {
				// currentNeighbor is the node of topMostNodes that is the
				// neighbor or contains it.
				var currentNeighbor *NodeInput
				for n := neighborNode; n != nil; n = n.Parent {
					if hasNode(topMostNodes, n) {
						currentNeighbor = n
						break
					}
				}
				// An empty collection has no first element, and the Set of
				// the JavaScript does not have undefined: the body then runs
				// for no node.
				if currentNeighbor != nil && !visited[currentNeighbor] {
					childrenOfNeighbor := append([]*NodeInput{currentNeighbor}, descendants(currentNeighbor)...)

					for _, node := range childrenOfNeighbor {
						queue = append(queue, node)
						visited[node] = true
						if hasNode(topMostNodes, node) {
							visitedTopMostNodes = append(visitedTopMostNodes, node)
						}
					}
				}
			}
		}

		if len(visitedTopMostNodes) == len(topMostNodes) {
			isConnected = true
		}

		if !isConnected || isConnected && count > 1 {
			minDegreeNode := visitedTopMostNodes[0]
			minDegree := minDegreeNode.Degree
			for _, node := range visitedTopMostNodes {
				if node.Degree < minDegree {
					minDegree = node.Degree
					minDegreeNode = node
				}
			}
			nodesConnectedToDummy = append(nodesConnectedToDummy, minDegreeNode.ID)
			// topMostNodes = topMostNodes.difference(temp)
			var rest []*NodeInput
			for _, node := range topMostNodes {
				if !hasNode(visitedTopMostNodes, node) {
					rest = append(rest, node)
				}
			}
			visitedTopMostNodes = nil
			topMostNodes = rest
			count++
		}
		if isConnected {
			break
		}
		if len(topMostNodes) == 0 {
			// The JavaScript throws here.
			bail.Out("fcose: the components of a graph without nodes")
		}
	}

	if len(nodesConnectedToDummy) > 0 {
		key := "dummy" + strconv.Itoa(len(dummy.keys)+1)
		if _, dup := dummy.lists[key]; !dup {
			dummy.keys = append(dummy.keys, key)
		}
		dummy.lists[key] = nodesConnectedToDummy
	}
}

// spectralLayout is spectralLayout: the positions of the nodes from the two
// largest eigenvectors of a matrix of graph distances to sampled nodes.
func spectralLayout(nodes []*NodeInput, opts *Options) *spectralResult {
	var parentNodes []*NodeInput
	for _, n := range nodes {
		if len(n.Children) > 0 {
			parentNodes = append(parentNodes, n)
		}
	}
	byID := make(map[string]*NodeInput, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
	}

	dummy := &dummyNodes{lists: map[string][]string{}} // map to keep dummy nodes and their neighbors
	nodeIndexes := map[string]int{}                    // map to keep indexes to nodes
	var indexOrder []string                            // the keys of nodeIndexes in the order of insertion
	parentChildMap := map[string]string{}              // mapping btw. compound and its representative node
	var allNodesNeighborhood [][]string                // array to keep neighborhood of all nodes
	var xCoords, yCoords []float64

	var samplesColumn []int // sampled vertices
	var minDistancesColumn []float64
	var c [][]float64   // column sampling matrix
	var phi [][]float64 // intersection of column and row sampling matrices
	var inv [][]float64 // inverse of PHI

	var nodeSize int

	const infinity = 100000000.0
	small := 0.000000001

	piTol := 0.0000001
	nodeSeparation := opts.NodeSeparation
	var sampleSize int

	/**** Spectral layout functions ****/

	// takes the index of the node(pivot) to initiate BFS as a parameter
	bfs := func(pivot, index int) int {
		path := make([]int, 0, nodeSize)
		front := 0
		distance := make([]float64, nodeSize)

		maxDist := 0.0 // the furthest node to be returned
		maxInd := 1

		for i := 0; i < nodeSize; i++ {
			distance[i] = infinity
		}

		path = append(path, pivot)
		distance[pivot] = 0

		for len(path)-1 >= front {
			current := path[front]
			front++
			neighbors := allNodesNeighborhood[current]
			for i := 0; i < len(neighbors); i++ {
				temp, ok := nodeIndexes[neighbors[i]]
				if ok && distance[temp] == infinity {
					distance[temp] = distance[current] + 1
					path = append(path, temp)
				}
			}
			c[current][index] = distance[current] * nodeSeparation
		}

		for i := 0; i < nodeSize; i++ {
			if c[i][index] < minDistancesColumn[i] {
				minDistancesColumn[i] = c[i][index]
			}
		}

		for i := 0; i < nodeSize; i++ {
			if minDistancesColumn[i] > maxDist {
				maxDist = minDistancesColumn[i]
				maxInd = i
			}
		}
		return maxInd
	}

	// apply BFS to all nodes or selected samples: the sampling is greedy
	allBFS := func() {
		sample := int(math.Floor(opts.Random() * float64(nodeSize)))

		minDistancesColumn = make([]float64, nodeSize)
		for i := 0; i < nodeSize; i++ {
			minDistancesColumn[i] = infinity
		}

		samplesColumn = make([]int, sampleSize)
		for i := 0; i < sampleSize; i++ {
			samplesColumn[i] = sample
			sample = bfs(sample, i)
		}

		// form the squared distances for C
		for i := 0; i < nodeSize; i++ {
			for j := 0; j < sampleSize; j++ {
				c[i][j] *= c[i][j]
			}
		}

		// form PHI
		phi = make([][]float64, sampleSize)
		for i := 0; i < sampleSize; i++ {
			phi[i] = make([]float64, sampleSize)
		}

		for i := 0; i < sampleSize; i++ {
			for j := 0; j < sampleSize; j++ {
				phi[i][j] = c[samplesColumn[j]][i]
			}
		}
	}

	// perform the SVD algorithm and apply a regularization step
	sample := func() {
		svdResult := svd(phi)

		aQ := svdResult.S
		aU := svdResult.U
		aV := svdResult.V

		maxS := aQ[0] * aQ[0] * aQ[0]

		aSig := make([][]float64, sampleSize)

		//  regularization
		for i := 0; i < sampleSize; i++ {
			aSig[i] = make([]float64, sampleSize)
			for j := 0; j < sampleSize; j++ {
				aSig[i][j] = 0
				if i == j {
					aSig[i][j] = aQ[i] / (float64(aQ[i]*aQ[i]) + maxS/(aQ[i]*aQ[i]))
				}
			}
		}

		inv = multMat(multMat(aV, aSig), transpose(aU))
	}

	// calculate final coordinates
	powerIteration := func() {
		// two largest eigenvalues
		var theta1, theta2 float64

		// initial guesses for eigenvectors
		y1 := make([]float64, nodeSize)
		y2 := make([]float64, nodeSize)

		v1 := make([]float64, nodeSize)
		v2 := make([]float64, nodeSize)

		for i := 0; i < nodeSize; i++ {
			y1[i] = opts.Random()
			y2[i] = opts.Random()
		}

		y1 = normalize(y1)
		y2 = normalize(y2)

		count := 0
		// to keep track of the improvement ratio in power iteration
		current := small
		previous := small

		var temp float64
		one := 1.0

		for {
			count++
			if count > maxPowerIterations {
				bail.Out("fcose: the power iteration does not converge")
			}

			copy(v1, y1)

			y1 = multGamma(multL(multGamma(v1), c, inv))
			theta1 = dotProduct(v1, y1)
			y1 = normalize(y1)

			current = dotProduct(v1, y1)

			temp = math.Abs(current / previous)

			if temp <= one+piTol && temp >= 1 {
				break
			}

			previous = current
		}

		copy(v1, y1)

		count = 0
		previous = small
		for {
			count++
			if count > maxPowerIterations {
				bail.Out("fcose: the power iteration does not converge")
			}

			copy(v2, y2)

			v2 = minusOp(v2, multCons(v1, dotProduct(v1, v2)))
			y2 = multGamma(multL(multGamma(v2), c, inv))
			theta2 = dotProduct(v2, y2)
			y2 = normalize(y2)

			current = dotProduct(v2, y2)

			temp = math.Abs(current / previous)

			if temp <= one+piTol && temp >= 1 {
				break
			}

			previous = current
		}

		copy(v2, y2)

		// theta1 now contains dominant eigenvalue
		// theta2 now contains the second-largest eigenvalue
		// V1 now contains theta1's eigenvector
		// V2 now contains theta2's eigenvector

		//populate the two vectors
		xCoords = multCons(v1, math.Sqrt(math.Abs(theta1)))
		yCoords = multCons(v2, math.Sqrt(math.Abs(theta2)))
	}

	/**** Preparation for spectral layout (Preprocessing) ****/

	// connect disconnected components (first top level, then inside of each compound node)
	connectComponents(getTopMostNodes(nodes), dummy)

	for _, ele := range parentNodes {
		connectComponents(getTopMostNodes(descendants(ele)), dummy)
	}

	// assign indexes to nodes (first real, then dummy nodes)
	index := 0
	setIndex := func(key string) {
		if _, ok := nodeIndexes[key]; !ok {
			indexOrder = append(indexOrder, key)
		}
		nodeIndexes[key] = index
		index++
	}
	for _, n := range nodes {
		if len(n.Children) == 0 {
			setIndex(n.ID)
		}
	}

	for _, key := range dummy.keys {
		setIndex(key)
	}

	// instantiate the neighborhood matrix
	allNodesNeighborhood = make([][]string, len(nodeIndexes))

	// form a parent-child map to keep representative node of each compound node
	for _, ele := range parentNodes {
		children := ele.Children

		childless := func(list []*NodeInput) []*NodeInput {
			var out []*NodeInput
			for _, n := range list {
				if len(n.Children) == 0 {
					out = append(out, n)
				}
			}
			return out
		}
		for len(childless(children)) == 0 {
			// if all children are compound then proceed with the first one
			children = children[0].Children
		}
		//  select the representative node - we can apply different methods here
		leaves := childless(children)
		index := 0
		minDegree := leaves[0].Degree
		for i, ele2 := range leaves {
			if ele2.Degree < minDegree {
				minDegree = ele2.Degree
				index = i
			}
		}
		parentChildMap[ele.ID] = leaves[index].ID
	}

	// add neighborhood relations (first real, then dummy nodes)
	for _, ele := range nodes {
		var eleIndex int
		var ok bool

		if len(ele.Children) > 0 {
			eleIndex, ok = nodeIndexes[parentChildMap[ele.ID]]
		} else {
			eleIndex, ok = nodeIndexes[ele.ID]
		}

		for _, node := range ele.Neighbors {
			if !ok {
				bail.Out("fcose: a node without an index in the spectral layout")
			}
			if len(node.Children) > 0 {
				allNodesNeighborhood[eleIndex] = append(allNodesNeighborhood[eleIndex], parentChildMap[node.ID])
			} else {
				allNodesNeighborhood[eleIndex] = append(allNodesNeighborhood[eleIndex], node.ID)
			}
		}
	}

	for _, key := range dummy.keys {
		eleIndex := nodeIndexes[key]
		for _, id := range dummy.lists[key] {
			disconnectedID := id
			if n := byID[id]; n != nil && len(n.Children) > 0 {
				disconnectedID = parentChildMap[id]
			}

			allNodesNeighborhood[eleIndex] = append(allNodesNeighborhood[eleIndex], disconnectedID)
			other, ok := nodeIndexes[disconnectedID]
			if !ok {
				bail.Out("fcose: a node without an index in the spectral layout")
			}
			allNodesNeighborhood[other] = append(allNodesNeighborhood[other], key)
		}
	}

	// nodeSize now only considers the size of transformed graph
	nodeSize = len(nodeIndexes)
	if nodeSize != index {
		// A node with the id of a dummy node: the Map of the JavaScript then
		// has fewer keys than indexes.
		bail.Out("fcose: a node with the id of a dummy node")
	}

	// If number of nodes in transformed graph is 1 or 2, either SVD or powerIteration causes problem
	// So skip spectral and layout the graph with cose
	if nodeSize > 2 {
		// if # of nodes in transformed graph is smaller than sample size,
		// then use # of nodes as sample size
		const optionSampleSize = 25
		sampleSize = min(nodeSize, optionSampleSize)

		// instantiates the partial matrices that will be used in spectral
		// layout. A member that the JavaScript does not set is undefined.
		c = make([][]float64, nodeSize)
		for i := range c {
			c[i] = make([]float64, sampleSize)
			for j := range c[i] {
				c[i][j] = nan
			}
		}

		/**** Apply spectral layout ****/

		allBFS()
		sample()
		powerIteration()

		return &spectralResult{nodeIndexes: nodeIndexes, xCoords: xCoords, yCoords: yCoords}
	}
	firstNode := byID[indexOrder[0]]
	if firstNode == nil {
		bail.Out("fcose: a spectral layout of dummy nodes")
	}
	xCoords = append(xCoords, firstNode.X)
	yCoords = append(yCoords, firstNode.Y)
	if nodeSize == 2 {
		// The JavaScript adds options.idealEdgeLength here, which is a
		// function in mermaid: the sum is a text, and the coordinate that
		// the layout computes from it is NaN.
		xCoords = append(xCoords, nan)
		yCoords = append(yCoords, firstNode.Y)
	}

	return &spectralResult{nodeIndexes: nodeIndexes, xCoords: xCoords, yCoords: yCoords}
}

// spectralBoundingBox is aux.calcBoundingBox: the bounds of the nodes
// without children inside parentNode, at their spectral coordinates.
func spectralBoundingBox(parentNode *NodeInput, s *spectralResult) (topLeftX, topLeftY, width, height float64) {
	// calculate bounds
	left := float64(maxSafeInteger)
	right := float64(-maxSafeInteger)
	top := float64(maxSafeInteger)
	bottom := float64(-maxSafeInteger)

	for _, node := range descendants(parentNode) {
		if len(node.Children) > 0 {
			continue
		}
		i, ok := s.nodeIndexes[node.ID]
		x, y := nan, nan
		if ok && i < len(s.xCoords) {
			x, y = s.xCoords[i], s.yCoords[i]
		}

		nodeLeft := x - float64(node.Width/2)
		nodeRight := x + float64(node.Width/2)
		nodeTop := y - float64(node.Height/2)
		nodeBottom := y + float64(node.Height/2)

		if left > nodeLeft {
			left = nodeLeft
		}

		if right < nodeRight {
			right = nodeRight
		}

		if top > nodeTop {
			top = nodeTop
		}

		if bottom < nodeBottom {
			bottom = nodeBottom
		}
	}

	return left, top, right - left, bottom - top
}

// maxSafeInteger is Number.MAX_SAFE_INTEGER.
const maxSafeInteger = 9007199254740991

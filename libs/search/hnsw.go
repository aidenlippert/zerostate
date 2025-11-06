package search

import (
	"container/heap"
	"math"
	"math/rand"
	"sync"
)

// HNSWIndex implements Hierarchical Navigable Small World graph for approximate nearest neighbor search
type HNSWIndex struct {
	mu sync.RWMutex
	
	// Graph structure
	nodes      []*HNSWNode
	layers     []int           // Max layer for each node
	entryPoint int             // ID of entry point node
	
	// Parameters
	M              int     // Number of bi-directional links per node
	Mmax           int     // Max number of connections per layer
	Mmax0          int     // Max connections for layer 0
	efConstruction int     // Size of dynamic candidate list during construction
	ml             float64 // Normalization factor for level generation
	
	// Distance function
	distFunc func(Vector, Vector) float64
}

// HNSWNode represents a node in the HNSW graph
type HNSWNode struct {
	id      int
	vector  Vector
	payload interface{} // Store agent card or metadata
	
	// Connections at each layer (layer -> neighbor IDs)
	connections [][]int
}

// NewHNSWIndex creates a new HNSW index
func NewHNSWIndex(M, efConstruction int) *HNSWIndex {
	return &HNSWIndex{
		nodes:          make([]*HNSWNode, 0),
		layers:         make([]int, 0),
		entryPoint:     -1,
		M:              M,
		Mmax:           M,
		Mmax0:          M * 2,
		efConstruction: efConstruction,
		ml:             1.0 / math.Log(float64(M)),
		distFunc:       EuclideanDistance,
	}
}

// SetDistanceFunc sets a custom distance function (default is Euclidean)
func (h *HNSWIndex) SetDistanceFunc(f func(Vector, Vector) float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.distFunc = f
}

// Add inserts a new vector into the index
func (h *HNSWIndex) Add(vector Vector, payload interface{}) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	nodeID := len(h.nodes)
	
	// Determine random level for this node
	level := h.randomLevel()
	
	node := &HNSWNode{
		id:          nodeID,
		vector:      vector,
		payload:     payload,
		connections: make([][]int, level+1),
	}
	
	for i := range node.connections {
		node.connections[i] = make([]int, 0, h.M)
	}
	
	h.nodes = append(h.nodes, node)
	h.layers = append(h.layers, level)
	
	// If this is the first node, make it the entry point
	if h.entryPoint == -1 {
		h.entryPoint = nodeID
		return nodeID
	}
	
	// Find nearest neighbors and insert
	h.insert(node, level)
	
	return nodeID
}

// insert adds a node to the graph structure
func (h *HNSWIndex) insert(node *HNSWNode, level int) {
	// Find nearest neighbors at each layer
	entryPoints := []int{h.entryPoint}
	
	// Search from top layer down to target layer
	for lc := len(h.nodes[h.entryPoint].connections) - 1; lc > level; lc-- {
		nearest := h.searchLayer(node.vector, entryPoints, 1, lc)
		if len(nearest) > 0 {
			entryPoints = []int{nearest[0].id}
		}
	}
	
	// Insert at each layer from level down to 0
	for lc := level; lc >= 0; lc-- {
		candidates := h.searchLayer(node.vector, entryPoints, h.efConstruction, lc)
		
		// Select M neighbors using heuristic
		M := h.M
		if lc == 0 {
			M = h.Mmax0
		}
		
		neighbors := h.selectNeighbors(candidates, M)
		
		// Add bidirectional links
		for _, neighbor := range neighbors {
			h.connect(node.id, neighbor.id, lc)
			
			// Prune neighbor's connections if needed
			maxConn := h.Mmax
			if lc == 0 {
				maxConn = h.Mmax0
			}
			
			if len(h.nodes[neighbor.id].connections[lc]) > maxConn {
				h.pruneConnections(neighbor.id, lc, maxConn)
			}
		}
		
		entryPoints = make([]int, len(neighbors))
		for i, n := range neighbors {
			entryPoints[i] = n.id
		}
	}
	
	// Update entry point if this node is higher
	if level > len(h.nodes[h.entryPoint].connections)-1 {
		h.entryPoint = node.id
	}
}

// Search finds the k nearest neighbors to the query vector
func (h *HNSWIndex) Search(query Vector, k int) []SearchResult {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	if h.entryPoint == -1 || len(h.nodes) == 0 {
		return nil
	}
	
	entryPoints := []int{h.entryPoint}
	
	// Search from top layer down to layer 0
	for lc := len(h.nodes[h.entryPoint].connections) - 1; lc > 0; lc-- {
		nearest := h.searchLayer(query, entryPoints, 1, lc)
		if len(nearest) > 0 {
			entryPoints = []int{nearest[0].id}
		}
	}
	
	// Search layer 0 with larger ef
	ef := k
	if ef < h.efConstruction {
		ef = h.efConstruction
	}
	
	candidates := h.searchLayer(query, entryPoints, ef, 0)
	
	// Return top k results
	if len(candidates) > k {
		candidates = candidates[:k]
	}
	
	results := make([]SearchResult, len(candidates))
	for i, c := range candidates {
		results[i] = SearchResult{
			ID:       c.id,
			Distance: c.distance,
			Vector:   h.nodes[c.id].vector,
			Payload:  h.nodes[c.id].payload,
		}
	}
	
	return results
}

// searchLayer searches for nearest neighbors at a specific layer
func (h *HNSWIndex) searchLayer(query Vector, entryPoints []int, ef int, layer int) []candidate {
	visited := make(map[int]bool)
	candidates := &candidateQueue{}
	results := &candidateQueue{}
	
	heap.Init(candidates)
	heap.Init(results)
	
	// Initialize with entry points
	for _, ep := range entryPoints {
		dist := h.distFunc(query, h.nodes[ep].vector)
		heap.Push(candidates, candidate{id: ep, distance: dist})
		heap.Push(results, candidate{id: ep, distance: -dist}) // Max heap for results
		visited[ep] = true
	}
	
	for candidates.Len() > 0 {
		current := heap.Pop(candidates).(candidate)
		
		if results.Len() > 0 && current.distance > -(*results)[0].distance {
			break
		}
		
		// Check neighbors at this layer
		if layer < len(h.nodes[current.id].connections) {
			for _, neighborID := range h.nodes[current.id].connections[layer] {
				if !visited[neighborID] {
					visited[neighborID] = true
					
					dist := h.distFunc(query, h.nodes[neighborID].vector)
					
					if results.Len() < ef || dist < -(*results)[0].distance {
						heap.Push(candidates, candidate{id: neighborID, distance: dist})
						heap.Push(results, candidate{id: neighborID, distance: -dist})
						
						if results.Len() > ef {
							heap.Pop(results)
						}
					}
				}
			}
		}
	}
	
	// Convert results to sorted list (ascending by distance)
	sorted := make([]candidate, results.Len())
	for i := len(sorted) - 1; i >= 0; i-- {
		c := heap.Pop(results).(candidate)
		sorted[i] = candidate{id: c.id, distance: -c.distance}
	}
	
	return sorted
}

// selectNeighbors selects M neighbors using a heuristic
func (h *HNSWIndex) selectNeighbors(candidates []candidate, M int) []candidate {
	if len(candidates) <= M {
		return candidates
	}
	
	// Simple heuristic: return M nearest
	return candidates[:M]
}

// connect creates a bidirectional link between two nodes at a layer
func (h *HNSWIndex) connect(a, b, layer int) {
	// Ensure both nodes have enough layers allocated
	if layer >= len(h.nodes[a].connections) {
		// Expand connections slice
		newConns := make([][]int, layer+1)
		copy(newConns, h.nodes[a].connections)
		for i := len(h.nodes[a].connections); i < layer+1; i++ {
			newConns[i] = make([]int, 0, h.M)
		}
		h.nodes[a].connections = newConns
	}
	
	if layer >= len(h.nodes[b].connections) {
		// Expand connections slice
		newConns := make([][]int, layer+1)
		copy(newConns, h.nodes[b].connections)
		for i := len(h.nodes[b].connections); i < layer+1; i++ {
			newConns[i] = make([]int, 0, h.M)
		}
		h.nodes[b].connections = newConns
	}
	
	// Add b to a's connections
	if !h.hasConnection(a, b, layer) {
		h.nodes[a].connections[layer] = append(h.nodes[a].connections[layer], b)
	}
	
	// Add a to b's connections
	if !h.hasConnection(b, a, layer) {
		h.nodes[b].connections[layer] = append(h.nodes[b].connections[layer], a)
	}
}

// hasConnection checks if a connection exists
func (h *HNSWIndex) hasConnection(from, to, layer int) bool {
	if layer >= len(h.nodes[from].connections) {
		return false
	}
	
	for _, conn := range h.nodes[from].connections[layer] {
		if conn == to {
			return true
		}
	}
	return false
}

// pruneConnections removes the farthest connections to maintain max count
func (h *HNSWIndex) pruneConnections(nodeID, layer, maxConn int) {
	connections := h.nodes[nodeID].connections[layer]
	if len(connections) <= maxConn {
		return
	}
	
	// Calculate distances to all neighbors
	candidates := make([]candidate, len(connections))
	for i, neighborID := range connections {
		dist := h.distFunc(h.nodes[nodeID].vector, h.nodes[neighborID].vector)
		candidates[i] = candidate{id: neighborID, distance: dist}
	}
	
	// Sort by distance and keep closest
	selected := h.selectNeighbors(candidates, maxConn)
	
	h.nodes[nodeID].connections[layer] = make([]int, len(selected))
	for i, c := range selected {
		h.nodes[nodeID].connections[layer][i] = c.id
	}
}

// randomLevel generates a random layer level for a new node
func (h *HNSWIndex) randomLevel() int {
	level := 0
	for rand.Float64() < h.ml && level < 16 {
		level++
	}
	return level
}

// Size returns the number of nodes in the index
func (h *HNSWIndex) Size() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.nodes)
}

// candidate represents a search candidate with distance
type candidate struct {
	id       int
	distance float64
}

// candidateQueue implements heap.Interface for priority queue
type candidateQueue []candidate

func (pq candidateQueue) Len() int           { return len(pq) }
func (pq candidateQueue) Less(i, j int) bool { return pq[i].distance < pq[j].distance }
func (pq candidateQueue) Swap(i, j int)      { pq[i], pq[j] = pq[j], pq[i] }

func (pq *candidateQueue) Push(x interface{}) {
	*pq = append(*pq, x.(candidate))
}

func (pq *candidateQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}

// SearchResult represents a search result
type SearchResult struct {
	ID       int
	Distance float64
	Vector   Vector
	Payload  interface{}
}

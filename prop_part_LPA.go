package main

import (
	"cmp" // For Go 1.21+ used in sorting Influence structs
	"fmt"

	// For Go 1.21+
	"maps"
	"slices"
	"sync"
)

// LabelSet represents the set of labels for a node (table).
// It's a map where keys are label strings and values are empty structs for set semantics.
type LabelSet map[string]struct{}

// Influence stores information about an influencing table and the constraint causing it.
type Influence struct {
	InfluencerTable string // The table that exerts influence (original ReferencedTable)
	ConstraintName  string // The FkConstraintName for this specific influence
}

// LPA encapsulates the state and logic for the Label Propagation Algorithm.
type LPA struct {
	// graph stores the directed graph.
	// For a key FkTable, the value is a slice of Influence structs.
	// Each struct details an InfluencerTable (ReferencedTable) and the ConstraintName.
	// FkTable is influenced by Influence.InfluencerTable via Influence.ConstraintName.
	graph map[string][]Influence

	// nodes is a sorted list of all unique table names. This primarily aids in
	// deterministic initialization of goroutines if needed, though the parallel
	// nature makes strict order less critical for computation itself.
	nodes []string

	// currentLabels holds the set of labels for each node.
	// Access to currentLabels is protected by mu.
	currentLabels map[string]LabelSet
	mu            sync.RWMutex
}

// NewLPA creates and initializes a new LPA instance from a list of relations.
// It builds the graph structure and sets initial labels (each node gets its own name as a label).
func NewLPA(relations []Relation) (*LPA, error) {
	adj := make(map[string][]Influence)
	nodeSet := make(map[string]struct{}) // To collect unique node names
	initialLabels := make(map[string]LabelSet)

	if relations == nil {
		// Handle nil input gracefully by returning an empty LPA state.
		// Depending on strictness, an error could be returned instead:
		// return nil, fmt.Errorf("input relations cannot be nil")
		return &LPA{
			graph:         adj,
			nodes:         []string{},
			currentLabels: initialLabels,
		}, nil
	}

	for _, rel := range relations {
		if rel.FkTable == "" || rel.ReferencedTable == "" {
			// Optional: Add more robust validation or logging for empty table names or constraint names.
			// fmt.Printf("Warning: Relation contains empty FkTable ('%s') or ReferencedTable ('%s')\n", rel.FkTable, rel.ReferencedTable)
		}

		// Add both tables to the set of known nodes
		nodeSet[rel.FkTable] = struct{}{}
		nodeSet[rel.ReferencedTable] = struct{}{}

		// The FkTable is influenced by the ReferencedTable.
		// Add ReferencedTable to the list of influencers for FkTable.
		influence := Influence{
			InfluencerTable: rel.ReferencedTable,
			ConstraintName:  rel.FkConstraintName, // Store the constraint name
		}
		adj[rel.FkTable] = append(adj[rel.FkTable], influence)
	}

	// Create a sorted list of nodes. Sorting helps in producing deterministic
	// outputs if the order of processing matters in any part (e.g., logging, testing).
	nodeList := make([]string, 0, len(nodeSet))
	for nodeName := range nodeSet {
		nodeList = append(nodeList, nodeName)
	}
	slices.Sort(nodeList) // Sorts in-place. Requires Go 1.21+.

	// Initialize labels: each node starts with its own name as its only label.
	// Also, ensure all nodes (even those only appearing as ReferencedTable)
	// exist in the adjacency list, potentially with an empty list of influencers.
	for _, nodeName := range nodeList {
		initialLabels[nodeName] = LabelSet{nodeName: {}}
		if _, ok := adj[nodeName]; !ok {
			// This node might only be a ReferencedTable or an isolated table.
			// It has no outgoing influences defined by FKs where it is the FkTable.
			adj[nodeName] = []Influence{} // Ensure all nodes exist in the graph map
		}
	}

	return &LPA{
		graph:         adj,
		nodes:         nodeList,
		currentLabels: initialLabels,
	}, nil
}

// Run executes the Overlapping Label Propagation Algorithm.
// It iterates up to maxIterations or until labels converge.
// Goroutines are used to compute label updates for nodes in parallel within each iteration.
// This implementation uses a synchronous update model for labels: all nodes' labels
// for iteration `k` are based on labels from iteration `k-1`.
// The overlapping nature (union of labels) means labels are only added,
// which makes the algorithm inherently stable and less prone to oscillations
// compared to majority-voting LPAs.
func (l *LPA) Run(maxIterations int) (map[string]LabelSet, int) {
	if len(l.nodes) == 0 {
		return make(map[string]LabelSet), 0
	}

	for iter := 0; iter < maxIterations; iter++ {
		changedInIteration := false

		// proposedLabelUpdates will store the newly computed LabelSets for each node in this iteration.
		// Access to this map from goroutines must be synchronized.
		proposedLabelUpdates := make(map[string]LabelSet, len(l.nodes))
		var proposedMu sync.Mutex
		var wg sync.WaitGroup

		// Create a snapshot of currentLabels for all goroutines to read from.
		// This ensures that all nodes in this iteration base their updates on the
		// state from the end of the previous iteration (or initial state for iter 0).
		// This is key for the "synchronous parallel" update style.
		l.mu.RLock()
		labelsSnapshot := make(map[string]LabelSet, len(l.currentLabels))
		for node, ls := range l.currentLabels {
			labelsSnapshot[node] = cloneLabelSet(ls) // Deep copy each LabelSet
		}
		l.mu.RUnlock()

		for _, nodeName := range l.nodes { // Iterate over a consistent list of nodes
			wg.Add(1)
			go func(currentNodeName string) {
				defer wg.Done()

				// Start with the node's own inherent label. This ensures it's always part of its set.
				newNodeLabels := LabelSet{currentNodeName: {}}

				// Get the list of nodes that influence the currentNode.
				// l.graph is read-only after LPA initialization, so no lock needed here.
				directInfluences := l.graph[currentNodeName]

				for _, influenceDetail := range directInfluences {
					influencerName := influenceDetail.InfluencerTable // Table name to look up labels
					// influenceDetail.ConstraintName is available here if needed for any logic,
					// but standard LPA just uses the influencer's labels.
					if influencerLabels, ok := labelsSnapshot[influencerName]; ok {
						for label := range influencerLabels {
							newNodeLabels[label] = struct{}{} // Add label to the set (union operation)
						}
					}
				}

				// Safely store the computed labels for this node.
				proposedMu.Lock()
				proposedLabelUpdates[currentNodeName] = newNodeLabels
				proposedMu.Unlock()
			}(nodeName)
		}
		wg.Wait() // Wait for all goroutines to compute their proposed labels.

		// Apply the proposed updates to currentLabels and check for convergence.
		l.mu.Lock()
		for nodeName, newLabels := range proposedLabelUpdates {
			// Compare newLabels with the actual l.currentLabels[nodeName] (not the snapshot)
			// to determine if a change occurred in this iteration.
			if !areLabelSetsEqual(l.currentLabels[nodeName], newLabels) {
				l.currentLabels[nodeName] = newLabels // Update the main label set for the next iteration
				changedInIteration = true
			}
		}
		l.mu.Unlock()

		if !changedInIteration {
			// fmt.Printf("LPA converged after %d iterations.\n", iter+1)
			return l.GetLabelsCopy(), iter + 1 // Converged. iter+1 because iter is 0-indexed.
		}
	}

	fmt.Printf("LPA reached max iterations (%d).\n", maxIterations)
	return l.GetLabelsCopy(), maxIterations // Max iterations reached
}

// GetLabelsCopy returns a deep copy of the current labels.
// This is safe for callers to read or modify without affecting the LPA's internal state.
func (l *LPA) GetLabelsCopy() map[string]LabelSet {
	l.mu.RLock()
	defer l.mu.RUnlock()

	copiedLabels := make(map[string]LabelSet, len(l.currentLabels))
	for node, ls := range l.currentLabels {
		copiedLabels[node] = cloneLabelSet(ls)
	}
	return copiedLabels
}

// GetGraphForOutput returns a copy of the graph structure for output purposes.
// This allows external functions to read graph details without modifying internal state.
func (l *LPA) GetGraphForOutput() map[string][]Influence {
	// Create a shallow copy of the map. The Influence slices are also copied.
	// Since Influence structs are simple and l.graph is read-only after NewLPA during Run,
	// this level of copying is generally safe for reading.
	copiedGraph := make(map[string][]Influence, len(l.graph))
	for FkTable, influences := range l.graph {
		if influences == nil { // Handle case of nodes with no outgoing FKs parsed
			copiedGraph[FkTable] = []Influence{}
		} else {
			influencesCopy := make([]Influence, len(influences))
			copy(influencesCopy, influences)
			copiedGraph[FkTable] = influencesCopy
		}
	}
	return copiedGraph
}

// cloneLabelSet creates a deep copy of a LabelSet.
func cloneLabelSet(ls LabelSet) LabelSet {
	if ls == nil {
		return make(LabelSet) // Return empty, non-nil LabelSet
	}
	// For Go 1.21+
	return maps.Clone(ls)
}

// areLabelSetsEqual checks if two LabelSets are identical.
func areLabelSetsEqual(s1, s2 LabelSet) bool {
	return maps.Equal(s1, s2) // Go 1.21+
}

// PrintDetailedOutput prints the labels for each table and their direct influences, including constraint names.
// It uses the LPA's nodes for sorted table output, current labels, and the graph for influence details.
func PrintDetailedOutput(title string, lpaInstance *LPA) {
	fmt.Println(title)

	// Access nodes and graph directly from lpaInstance. Labels are copied for safety if needed.
	// For printing, direct read with RLock is fine for currentLabels.
	// lpaInstance.nodes is sorted during NewLPA.

	lpaInstance.mu.RLock()
	defer lpaInstance.mu.RUnlock()

	for _, tableName := range lpaInstance.nodes { // Iterate in sorted order
		// Print Labels
		labels, ok := lpaInstance.currentLabels[tableName]
		if !ok {
			fmt.Printf("Table %s: (no labels found)\n", tableName) // Should not happen
			continue
		}
		labelList := make([]string, 0, len(labels))
		for label := range labels {
			labelList = append(labelList, label)
		}
		slices.Sort(labelList)
		fmt.Printf("Table %s: Labels %v\n", tableName, labelList)

		// Print Direct Influences from the perspective of FkTable <- ReferencedTable
		// The graph stores FkTable -> []Influence{ReferencedTable, ConstraintName}
		// This means tableName *is influenced by* entries in lpaInstance.graph[tableName]
		// No, the graph is FkTable -> []Influence{InfluencerTable, ConstraintName}
		// So, if `tableName` is an FkTable, `lpaInstance.graph[tableName]` lists its influencers.

		// The graph is defined such that graph[FkTable] contains ReferencedTable (InfluencerTable).
		// So, `tableName` is influenced by `influence.InfluencerTable`.
		influencesOnTable := lpaInstance.graph[tableName]

		if len(influencesOnTable) > 0 {
			fmt.Printf("  Directly influenced by (as FkTable via constraint):\n")

			// Sort influences for deterministic output
			sortedInfluences := make([]Influence, len(influencesOnTable))
			copy(sortedInfluences, influencesOnTable)
			slices.SortFunc(sortedInfluences, func(a, b Influence) int {
				// Using cmp.Compare (Go 1.21+)
				if c := cmp.Compare(a.InfluencerTable, b.InfluencerTable); c != 0 {
					return c
				}
				return cmp.Compare(a.ConstraintName, b.ConstraintName)
				// Alternative using strings.Compare:
				// if res := strings.Compare(a.InfluencerTable, b.InfluencerTable); res != 0 { return res }
				// return strings.Compare(a.ConstraintName, b.ConstraintName)
			})

			for _, influence := range sortedInfluences {
				fmt.Printf("    - Table '%s' (Constraint: '%s')\n", influence.InfluencerTable, influence.ConstraintName)
			}
		}
	}
}

/*
// Example Usage (typically in a main function or a test file)
func main() {
	relations := []Relation{
		{FkTable: "orders", ReferencedTable: "customers", FkConstraintName: "fk_orders_cust"},
		{FkTable: "order_items", ReferencedTable: "orders", FkConstraintName: "fk_oi_ord"},
		{FkTable: "order_items", ReferencedTable: "products", FkConstraintName: "fk_oi_prod"},
		{FkTable: "reviews", ReferencedTable: "products", FkConstraintName: "fk_rev_prod"},
		{FkTable: "reviews", ReferencedTable: "customers", FkConstraintName: "fk_rev_cust"},
		{FkTable: "employee_projects", ReferencedTable: "employees", FkConstraintName: "fk_ep_emp"},
		{FkTable: "employee_projects", ReferencedTable: "projects", FkConstraintName: "fk_ep_proj"},
		{FkTable: "project_tasks", ReferencedTable: "projects", FkConstraintName: "fk_pt_proj"},
		{FkTable: "employees", ReferencedTable: "departments", FkConstraintName: "fk_emp_dept"},
		// Example of a table that is only referenced, not an FkTable itself initially in this list
		// {FkTable: "some_other_table", ReferencedTable: "departments", FkConstraintName: "fk_sot_dept"},
		// Example of an isolated table (would need to be added manually if not in relations)
		// To ensure "isolated_table" is part of the system, it must appear in at least one relation,
		// or be added to nodeSet manually in NewLPA if such cases are possible.
		// For this LPA, nodes are derived from relations.
	}

	lpaAlg, err := NewLPA(relations)
	if err != nil {
		fmt.Printf("Error creating LPA: %v\n", err)
		return
	}

	PrintDetailedOutput("Initial State (Labels and Direct Influences):", lpaAlg)
	fmt.Println("---")

	// Run LPA with a maximum of 10 iterations
	_, iterations := lpaAlg.Run(10) // finalLabels are now read via lpaAlg for PrintDetailedOutput

	fmt.Printf("\n--- Running LPA ---\n")
	fmt.Printf("LPA finished after %d iterations.\n", iterations)
	fmt.Println("\nFinal State (Labels and Direct Influences):")
	PrintDetailedOutput("", lpaAlg) // Title is optional or can be part of the surrounding prints

	// Expected propagation:
	// - "departments" label should reach "employees", then "employee_projects".
	// - "customers" label should reach "orders", then "order_items". It also reaches "reviews".
	// - "products" label should reach "order_items" and "reviews".
	// - "projects" label should reach "employee_projects" and "project_tasks".
}
*/

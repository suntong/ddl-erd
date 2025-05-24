package main

import (
	"cmp"
	"fmt"
	"log"
	"slices"
)

// NewLPA creates and initializes a new LPA instance from a list of relations.
// It builds the graph structure (including constraint names) and sets initial labels.
func NewLPA(relations []Relation) (*LPA, error) {
	adj := make(map[string][]Influence) // Changed to store Influence structs
	nodeSet := make(map[string]struct{})
	initialLabels := make(map[string]LabelSet)

	if relations == nil {
		return &LPA{
			graph:         adj,
			nodes:         []string{},
			CurrentLabels: initialLabels,
		}, nil
	}

	for _, rel := range relations {
		if rel.FkTable == "" || rel.ReferencedTable == "" {
			// Optional: Add more robust validation or logging for empty table names or constraint names.
			// fmt.Printf("Warning: Relation contains empty FkTable ('%s') or ReferencedTable ('%s')\n", rel.FkTable, rel.ReferencedTable)
		}

		nodeSet[rel.FkTable] = struct{}{}
		nodeSet[rel.ReferencedTable] = struct{}{}

		// FkTable is influenced by ReferencedTable via FkConstraintName.
		influence := Influence{
			InfluencerTable: rel.ReferencedTable,
			ConstraintName:  rel.FkConstraintName, // Store the constraint name
		}
		adj[rel.FkTable] = append(adj[rel.FkTable], influence)
	}

	nodeList := make([]string, 0, len(nodeSet))
	for nodeName := range nodeSet {
		nodeList = append(nodeList, nodeName)
	}
	slices.Sort(nodeList)

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
		CurrentLabels: initialLabels,
	}, nil
}

func generateLPADotFiles(relations []Relation, tablesDisplay map[string]*TableDisplayInfo) {
	lpa, err := NewLPA(relations)
	if err != nil {
		log.Fatalf("Error creating LPA: %v\n", err)
	}
	iterations := lpa.Run(10) // finalLabels are now read via lpa for PrintDetailedOutput
	fmt.Printf("LPA finished after %d iterations.\n", iterations)
	// lpa.Dump()

	for _, tableName := range lpa.Nodes() { // Iterate in sorted order
		// Labels (directed graph of FkTable -> ReferencedTable)
		labels, ok := lpa.CurrentLabels[tableName]
		if !ok {
			fmt.Printf("Internal error -- Table %s: (no labels found)\n", tableName) // Should not happen
			continue
		}
		if len(labels) == 1 {
			fmt.Printf("== No Ref Tables (%s). Skip sub-graphing\n", tableName)
			continue
		}

		labelList := make([]string, 0, len(labels))
		for label := range labels {
			labelList = append(labelList, label)
		}
		slices.Sort(labelList)
		fmt.Printf("\n== Generating for Table %s: Labels %v\n", tableName, labelList)

		refTables := lpa.RefTables(tableName)
		// Sort for deterministic output
		sortedRefTables := make([]Influence, len(refTables))
		copy(sortedRefTables, refTables)
		slices.SortFunc(sortedRefTables, func(a, b Influence) int {
			// Using cmp.Compare (Go 1.21+)
			if c := cmp.Compare(a.InfluencerTable, b.InfluencerTable); c != 0 {
				return c
			}
			return cmp.Compare(a.ConstraintName, b.ConstraintName)
		})

		// Tables in sub-graph are current tableName + sortedRefTables
		//subTables := make([]string, 0, len(refTables)+1)
		//subTables = append(subTables, tableName)

		subTablesDisplay := make(map[string]*TableDisplayInfo)
		subTablesDisplay[tableName] = tablesDisplay[tableName]
		for _, table := range sortedRefTables {
			subTablesDisplay[table.InfluencerTable] =
				tablesDisplay[table.InfluencerTable]
		}
		sb := buildTable(relations, subTablesDisplay)

		// build LPA sub Edges
		uniqueEdges := make(map[string]bool) // To avoid duplicate edges if input has redundancy
		for _, ref := range sortedRefTables {
			//fmt.Printf("    - Table '%s' (Constraint: '%s')\n", ref.InfluencerTable, ref.ConstraintName)
			edgeKey := fmt.Sprintf("\"%s\" -> \"%s\" :: %s", tableName, ref.InfluencerTable, ref.ConstraintName)
			if !uniqueEdges[edgeKey] {
				sb.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\" [label=\" %s \"];\n",
					tableName, ref.InfluencerTable, ref.ConstraintName))
				uniqueEdges[edgeKey] = true
			}
		}
		sb.WriteString("}\n")
		fmt.Println(sb.String())
	}
}

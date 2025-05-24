package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// --- Data Structures ---

// Relation represents a parsed foreign key relationship.
type Relation struct {
	FkTable             string
	FkConstraintName    string
	FkColumns           []string
	ReferencedTable     string
	ReferencedPkColumns []string // Columns in the referenced table targeted by the FK
}

// For DOT Generation:
// FKDetail describes a foreign key constraint originating from a table.
type FKDetail struct {
	ConstraintName    string
	LocalColumns      []string
	ReferencedTable   string
	ReferencedColumns []string
}

// ReferencedByDetail describes an incoming foreign key referencing a table.
type ReferencedByDetail struct {
	IncomingFKConstraintName string   // Name of the FK constraint on the other table
	FromTable                string   // The table that defines this incoming FK
	FromTableFkColumns       []string // The FK columns in the FromTable
	ThisTableReferencedCols  []string // Columns in the current table that are referenced
}

// TableDisplayInfo aggregates information for displaying a single table node in Graphviz.
type TableDisplayInfo struct {
	Name              string
	DefinedFKs        []FKDetail           // FKs originating from this table
	ReferencedBy      []ReferencedByDetail // FKs from other tables pointing to this table's columns
	ReferencedColsMap map[string]bool      // Helper to list unique columns of this table that are referenced
}

// --- Core Functions ---

func readInput() (string, error) {
	inputBytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("failed to read from stdin: %w", err)
	}
	return string(inputBytes), nil
}

func generateDot(relations []Relation, tablesDisplay map[string]*TableDisplayInfo) string {
	sb := buildTable(relations, tablesDisplay)
	return buildEdges(relations, sb)
}

func defineTableDisplay(relations []Relation) map[string]*TableDisplayInfo {
	tablesDisplay := make(map[string]*TableDisplayInfo)

	// Aggregate information into TableDisplayInfo
	for _, rel := range relations {
		// Ensure FkTable exists and add to its DefinedFKs
		if _, ok := tablesDisplay[rel.FkTable]; !ok {
			tablesDisplay[rel.FkTable] = &TableDisplayInfo{Name: rel.FkTable, ReferencedColsMap: make(map[string]bool)}
		}
		tablesDisplay[rel.FkTable].DefinedFKs = append(tablesDisplay[rel.FkTable].DefinedFKs, FKDetail{
			ConstraintName:    rel.FkConstraintName,
			LocalColumns:      rel.FkColumns,
			ReferencedTable:   rel.ReferencedTable,
			ReferencedColumns: rel.ReferencedPkColumns,
		})

		// Ensure ReferencedTable exists and add to its ReferencedBy
		if _, ok := tablesDisplay[rel.ReferencedTable]; !ok {
			tablesDisplay[rel.ReferencedTable] = &TableDisplayInfo{Name: rel.ReferencedTable, ReferencedColsMap: make(map[string]bool)}
		}
		tablesDisplay[rel.ReferencedTable].ReferencedBy = append(tablesDisplay[rel.ReferencedTable].ReferencedBy, ReferencedByDetail{
			IncomingFKConstraintName: rel.FkConstraintName,
			FromTable:                rel.FkTable,
			FromTableFkColumns:       rel.FkColumns,
			ThisTableReferencedCols:  rel.ReferencedPkColumns,
		})
		for _, col := range rel.ReferencedPkColumns {
			tablesDisplay[rel.ReferencedTable].ReferencedColsMap[col] = true
		}
	}
	return tablesDisplay
}

func buildTable(relations []Relation, tablesDisplay map[string]*TableDisplayInfo) *strings.Builder {
	var sb strings.Builder

	sb.WriteString("digraph ERD {\n")
	sb.WriteString("  rankdir=LR;\n")
	// sb.WriteString("  graph [splines=ortho];\n") // Using ortho splines
	sb.WriteString("  node [shape=plaintext, style=filled, fillcolor=lightyellow];\n")
	// Using xlabel for edges, as requested
	sb.WriteString("  edge [arrowhead=crow, arrowtail=none, dir=both, labelfontsize=10];\n\n")

	// Define table nodes with HTML-like labels
	for _, table := range tablesDisplay {
		sb.WriteString(fmt.Sprintf("  \"%s\" [\n    label=<\n", table.Name))
		sb.WriteString("    <TABLE BORDER=\"0\" CELLBORDER=\"1\" CELLSPACING=\"0\" BGCOLOR=\"lightyellow\">\n")
		// Table Name Header
		sb.WriteString(fmt.Sprintf("      <TR><TD COLSPAN=\"2\" BGCOLOR=\"lightblue\"><B>%s</B></TD></TR>\n", table.Name))

		// Columns that are referenced by other tables (acting as PK/UK for those FKs)
		if len(table.ReferencedBy) > 0 {
			sb.WriteString("      <TR><TD COLSPAN=\"2\" ALIGN=\"LEFT\"><I>Referenced By Other Tables As Key:</I></TD></TR>\n")
			// List unique referenced columns of this table
			uniqueRefCols := []string{}
			for col := range table.ReferencedColsMap {
				uniqueRefCols = append(uniqueRefCols, col)
			}
			// Sort for consistent output (optional)
			// sort.Strings(uniqueRefCols)
			for _, col := range uniqueRefCols {
				sb.WriteString(fmt.Sprintf("      <TR><TD ALIGN=\"LEFT\">%s</TD><TD ALIGN=\"LEFT\">(Referenced)</TD></TR>\n", col))
			}

			// Optionally, list details of each incoming FK (can be verbose)
			// for _, ref := range table.ReferencedBy {
			// 	 refByLabel := fmt.Sprintf("%s from %s (%s &rarr; %s)",
			// 	    ref.IncomingFKConstraintName, ref.FromTable,
			// 	    strings.Join(ref.FromTableFkColumns, ", "),
			// 	    strings.Join(ref.ThisTableReferencedCols, ", "))
			// 	 sb.WriteString(fmt.Sprintf("      <TR><TD ALIGN=\"LEFT\" COLSPAN=\"2\">%s</TD></TR>\n", refByLabel))
			// }
		}

		// Foreign Keys defined in this table
		if len(table.DefinedFKs) > 0 {
			sb.WriteString("      <TR><TD COLSPAN=\"2\" ALIGN=\"LEFT\"><I>Foreign Keys Defined Here:</I></TD></TR>\n")
			for _, fk := range table.DefinedFKs {
				localColsStr := strings.Join(fk.LocalColumns, ", ")
				refColsStr := strings.Join(fk.ReferencedColumns, ", ")
				fkLabel := fmt.Sprintf("%s (%s) &rarr; %s (%s)", fk.ConstraintName, localColsStr, fk.ReferencedTable, refColsStr)
				sb.WriteString(fmt.Sprintf("      <TR><TD ALIGN=\"LEFT\" COLSPAN=\"2\">%s</TD></TR>\n", fkLabel))
				// List the local FK columns separately for clarity
				// for _, col := range fk.LocalColumns {
				// 	sb.WriteString(fmt.Sprintf("      <TR><TD ALIGN=\"LEFT\">%s</TD><TD ALIGN=\"LEFT\">(FK)</TD></TR>\n", col))
				// }
			}
		}
		sb.WriteString("    </TABLE>\n")
		sb.WriteString("    >\n  ];\n\n")
	}

	return &sb
}

func buildEdges(relations []Relation, sb *strings.Builder) string {
	// Define edges for foreign key relationships
	uniqueEdges := make(map[string]bool) // To avoid duplicate edges if input has redundancy
	for _, rel := range relations {
		edgeKey := fmt.Sprintf("\"%s\" -> \"%s\" :: %s", rel.FkTable, rel.ReferencedTable, rel.FkConstraintName)
		if !uniqueEdges[edgeKey] {
			sb.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\" [label=\" %s \"];\n",
				rel.FkTable, rel.ReferencedTable, rel.FkConstraintName))
			uniqueEdges[edgeKey] = true
		}
	}

	sb.WriteString("}\n")
	return sb.String()
}

// --- Helper Functions ---

// SanitizeIdentifier removes schema prefixes and brackets from SQL identifiers.
// Examples: [dbo].[MyTable] -> MyTable, dbo.MyTable -> MyTable, [MyTable] -> MyTable, MyTable -> MyTable
func SanitizeIdentifier(raw string) string {
	s := strings.TrimSpace(raw)
	// Remove brackets from individual parts if they exist, e.g. [dbo].[table] -> dbo.table
	// This regex finds content within brackets: [content] -> content
	reBrackets := regexp.MustCompile(`\[([^\]]+)\]`)
	s = reBrackets.ReplaceAllString(s, "$1")

	// After brackets are handled (e.g., "[dbo].[TABLE]" becomes "dbo.TABLE"),
	// split by '.' and take the last part for the actual identifier.
	parts := strings.Split(s, ".")
	return strings.TrimSpace(parts[len(parts)-1])
}

// SanitizeIdentifierList takes a comma-separated string of identifiers,
// sanitizes each, and returns a slice of strings.
func SanitizeIdentifierList(rawList string) []string {
	rawIdentifiers := strings.Split(rawList, ",")
	sanitizedList := make([]string, 0, len(rawIdentifiers))
	for _, rawID := range rawIdentifiers {
		trimmedID := strings.TrimSpace(rawID)
		if trimmedID != "" {
			sanitizedList = append(sanitizedList, SanitizeIdentifier(trimmedID))
		}
	}
	return sanitizedList
}

// extractNamedGroups converts regex matches with named capture groups into a map.
func extractNamedGroups(regex *regexp.Regexp, matches []string) map[string]string {
	result := make(map[string]string)
	for i, name := range regex.SubexpNames() {
		if i != 0 && name != "" && i < len(matches) { // Ensure index is valid
			result[name] = strings.TrimSpace(matches[i])
		}
	}
	return result
}

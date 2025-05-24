package main

import (
	"regexp"
)

func parseSQL(sqlContent string) []Relation {
	var relations []Relation

	// Regex patterns for identifiers
	// An identifier can be [bracketed] or unbracketed_word.
	// A schema.table can be [schema].[table], schema.table, [schema].table, schema.[table]
	tableIdentifierPattern := `(?:\[[^\]]+\]|[\w_]+)(?:\s*\.\s*(?:\[[^\]]+\]|[\w_]+))*`
	constraintNamePattern := `(?:\[[^\]]+\]|[\w_]+)`
	// Matches one or more columns, potentially bracketed, separated by commas
	columnListPattern := `(?:\[[^\]]+\]|[\w_]+)(?:\s*,\s*(?:\[[^\]]+\]|[\w_]+))*`

	// Regex for Foreign Key constraints, of the format:
	// ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY (...) REFERENCES ... (...)
	// Like:
	// ALTER TABLE Sales.TempSalesReason
	//   ADD CONSTRAINT FK_TempSales_SalesReason FOREIGN KEY (TempID)
	//   REFERENCES Sales.SalesReason (SalesReasonID)
	// (?is) for case-insensitive and dot-matches-newline
	fkRegexStr := `(?is)ALTER\s+TABLE\s+(?P<FkTable>` + tableIdentifierPattern + `)\s+` +
		`ADD\s+CONSTRAINT\s+(?P<ConstraintName>` + constraintNamePattern + `)\s+` +
		`FOREIGN\s+KEY\s*\((?P<FkColumns>` + columnListPattern + `)\)\s+` +
		`REFERENCES\s+(?P<PkTable>` + tableIdentifierPattern + `)\s*` +
		`\((?P<PkColumns>` + columnListPattern + `)\)`
	fkRegex := regexp.MustCompile(fkRegexStr)

	allMatches := fkRegex.FindAllStringSubmatch(sqlContent, -1)

	for _, matchData := range allMatches {
		result := extractNamedGroups(fkRegex, matchData)

		fkTableName := SanitizeIdentifier(result["FkTable"])
		constraintName := SanitizeIdentifier(result["ConstraintName"])
		fkColumns := SanitizeIdentifierList(result["FkColumns"])
		pkTableName := SanitizeIdentifier(result["PkTable"])
		pkColumns := SanitizeIdentifierList(result["PkColumns"])

		relations = append(relations, Relation{
			FkTable:             fkTableName,
			FkConstraintName:    constraintName,
			FkColumns:           fkColumns,
			ReferencedTable:     pkTableName,
			ReferencedPkColumns: pkColumns,
		})
	}
	return relations
}

package golang

import "strings"

type DynamicQuery struct {
	StaticCount int
	Opts        []DynamicPredicate
	SortColumns []DynamicSortColumn
}

type DynamicPredicate struct {
	FieldName string
	VarName   string
	GoType    string
	Column    string
	SQLOp     string
	IsSlice   bool
}

type DynamicSortColumn struct {
	ConstName string
	Value     string
}

// parseDynamicComments extracts dynamic query annotations from
// a query's comments. The @dynamic param -> operator map and the @dynamic-sort column
// allowlist. It also returns the remaining comments with those annotation lines removed
// to avoid rendering them as Go doc commetns.
func parseDynamicComments(comments []string) (ops map[string]string, sort, filtered []string) {
	ops = make(map[string]string)
	for _, c := range comments {
		fields := strings.Fields(c)
		if len(fields) == 0 {
			filtered = append(filtered, c)
			continue
		}
		switch fields[0] {
		case "@dynamic":
			if len(fields) >= 2 {
				ops[fields[1]] = ""
			}
		case "@dynamic-sort":
			for _, col := range fields[1:] {
				if col = strings.TrimSuffix(col, ","); col != "" {
					sort = append(sort, col)
				}
			}
		default:
			filtered = append(filtered, c)
		}
	}
	return ops, sort, filtered
}

package golang

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

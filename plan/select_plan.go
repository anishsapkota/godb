package plan

import (
	"godb/query"
	"godb/query/scan"
	"godb/record"
)

type SelectPlan struct {
	plan      Plan
	predicate *query.Predicate
}

func NewSelectPlan(p Plan, pred *query.Predicate) *SelectPlan {
	return &SelectPlan{plan: p, predicate: pred}
}

func (sp *SelectPlan) Open() scan.Scan {
	s := sp.plan.Open()
	sel, err := query.NewSelectScan(s, sp.predicate)
	if err != nil {
		panic(err)
	}
	return sel
}

func (sp *SelectPlan) BlocksAccessed() int {
	return sp.plan.BlocksAccessed()
}

func (sp *SelectPlan) RecordsOutput() int {
	rf := sp.predicate.ReductionFactor(sp.plan)
	if rf == 0 {
		return sp.plan.RecordsOutput()
	}
	return sp.plan.RecordsOutput() / rf
}

func (sp *SelectPlan) DistinctValues(fieldName string) int {
	if sp.predicate.EquatesWithConstant(fieldName) != nil {
		return 1
	}
	if other := sp.predicate.EquatesWithField(fieldName); other != "" {
		return min(sp.plan.DistinctValues(fieldName), sp.plan.DistinctValues(other))
	}
	return sp.plan.DistinctValues(fieldName)
}

func (sp *SelectPlan) Schema() *record.Schema {
	return sp.plan.Schema()
}

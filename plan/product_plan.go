package plan

import (
	"godb/query"
	"godb/query/scan"
	"godb/record"
)

type ProductPlan struct {
	plan1  Plan
	plan2  Plan
	schema *record.Schema
}

func NewProductPlan(p1, p2 Plan) *ProductPlan {
	schema := record.NewSchema()
	schema.AddAll(p1.Schema())
	schema.AddAll(p2.Schema())
	return &ProductPlan{plan1: p1, plan2: p2, schema: schema}
}

func (pp *ProductPlan) Open() scan.Scan {
	s1 := pp.plan1.Open()
	s2 := pp.plan2.Open()
	return query.NewProductScan(s1, s2)
}

func (pp *ProductPlan) BlocksAccessed() int {
	return pp.plan1.BlocksAccessed() + pp.plan1.RecordsOutput()*pp.plan2.BlocksAccessed()
}

func (pp *ProductPlan) RecordsOutput() int {
	return pp.plan1.RecordsOutput() * pp.plan2.RecordsOutput()
}

func (pp *ProductPlan) DistinctValues(fieldName string) int {
	if pp.plan1.Schema().HasField(fieldName) {
		return pp.plan1.DistinctValues(fieldName)
	}
	return pp.plan2.DistinctValues(fieldName)
}

func (pp *ProductPlan) Schema() *record.Schema {
	return pp.schema
}

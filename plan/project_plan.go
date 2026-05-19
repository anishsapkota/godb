package plan

import (
	"mydb/query"
	"mydb/query/scan"
	"mydb/record"
)

type ProjectPlan struct {
	plan   Plan
	schema *record.Schema
}

func NewProjectPlan(p Plan, fieldList []string) *ProjectPlan {
	schema := record.NewSchema()
	for _, fieldName := range fieldList {
		schema.Add(fieldName, p.Schema())
	}
	return &ProjectPlan{plan: p, schema: schema}
}

func (pp *ProjectPlan) Open() scan.Scan {
	s := pp.plan.Open()
	proj, err := query.NewProjectScan(s, pp.schema.Fields())
	if err != nil {
		panic(err)
	}
	return proj
}

func (pp *ProjectPlan) BlocksAccessed() int {
	return pp.plan.BlocksAccessed()
}

func (pp *ProjectPlan) RecordsOutput() int {
	return pp.plan.RecordsOutput()
}

func (pp *ProjectPlan) DistinctValues(fieldName string) int {
	return pp.plan.DistinctValues(fieldName)
}

func (pp *ProjectPlan) Schema() *record.Schema {
	return pp.schema
}

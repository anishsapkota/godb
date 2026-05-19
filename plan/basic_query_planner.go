package plan

import (
	"mydb/metadata"
	"mydb/parse"
	"mydb/tx"
)

type QueryPlanner interface {
	CreatePlan(data *parse.QueryData, tx *tx.Transaction) (Plan, error)
}

type BasicQueryPlanner struct {
	mdm *metadata.Manager
}

func NewBasicQueryPlanner(mdm *metadata.Manager) *BasicQueryPlanner {
	return &BasicQueryPlanner{mdm: mdm}
}

func (qp *BasicQueryPlanner) CreatePlan(data *parse.QueryData, tx *tx.Transaction) (Plan, error) {
	var plans []Plan
	for _, tableName := range data.Tables() {
		viewDef, err := qp.mdm.GetViewDefinition(tableName, tx)
		if err == nil && viewDef != "" {
			parser := parse.NewParser(viewDef)
			viewData, err := parser.Query()
			if err != nil {
				return nil, err
			}
			viewPlan, err := qp.CreatePlan(viewData, tx)
			if err != nil {
				return nil, err
			}
			plans = append(plans, viewPlan)
		} else {
			tp, err := NewTablePlan(tx, tableName, qp.mdm)
			if err != nil {
				return nil, err
			}
			plans = append(plans, tp)
		}
	}

	p := plans[0]
	for _, next := range plans[1:] {
		p = NewProductPlan(p, next)
	}
	p = NewSelectPlan(p, data.Pred())
	return NewProjectPlan(p, data.Fields()), nil
}

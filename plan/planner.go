package plan

import (
	"fmt"
	"mydb/parse"
	"mydb/tx"
)

type Planner struct {
	qp QueryPlanner
	up UpdatePlanner
}

func NewPlanner(qp QueryPlanner, up UpdatePlanner) *Planner {
	return &Planner{qp: qp, up: up}
}

func (p *Planner) CreateQueryPlan(sql string, tx *tx.Transaction) (Plan, error) {
	parser := parse.NewParser(sql)
	data, err := parser.Query()
	if err != nil {
		return nil, err
	}
	return p.qp.CreatePlan(data, tx)
}

func (p *Planner) ExecuteUpdate(sql string, tx *tx.Transaction) (int, error) {
	parser := parse.NewParser(sql)
	cmd, err := parser.UpdateCmd()
	if err != nil {
		return 0, err
	}
	switch data := cmd.(type) {
	case *parse.InsertData:
		return p.up.ExecuteInsert(data, tx)
	case *parse.DeleteData:
		return p.up.ExecuteDelete(data, tx)
	case *parse.ModifyData:
		return p.up.ExecuteModify(data, tx)
	case *parse.CreateTableData:
		return p.up.ExecuteCreateTable(data, tx)
	case *parse.CreateViewData:
		return p.up.ExecuteCreateView(data, tx)
	case *parse.CreateIndexData:
		return p.up.ExecuteCreateIndex(data, tx)
	default:
		return 0, fmt.Errorf("unknown command type: %T", cmd)
	}
}

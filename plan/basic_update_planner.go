package plan

import (
	"godb/metadata"
	"godb/parse"
	"godb/query/scan"
	"godb/query/table"
	"godb/tx"
)

type UpdatePlanner interface {
	ExecuteInsert(data *parse.InsertData, tx *tx.Transaction) (int, error)
	ExecuteDelete(data *parse.DeleteData, tx *tx.Transaction) (int, error)
	ExecuteModify(data *parse.ModifyData, tx *tx.Transaction) (int, error)
	ExecuteCreateTable(data *parse.CreateTableData, tx *tx.Transaction) (int, error)
	ExecuteCreateView(data *parse.CreateViewData, tx *tx.Transaction) (int, error)
	ExecuteCreateIndex(data *parse.CreateIndexData, tx *tx.Transaction) (int, error)
}

type BasicUpdatePlanner struct {
	mdm *metadata.Manager
}

func NewBasicUpdatePlanner(mdm *metadata.Manager) *BasicUpdatePlanner {
	return &BasicUpdatePlanner{mdm: mdm}
}

func (up *BasicUpdatePlanner) ExecuteInsert(data *parse.InsertData, tx *tx.Transaction) (int, error) {
	layout, err := up.mdm.GetLayout(data.TableName(), tx)
	if err != nil {
		return 0, err
	}
	ts, err := table.NewTableScan(tx, data.TableName(), layout)
	if err != nil {
		return 0, err
	}
	defer ts.Close()
	if err := ts.Insert(); err != nil {
		return 0, err
	}
	fields := data.Fields()
	values := data.Values()
	for i, field := range fields {
		if err := ts.SetVal(field, values[i]); err != nil {
			return 0, err
		}
	}
	return 1, nil
}

func (up *BasicUpdatePlanner) ExecuteDelete(data *parse.DeleteData, tx *tx.Transaction) (int, error) {
	tp, err := NewTablePlan(tx, data.TableName(), up.mdm)
	if err != nil {
		return 0, err
	}
	s := NewSelectPlan(tp, data.Predicate()).Open()
	us := s.(scan.UpdateScan)
	count := 0
	for {
		ok, err := s.Next()
		if err != nil {
			s.Close()
			return count, err
		}
		if !ok {
			break
		}
		if err := us.Delete(); err != nil {
			s.Close()
			return count, err
		}
		count++
	}
	s.Close()
	return count, nil
}

func (up *BasicUpdatePlanner) ExecuteModify(data *parse.ModifyData, tx *tx.Transaction) (int, error) {
	tp, err := NewTablePlan(tx, data.TableName(), up.mdm)
	if err != nil {
		return 0, err
	}
	s := NewSelectPlan(tp, data.Predicate()).Open()
	us := s.(scan.UpdateScan)
	count := 0
	for {
		ok, err := s.Next()
		if err != nil {
			s.Close()
			return count, err
		}
		if !ok {
			break
		}
		val, err := data.NewValue().Evaluate(s)
		if err != nil {
			s.Close()
			return count, err
		}
		if err := us.SetVal(data.TargetField(), val); err != nil {
			s.Close()
			return count, err
		}
		count++
	}
	s.Close()
	return count, nil
}

func (up *BasicUpdatePlanner) ExecuteCreateTable(data *parse.CreateTableData, tx *tx.Transaction) (int, error) {
	return 0, up.mdm.CreateTable(data.TableName(), data.NewSchema(), tx)
}

func (up *BasicUpdatePlanner) ExecuteCreateView(data *parse.CreateViewData, tx *tx.Transaction) (int, error) {
	return 0, up.mdm.CreateView(data.ViewName(), data.ViewDefinition(), tx)
}

func (up *BasicUpdatePlanner) ExecuteCreateIndex(data *parse.CreateIndexData, tx *tx.Transaction) (int, error) {
	return 0, up.mdm.CreateIndex(data.IndexName(), data.TableName(), data.FieldName(), tx)
}

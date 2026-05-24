package plan

import (
	"godb/metadata"
	"godb/query/scan"
	"godb/query/table"
	"godb/record"
	"godb/tx"
)

type TablePlan struct {
	tableName string
	tx        *tx.Transaction
	layout    *record.Layout
	statInfo  *metadata.StatInfo
}

func NewTablePlan(tx *tx.Transaction, tableName string, mdm *metadata.Manager) (*TablePlan, error) {
	layout, err := mdm.GetLayout(tableName, tx)
	if err != nil {
		return nil, err
	}
	statInfo, err := mdm.GetStatInfo(tableName, layout, tx)
	if err != nil {
		return nil, err
	}
	return &TablePlan{tableName: tableName, tx: tx, layout: layout, statInfo: statInfo}, nil
}

func (tp *TablePlan) Open() scan.Scan {
	ts, err := table.NewTableScan(tp.tx, tp.tableName, tp.layout)
	if err != nil {
		panic(err)
	}
	return ts
}

func (tp *TablePlan) BlocksAccessed() int {
	return tp.statInfo.BlocksAccessed()
}

func (tp *TablePlan) RecordsOutput() int {
	return tp.statInfo.RecordsOutput()
}

func (tp *TablePlan) DistinctValues(fieldName string) int {
	return tp.statInfo.DistinctValues(fieldName)
}

func (tp *TablePlan) Schema() *record.Schema {
	return tp.layout.Schema()
}

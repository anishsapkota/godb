package server

import (
	"fmt"
	"godb/buffer"
	"godb/file"
	"godb/log"
	"godb/metadata"
	"godb/plan"
	"godb/tx"
	"godb/tx/concurrency"
)

const (
	blockSize  = 4096
	bufferSize = 8
	logFile    = "godb.log"
)

type GoDB struct {
	fileManager     *file.Manager
	bufferManager   *buffer.Manager
	logManager      *log.Manager
	metadataManager *metadata.Manager
	lockTable       *concurrency.LockTable
	planner         *plan.Planner
}

// NewGoDBWithOptions is a constructor that is mostly useful for debugging purposes.
func NewGoDBWithOptions(dirName string, blockSize, bufferSize int) (*GoDB, error) {
	db := &GoDB{}
	var err error

	if db.fileManager, err = file.NewManager(dirName, blockSize); err != nil {
		return nil, err
	}
	if db.logManager, err = log.NewLogManager(db.fileManager, logFile); err != nil {
		return nil, err
	}
	db.bufferManager = buffer.NewManager(db.fileManager, db.logManager, bufferSize)
	db.lockTable = concurrency.NewLockTable()

	return db, nil
}

// NewGoDB creates a new GoDB instance.
func NewGoDB(dirName string) (*GoDB, error) {
	db, err := NewGoDBWithOptions(dirName, blockSize, bufferSize)
	if err != nil {
		return nil, err
	}

	transaction := db.NewTx()
	isNew := db.fileManager.IsNew()

	if isNew {
		fmt.Printf("creating new database\n")
	} else {
		fmt.Printf("recovering existing database\n")
		if err := transaction.Recover(); err != nil {
			return nil, err
		}
	}

	db.metadataManager, err = metadata.NewManager(isNew, transaction)
	if err != nil {
		transaction.Rollback()
		return nil, fmt.Errorf("metadata init: %w", err)
	}

	qp := plan.NewBasicQueryPlanner(db.metadataManager)
	up := plan.NewBasicUpdatePlanner(db.metadataManager)
	db.planner = plan.NewPlanner(qp, up)

	err = transaction.Commit()
	return db, err
}

func (db *GoDB) NewTx() *tx.Transaction {
	return tx.NewTransaction(db.fileManager, db.logManager, db.bufferManager, db.lockTable)
}

func (db *GoDB) MetadataManager() *metadata.Manager {
	return db.metadataManager
}

func (db *GoDB) FileManager() *file.Manager {
	return db.fileManager
}

func (db *GoDB) LogManager() *log.Manager {
	return db.logManager
}

func (db *GoDB) BufferManager() *buffer.Manager {
	return db.bufferManager
}

func (db *GoDB) Planner() *plan.Planner {
	return db.planner
}

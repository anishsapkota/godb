package server

import (
	"fmt"
	"mydb/buffer"
	"mydb/file"
	"mydb/log"
	"mydb/metadata"
	"mydb/plan"
	"mydb/tx"
	"mydb/tx/concurrency"
)

const (
	blockSize  = 4096
	bufferSize = 8
	logFile    = "mydb.log"
)

type MyDB struct {
	fileManager     *file.Manager
	bufferManager   *buffer.Manager
	logManager      *log.Manager
	metadataManager *metadata.Manager
	lockTable       *concurrency.LockTable
	planner         *plan.Planner
}

// NewMyDBWithOptions is a constructor that is mostly useful for debugging purposes.
func NewMyDBWithOptions(dirName string, blockSize, bufferSize int) (*MyDB, error) {
	db := &MyDB{}
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

// NewMyDB creates a new MyDB instance. Use this constructor for production code.
func NewMyDB(dirName string) (*MyDB, error) {
	db, err := NewMyDBWithOptions(dirName, blockSize, bufferSize)
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

func (db *MyDB) NewTx() *tx.Transaction {
	return tx.NewTransaction(db.fileManager, db.logManager, db.bufferManager, db.lockTable)
}

func (db *MyDB) MetadataManager() *metadata.Manager {
	return db.metadataManager
}

func (db *MyDB) FileManager() *file.Manager {
	return db.fileManager
}

func (db *MyDB) LogManager() *log.Manager {
	return db.logManager
}

func (db *MyDB) BufferManager() *buffer.Manager {
	return db.bufferManager
}

func (db *MyDB) Planner() *plan.Planner {
	return db.planner
}

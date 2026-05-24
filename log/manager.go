package log

import (
	"fmt"
	"godb/file"
	"sync"
)

// Manager manages the log file. It provides methods to append log records and to iterate over them.
// The log file contains a series of log records, each of which is a sequence of bytes. The log records are written
// backwards in the file.
// The log file is processed in blocks, and the log records are written to the most recently allocated block.
// When a block is full, a new block is allocated and used.
// The log manager is responsible for managing the log records in the log file.
// The log manager is thread-safe.
type Manager struct {
	fm           *file.Manager
	logFile      string
	logPage      *file.Page
	currentBlock *file.BlockId
	latestLSN    int
	lastSavedLSN int
	mu           sync.Mutex
}

func NewLogManager(fm *file.Manager, logFile string) (*Manager, error) {
	//Create a new empty page
	logPage := file.NewPageOfSize(fm.BlockSize())

	logSize, err := fm.Length(logFile)
	if err != nil {
		return nil, fmt.Errorf("failed to get log file lenght : %v", err)
	}

	var currentBlock *file.BlockId
	if logSize == 0 {
		//if log file is empty, append a new empty block to it.
		currentBlock, err = appendNewBlock(fm, logFile, logPage)
		if err != nil {
			return nil, fmt.Errorf("failed to append a new block: %v", err)
		}

	} else {
		//If log file is not empty, read the last block into the page
		currentBlock = &file.BlockId{File: logFile, BlockNumber: logSize - 1}
		if err := fm.Read(currentBlock, logPage); err != nil {
			return nil, fmt.Errorf("failed to read log page: %v", err)
		}
	}
	return &Manager{
		fm:           fm,
		logFile:      logFile,
		logPage:      logPage,
		currentBlock: currentBlock,
		latestLSN:    0,
	}, nil

}

func (m *Manager) Flush(lsn int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if lsn >= m.lastSavedLSN {
		return m.flush()
	}
	return nil
}

func (m *Manager) Iterator() (*Iterator, error) {
	if err := m.flush(); err != nil {
		return nil, fmt.Errorf("failed to flush log: %v", err)
	}
	return NewIterator(m.fm, m.currentBlock)
}

// The beginning of the buffer contains the location of the last-written record (the "boundary").
// Storing the records backwards makes it easy to read them in reverse order.
// Returns the LSN of the final value.
func (m *Manager) Append(logRecord []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	//Get the current boundary
	boundary := int(m.logPage.GetInt(0))

	recordSize := len(logRecord)
	bytesNeeded := recordSize + 4 // 4 bytes for the integer storing the record size
	if boundary-bytesNeeded < 4 {
		if err := m.flush(); err != nil {
			return 0, fmt.Errorf("failed to flush log: %v", err)
		}

		var err error
		m.currentBlock, err = appendNewBlock(m.fm, m.logFile, m.logPage)
		if err != nil {
			return 0, fmt.Errorf("failed to append new block: %v", err)
		}

		boundary = int(m.logPage.GetInt(0))
	}

	recordPosition := boundary - bytesNeeded

	m.logPage.SetBytes(recordPosition, logRecord)

	m.logPage.SetInt(0, recordPosition)

	m.latestLSN++
	return m.latestLSN, nil
}

func appendNewBlock(fm *file.Manager, logFile string, logPage *file.Page) (*file.BlockId, error) {
	block, err := fm.Append(logFile)
	if err != nil {
		return nil, fmt.Errorf("failed to append new block: %v", err)
	}
	logPage.SetInt(0, fm.BlockSize())

	if err := fm.Write(block, logPage); err != nil {
		return nil, fmt.Errorf("failed to write new block: %v", err)
	}
	return block, nil
}

// flush writes the buffer to the log file. This method is not thread-safe.
func (m *Manager) flush() error {
	if err := m.fm.Write(m.currentBlock, m.logPage); err != nil {
		return fmt.Errorf("failed to write log page:%v", err)
	}
	m.lastSavedLSN = m.latestLSN
	return nil
}

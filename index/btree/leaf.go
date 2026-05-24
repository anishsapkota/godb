package btree

import (
	"godb/file"
	"godb/record"
	"godb/tx"
)

// leafSplit is returned by insert when the leaf page splits.
type leafSplit struct {
	key      any
	blockNum int
}

type btreeLeaf struct {
	tx          *tx.Transaction
	layout      *record.Layout
	searchKey   any
	page        *btreePage
	currentSlot int
	fileName    string
}

func newBTreeLeaf(tx *tx.Transaction, block *file.BlockId, layout *record.Layout, searchKey any) (*btreeLeaf, error) {
	page, err := newBTreePage(tx, block, layout)
	if err != nil {
		return nil, err
	}
	slot, err := page.findLeafSlot(searchKey)
	if err != nil {
		page.close()
		return nil, err
	}
	return &btreeLeaf{
		tx:          tx,
		layout:      layout,
		searchKey:   searchKey,
		page:        page,
		currentSlot: slot,
		fileName:    block.Filename(),
	}, nil
}

func (l *btreeLeaf) close() {
	if l.page != nil {
		l.page.close()
		l.page = nil
	}
}

// next advances to the next entry matching searchKey, following sibling chain as needed.
func (l *btreeLeaf) next() (bool, error) {
	l.currentSlot++
	for {
		n, err := l.page.getNumRecs()
		if err != nil {
			return false, err
		}
		if l.currentSlot < n {
			val, err := l.page.getDataVal(l.currentSlot)
			if err != nil {
				return false, err
			}
			cmp := compareVals(val, l.searchKey)
			if cmp == 0 {
				return true, nil
			}
			if cmp > 0 {
				return false, nil
			}
			l.currentSlot++
			continue
		}
		// exhausted this page — try sibling
		sibling, err := l.page.getExtra1()
		if err != nil {
			return false, err
		}
		if sibling < 0 {
			return false, nil
		}
		l.page.close()
		sibBlk := file.NewBlockId(l.fileName, sibling)
		l.page, err = newBTreePage(l.tx, sibBlk, l.layout)
		if err != nil {
			return false, err
		}
		l.currentSlot = 0
	}
}

func (l *btreeLeaf) getDataRID() (*record.ID, error) {
	return l.page.getDataRID(l.currentSlot)
}

func (l *btreeLeaf) deleteCurrent() error {
	return l.page.delete(l.currentSlot)
}

// insert inserts (key, rid) into the leaf in sorted order.
// Returns a leafSplit if the page was full and had to split, or nil otherwise.
func (l *btreeLeaf) insert(key any, rid *record.ID) (*leafSplit, error) {
	// find insertion slot: first position where key[slot] >= key
	n, err := l.page.getNumRecs()
	if err != nil {
		return nil, err
	}
	slot := 0
	for slot < n {
		val, err := l.page.getDataVal(slot)
		if err != nil {
			return nil, err
		}
		if compareVals(val, key) >= 0 {
			break
		}
		slot++
	}

	full, err := l.page.isFull()
	if err != nil {
		return nil, err
	}

	if full {
		// split at midpoint
		splitPos := n / 2
		sibling, err := l.page.getExtra1()
		if err != nil {
			return nil, err
		}
		newBlkNum, err := l.page.split(l.fileName, splitPos, sibling)
		if err != nil {
			return nil, err
		}
		// current page now points to new page as next sibling
		if err := l.page.setExtra1(newBlkNum); err != nil {
			return nil, err
		}

		// get first key of new page to return as split key
		newBlk := file.NewBlockId(l.fileName, newBlkNum)
		newPage, err := newBTreePage(l.tx, newBlk, l.layout)
		if err != nil {
			return nil, err
		}
		splitKey, err := newPage.getDataVal(0)
		newPage.close()
		if err != nil {
			return nil, err
		}

		// insert into the appropriate half
		if compareVals(key, splitKey) < 0 {
			if err := l.doInsert(slot, key, rid); err != nil {
				return nil, err
			}
		} else {
			// insert into new (right) page
			l.page.close()
			l.page, err = newBTreePage(l.tx, newBlk, l.layout)
			if err != nil {
				return nil, err
			}
			nn, _ := l.page.getNumRecs()
			insertSlot := 0
			for insertSlot < nn {
				val, _ := l.page.getDataVal(insertSlot)
				if compareVals(val, key) >= 0 {
					break
				}
				insertSlot++
			}
			if err := l.doInsert(insertSlot, key, rid); err != nil {
				return nil, err
			}
		}

		return &leafSplit{key: splitKey, blockNum: newBlkNum}, nil
	}

	return nil, l.doInsert(slot, key, rid)
}

func (l *btreeLeaf) doInsert(slot int, key any, rid *record.ID) error {
	if err := l.page.insert(slot); err != nil {
		return err
	}
	if err := l.page.setDataVal(slot, key); err != nil {
		return err
	}
	return l.page.setDataRID(slot, rid)
}

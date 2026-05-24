package btree

import (
	"godb/file"
	"godb/record"
	"godb/tx"
)

// dirEntry is a (key, childBlock) pair propagated up on a directory split.
type dirEntry struct {
	key      any
	blockNum int
}

type btreeDir struct {
	tx       *tx.Transaction
	layout   *record.Layout
	page     *btreePage
	fileName string
}

func newBTreeDir(tx *tx.Transaction, block *file.BlockId, layout *record.Layout) (*btreeDir, error) {
	page, err := newBTreePage(tx, block, layout)
	if err != nil {
		return nil, err
	}
	return &btreeDir{
		tx:       tx,
		layout:   layout,
		page:     page,
		fileName: block.Filename(),
	}, nil
}

func (d *btreeDir) close() {
	if d.page != nil {
		d.page.close()
		d.page = nil
	}
}

// search descends from this directory page to find the leaf block for searchKey.
// extra1 of dir pages holds the level (0 = points directly to leaf blocks).
func (d *btreeDir) search(searchKey any) (int, error) {
	childBlk, err := d.findChildBlock(searchKey)
	if err != nil {
		return 0, err
	}
	level, err := d.page.getExtra1()
	if err != nil {
		return 0, err
	}
	if level == 0 {
		return childBlk, nil
	}
	childBlock := file.NewBlockId(d.fileName, childBlk)
	child, err := newBTreeDir(d.tx, childBlock, d.layout)
	if err != nil {
		return 0, err
	}
	defer child.close()
	return child.search(searchKey)
}

// insert inserts (splitKey, splitBlock) arising from a leaf split into this directory.
// Recursively descends to the correct level, splitting dir pages as needed.
// Returns a dirEntry if this dir page also split (propagate to parent), or nil otherwise.
func (d *btreeDir) insert(splitKey any, splitBlock int) (*dirEntry, error) {
	level, err := d.page.getExtra1()
	if err != nil {
		return nil, err
	}
	if level == 0 {
		return d.insertEntry(splitKey, splitBlock)
	}
	// recurse into child
	childBlk, err := d.findChildBlock(splitKey)
	if err != nil {
		return nil, err
	}
	child, err := newBTreeDir(d.tx, file.NewBlockId(d.fileName, childBlk), d.layout)
	if err != nil {
		return nil, err
	}
	childSplit, err := child.insert(splitKey, splitBlock)
	child.close()
	if err != nil {
		return nil, err
	}
	if childSplit != nil {
		return d.insertEntry(childSplit.key, childSplit.blockNum)
	}
	return nil, nil
}

// makeNewRoot is called when the root (block 0) itself splits.
// e is the entry pushed up from the root split.
// Copies current root content to a new block, then resets root as a new level.
func (d *btreeDir) makeNewRoot(e *dirEntry) error {
	// save current root state
	leftmost, err := d.page.getExtra2()
	if err != nil {
		return err
	}
	level, err := d.page.getExtra1()
	if err != nil {
		return err
	}
	n, err := d.page.getNumRecs()
	if err != nil {
		return err
	}

	// create a new block to hold the old root's contents
	newBlock, err := d.tx.Append(d.fileName)
	if err != nil {
		return err
	}
	newPage, err := newBTreePage(d.tx, newBlock, d.layout)
	if err != nil {
		return err
	}
	if err := newPage.format(); err != nil {
		newPage.close()
		return err
	}
	if err := newPage.setExtra1(level); err != nil {
		newPage.close()
		return err
	}
	if err := newPage.setExtra2(leftmost); err != nil {
		newPage.close()
		return err
	}
	// copy all entries from old root to new block
	for i := 0; i < n; i++ {
		if err := newPage.insert(i); err != nil {
			newPage.close()
			return err
		}
		if err := d.page.transferTo(i, newPage, i); err != nil {
			newPage.close()
			return err
		}
	}
	newPage.close()

	// reset root: clear entries, bump level, set leftmost = new block
	for {
		nn, _ := d.page.getNumRecs()
		if nn == 0 {
			break
		}
		if err := d.page.delete(0); err != nil {
			return err
		}
	}
	if err := d.page.setExtra1(level + 1); err != nil {
		return err
	}
	if err := d.page.setExtra2(newBlock.Number()); err != nil {
		return err
	}

	// insert the promoted split entry into the new root
	if err := d.page.insert(0); err != nil {
		return err
	}
	if err := d.page.setDataVal(0, e.key); err != nil {
		return err
	}
	return d.page.setBlockField(0, e.blockNum)
}

// findChildBlock returns the child block number for the given searchKey.
func (d *btreeDir) findChildBlock(searchKey any) (int, error) {
	slot, err := d.page.findDirSlot(searchKey)
	if err != nil {
		return 0, err
	}
	if slot < 0 {
		return d.page.getExtra2() // leftmost child
	}
	return d.page.getBlockField(slot)
}

// insertEntry inserts (key, childBlk) into this dir page, splitting if full.
func (d *btreeDir) insertEntry(key any, childBlk int) (*dirEntry, error) {
	n, err := d.page.getNumRecs()
	if err != nil {
		return nil, err
	}
	// find insert position
	slot := 0
	for slot < n {
		val, err := d.page.getDataVal(slot)
		if err != nil {
			return nil, err
		}
		if compareVals(val, key) > 0 {
			break
		}
		slot++
	}

	full, err := d.page.isFull()
	if err != nil {
		return nil, err
	}
	if full {
		level, err := d.page.getExtra1()
		if err != nil {
			return nil, err
		}
		splitPos := n / 2
		newBlkNum, err := d.page.split(d.fileName, splitPos, level)
		if err != nil {
			return nil, err
		}

		// get promoted key: first key of new page, which gets pushed up (removed from new page)
		newBlk := file.NewBlockId(d.fileName, newBlkNum)
		newPage, err := newBTreePage(d.tx, newBlk, d.layout)
		if err != nil {
			return nil, err
		}
		splitKey, err := newPage.getDataVal(0)
		if err != nil {
			newPage.close()
			return nil, err
		}
		// the new page's leftmost child = the child pointer of the promoted key
		promotedChild, err := newPage.getBlockField(0)
		if err != nil {
			newPage.close()
			return nil, err
		}
		if err := newPage.delete(0); err != nil {
			newPage.close()
			return nil, err
		}
		if err := newPage.setExtra2(promotedChild); err != nil {
			newPage.close()
			return nil, err
		}
		newPage.close()

		// insert into the appropriate page
		if compareVals(key, splitKey) < 0 {
			if err := d.doInsert(slot, key, childBlk); err != nil {
				return nil, err
			}
		} else {
			np, err := newBTreePage(d.tx, newBlk, d.layout)
			if err != nil {
				return nil, err
			}
			nn, _ := np.getNumRecs()
			insertSlot := 0
			for insertSlot < nn {
				val, _ := np.getDataVal(insertSlot)
				if compareVals(val, key) > 0 {
					break
				}
				insertSlot++
			}
			if err := np.insert(insertSlot); err != nil {
				np.close()
				return nil, err
			}
			if err := np.setDataVal(insertSlot, key); err != nil {
				np.close()
				return nil, err
			}
			if err := np.setBlockField(insertSlot, childBlk); err != nil {
				np.close()
				return nil, err
			}
			np.close()
		}

		return &dirEntry{key: splitKey, blockNum: newBlkNum}, nil
	}

	return nil, d.doInsert(slot, key, childBlk)
}

func (d *btreeDir) doInsert(slot int, key any, childBlk int) error {
	if err := d.page.insert(slot); err != nil {
		return err
	}
	if err := d.page.setDataVal(slot, key); err != nil {
		return err
	}
	return d.page.setBlockField(slot, childBlk)
}

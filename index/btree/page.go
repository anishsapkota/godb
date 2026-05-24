package btree

import (
	"fmt"
	"godb/file"
	"godb/index"
	"godb/record"
	"godb/tx"
	"godb/utils"
	"strings"
	"time"
)

// Page header layout (3 ints = 12 bytes):
//   [0 .. IntSize-1]         : numRecs
//   [IntSize .. 2*IntSize-1] : extra1 — leaf: next sibling block (-1=none), dir: level
//   [2*IntSize .. 3*IntSize-1]: extra2 — leaf: unused (-1), dir: leftmost child block
const headerSize = 3 * utils.IntSize

type btreePage struct {
	tx     *tx.Transaction
	block  *file.BlockId
	layout *record.Layout
}

func newBTreePage(tx *tx.Transaction, block *file.BlockId, layout *record.Layout) (*btreePage, error) {
	if err := tx.Pin(block); err != nil {
		return nil, err
	}
	return &btreePage{tx: tx, block: block, layout: layout}, nil
}

func (p *btreePage) close() {
	if p.block != nil {
		p.tx.Unpin(p.block)
		p.block = nil
	}
}

func (p *btreePage) entryOffset(slot int, fieldName string) int {
	return headerSize + slot*p.layout.SlotSize() + p.layout.Offset(fieldName)
}

func (p *btreePage) getNumRecs() (int, error) {
	return p.tx.GetInt(p.block, 0)
}

func (p *btreePage) setNumRecs(n int) error {
	return p.tx.SetInt(p.block, 0, n, true)
}

// getExtra1: leaf=next sibling block, dir=level
func (p *btreePage) getExtra1() (int, error) {
	return p.tx.GetInt(p.block, utils.IntSize)
}

func (p *btreePage) setExtra1(val int) error {
	return p.tx.SetInt(p.block, utils.IntSize, val, true)
}

// getExtra2: leaf=unused, dir=leftmost child block
func (p *btreePage) getExtra2() (int, error) {
	return p.tx.GetInt(p.block, 2*utils.IntSize)
}

func (p *btreePage) setExtra2(val int) error {
	return p.tx.SetInt(p.block, 2*utils.IntSize, val, true)
}

func (p *btreePage) format() error {
	if err := p.setNumRecs(0); err != nil {
		return err
	}
	if err := p.setExtra1(-1); err != nil {
		return err
	}
	return p.setExtra2(-1)
}

func (p *btreePage) getDataVal(slot int) (any, error) {
	off := p.entryOffset(slot, index.DataValueField)
	switch p.layout.Schema().Type(index.DataValueField) {
	case record.Integer:
		return p.tx.GetInt(p.block, off)
	case record.Long:
		return p.tx.GetLong(p.block, off)
	case record.Short:
		return p.tx.GetShort(p.block, off)
	case record.Boolean:
		return p.tx.GetBool(p.block, off)
	case record.Varchar:
		return p.tx.GetString(p.block, off)
	case record.Date:
		return p.tx.GetDate(p.block, off)
	}
	return nil, fmt.Errorf("btree: unknown field type for data_value")
}

func (p *btreePage) setDataVal(slot int, val any) error {
	off := p.entryOffset(slot, index.DataValueField)
	switch v := val.(type) {
	case int:
		return p.tx.SetInt(p.block, off, v, true)
	case int64:
		return p.tx.SetLong(p.block, off, v, true)
	case int16:
		return p.tx.SetShort(p.block, off, v, true)
	case bool:
		return p.tx.SetBool(p.block, off, v, true)
	case string:
		return p.tx.SetString(p.block, off, v, true)
	case time.Time:
		return p.tx.SetDate(p.block, off, v, true)
	}
	return fmt.Errorf("btree: unsupported value type %T", val)
}

func (p *btreePage) getBlockField(slot int) (int, error) {
	return p.tx.GetInt(p.block, p.entryOffset(slot, index.BlockField))
}

func (p *btreePage) setBlockField(slot int, blkNum int) error {
	return p.tx.SetInt(p.block, p.entryOffset(slot, index.BlockField), blkNum, true)
}

func (p *btreePage) getDataRID(slot int) (*record.ID, error) {
	blk, err := p.tx.GetInt(p.block, p.entryOffset(slot, index.BlockField))
	if err != nil {
		return nil, err
	}
	id, err := p.tx.GetInt(p.block, p.entryOffset(slot, index.IDField))
	if err != nil {
		return nil, err
	}
	return record.NewID(blk, id), nil
}

func (p *btreePage) setDataRID(slot int, rid *record.ID) error {
	if err := p.tx.SetInt(p.block, p.entryOffset(slot, index.BlockField), rid.BlockNumber(), true); err != nil {
		return err
	}
	return p.tx.SetInt(p.block, p.entryOffset(slot, index.IDField), rid.Slot(), true)
}

func (p *btreePage) isFull() (bool, error) {
	n, err := p.getNumRecs()
	if err != nil {
		return false, err
	}
	return headerSize+(n+1)*p.layout.SlotSize() > p.tx.BlockSize(), nil
}

// insert shifts entries [slot..numRecs-1] right, increments numRecs. Does NOT set field values.
func (p *btreePage) insert(slot int) error {
	n, err := p.getNumRecs()
	if err != nil {
		return err
	}
	for i := n; i > slot; i-- {
		if err := p.copyRecord(i-1, i); err != nil {
			return err
		}
	}
	return p.setNumRecs(n + 1)
}

// delete shifts entries [slot+1..numRecs-1] left, decrements numRecs.
func (p *btreePage) delete(slot int) error {
	n, err := p.getNumRecs()
	if err != nil {
		return err
	}
	for i := slot + 1; i < n; i++ {
		if err := p.copyRecord(i, i-1); err != nil {
			return err
		}
	}
	return p.setNumRecs(n - 1)
}

func (p *btreePage) copyRecord(src, dst int) error {
	return p.transferTo(src, p, dst)
}

func (p *btreePage) transferTo(srcSlot int, dst *btreePage, dstSlot int) error {
	schema := p.layout.Schema()
	for _, field := range schema.Fields() {
		srcOff := p.entryOffset(srcSlot, field)
		dstOff := dst.entryOffset(dstSlot, field)
		switch schema.Type(field) {
		case record.Integer:
			v, err := p.tx.GetInt(p.block, srcOff)
			if err != nil {
				return err
			}
			if err := dst.tx.SetInt(dst.block, dstOff, v, true); err != nil {
				return err
			}
		case record.Long:
			v, err := p.tx.GetLong(p.block, srcOff)
			if err != nil {
				return err
			}
			if err := dst.tx.SetLong(dst.block, dstOff, v, true); err != nil {
				return err
			}
		case record.Short:
			v, err := p.tx.GetShort(p.block, srcOff)
			if err != nil {
				return err
			}
			if err := dst.tx.SetShort(dst.block, dstOff, v, true); err != nil {
				return err
			}
		case record.Boolean:
			v, err := p.tx.GetBool(p.block, srcOff)
			if err != nil {
				return err
			}
			if err := dst.tx.SetBool(dst.block, dstOff, v, true); err != nil {
				return err
			}
		case record.Varchar:
			v, err := p.tx.GetString(p.block, srcOff)
			if err != nil {
				return err
			}
			if err := dst.tx.SetString(dst.block, dstOff, v, true); err != nil {
				return err
			}
		case record.Date:
			v, err := p.tx.GetDate(p.block, srcOff)
			if err != nil {
				return err
			}
			if err := dst.tx.SetDate(dst.block, dstOff, v, true); err != nil {
				return err
			}
		}
	}
	return nil
}

// split creates a new page in fileName, moves entries [splitPos..numRecs-1] to it.
// The new page's extra1 is set to siblingVal.
// Returns the new block number.
func (p *btreePage) split(fileName string, splitPos int, siblingVal int) (int, error) {
	newBlock, err := p.tx.Append(fileName)
	if err != nil {
		return 0, err
	}
	newPage, err := newBTreePage(p.tx, newBlock, p.layout)
	if err != nil {
		return 0, err
	}
	defer newPage.close()

	if err := newPage.format(); err != nil {
		return 0, err
	}
	if err := newPage.setExtra1(siblingVal); err != nil {
		return 0, err
	}

	n, err := p.getNumRecs()
	if err != nil {
		return 0, err
	}
	transferSlot := 0
	for splitPos < n {
		if err := newPage.insert(transferSlot); err != nil {
			return 0, err
		}
		if err := p.transferTo(splitPos, newPage, transferSlot); err != nil {
			return 0, err
		}
		if err := p.delete(splitPos); err != nil {
			return 0, err
		}
		n--
		transferSlot++
	}
	return newBlock.Number(), nil
}

// findDirSlot returns the last slot i where key[i] <= searchKey, or -1 if none.
// Used for directory traversal: follow child at slot i (or leftmost if -1).
func (p *btreePage) findDirSlot(searchKey any) (int, error) {
	n, err := p.getNumRecs()
	if err != nil {
		return 0, err
	}
	slot := 0
	for slot < n {
		val, err := p.getDataVal(slot)
		if err != nil {
			return 0, err
		}
		if compareVals(val, searchKey) > 0 {
			break
		}
		slot++
	}
	return slot - 1, nil
}

// findLeafSlot returns the last slot i where key[i] < searchKey, or -1 if none.
// Used to position before the first entry with key >= searchKey.
func (p *btreePage) findLeafSlot(searchKey any) (int, error) {
	n, err := p.getNumRecs()
	if err != nil {
		return 0, err
	}
	slot := 0
	for slot < n {
		val, err := p.getDataVal(slot)
		if err != nil {
			return 0, err
		}
		if compareVals(val, searchKey) >= 0 {
			break
		}
		slot++
	}
	return slot - 1, nil
}

func compareVals(a, b any) int {
	switch av := a.(type) {
	case int:
		bv := b.(int)
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
		return 0
	case int64:
		bv := b.(int64)
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
		return 0
	case int16:
		bv := b.(int16)
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
		return 0
	case bool:
		bv := b.(bool)
		if av == bv {
			return 0
		}
		if !av {
			return -1
		}
		return 1
	case string:
		return strings.Compare(av, b.(string))
	case time.Time:
		bv := b.(time.Time)
		if av.Before(bv) {
			return -1
		}
		if av.After(bv) {
			return 1
		}
		return 0
	}
	panic(fmt.Sprintf("btree: compareVals unsupported type %T", a))
}

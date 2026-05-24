package btree

import (
	"godb/file"
	"godb/index"
	"godb/record"
	"godb/tx"
)

var _ index.Index = (*Index)(nil)

// Index implements a B+ tree index.
//
// Storage:
//   <indexName>.leaf — leaf pages, linked via sibling chain
//   <indexName>.dir  — directory (internal) pages; block 0 is always the root
//
// Directory page extra1 = level (0 = points to leaves), extra2 = leftmost child block.
// Leaf page extra1 = next sibling leaf block (-1 = last leaf), extra2 = unused.
type Index struct {
	tx         *tx.Transaction
	indexName  string
	leafLayout *record.Layout
	dirLayout  *record.Layout
	rootBlock  *file.BlockId
	leaf       *btreeLeaf
}

// NewIndex opens (or initialises) a B+ tree index.
// leafLayout is the layout produced by metadata.IndexInfo.CreateIndexLayout().
func NewIndex(tx *tx.Transaction, indexName string, leafLayout *record.Layout) (*Index, error) {
	leafFile := indexName + ".leaf"
	dirFile := indexName + ".dir"

	// build dir schema: same key type as leaf, but only the block field (no id)
	dirSchema := record.NewSchema()
	dirSchema.Add(index.DataValueField, leafLayout.Schema())
	dirSchema.AddIntField(index.BlockField)
	dirLayout := record.NewLayout(dirSchema)

	leafSize, err := tx.Size(leafFile)
	if err != nil {
		return nil, err
	}
	if leafSize == 0 {
		// first use: create one empty leaf block and one root dir block
		leaf0, err := tx.Append(leafFile)
		if err != nil {
			return nil, err
		}
		lp, err := newBTreePage(tx, leaf0, leafLayout)
		if err != nil {
			return nil, err
		}
		if err := lp.format(); err != nil {
			lp.close()
			return nil, err
		}
		lp.close()

		root, err := tx.Append(dirFile)
		if err != nil {
			return nil, err
		}
		rp, err := newBTreePage(tx, root, dirLayout)
		if err != nil {
			return nil, err
		}
		if err := rp.format(); err != nil {
			rp.close()
			return nil, err
		}
		// level=0 (points to leaves), leftmost child = leaf block 0
		if err := rp.setExtra1(0); err != nil {
			rp.close()
			return nil, err
		}
		if err := rp.setExtra2(0); err != nil {
			rp.close()
			return nil, err
		}
		rp.close()
	}

	return &Index{
		tx:         tx,
		indexName:  indexName,
		leafLayout: leafLayout,
		dirLayout:  dirLayout,
		rootBlock:  file.NewBlockId(dirFile, 0),
	}, nil
}

// SearchCost estimates block accesses to find all entries for one key.
// For a B+ tree of height h, cost ≈ h + matching leaf pages.
// Approximation: log_order(numBlocks) where order ≈ recordsPerBlock.
func SearchCost(numBlocks, recordsPerBlock int) int {
	if recordsPerBlock <= 1 {
		return numBlocks
	}
	cost := 1
	n := numBlocks
	for n > 1 {
		n = n / recordsPerBlock
		cost++
	}
	return cost
}

func (idx *Index) BeforeFirst(searchKey any) error {
	idx.Close()
	dir, err := newBTreeDir(idx.tx, idx.rootBlock, idx.dirLayout)
	if err != nil {
		return err
	}
	leafBlkNum, err := dir.search(searchKey)
	dir.close()
	if err != nil {
		return err
	}
	leafBlk := file.NewBlockId(idx.indexName+".leaf", leafBlkNum)
	leaf, err := newBTreeLeaf(idx.tx, leafBlk, idx.leafLayout, searchKey)
	if err != nil {
		return err
	}
	idx.leaf = leaf
	return nil
}

func (idx *Index) Next() (bool, error) {
	return idx.leaf.next()
}

func (idx *Index) GetDataRecordID() (*record.ID, error) {
	return idx.leaf.getDataRID()
}

func (idx *Index) Insert(dataValue any, dataRecordID *record.ID) error {
	// find the leaf block via directory
	dir, err := newBTreeDir(idx.tx, idx.rootBlock, idx.dirLayout)
	if err != nil {
		return err
	}
	leafBlkNum, err := dir.search(dataValue)
	if err != nil {
		dir.close()
		return err
	}

	leafBlk := file.NewBlockId(idx.indexName+".leaf", leafBlkNum)
	leaf, err := newBTreeLeaf(idx.tx, leafBlk, idx.leafLayout, dataValue)
	if err != nil {
		dir.close()
		return err
	}
	split, err := leaf.insert(dataValue, dataRecordID)
	leaf.close()
	if err != nil {
		dir.close()
		return err
	}

	if split == nil {
		dir.close()
		return nil
	}

	// leaf split: propagate into directory
	rootSplit, err := dir.insert(split.key, split.blockNum)
	if err != nil {
		dir.close()
		return err
	}
	if rootSplit != nil {
		err = dir.makeNewRoot(rootSplit)
	}
	dir.close()
	return err
}

func (idx *Index) Delete(dataValue any, dataRecordID *record.ID) error {
	if err := idx.BeforeFirst(dataValue); err != nil {
		return err
	}
	defer idx.Close()
	for {
		has, err := idx.Next()
		if err != nil {
			return err
		}
		if !has {
			return nil
		}
		rid, err := idx.GetDataRecordID()
		if err != nil {
			return err
		}
		if rid.Equals(dataRecordID) {
			return idx.leaf.deleteCurrent()
		}
	}
}

func (idx *Index) Close() {
	if idx.leaf != nil {
		idx.leaf.close()
		idx.leaf = nil
	}
}

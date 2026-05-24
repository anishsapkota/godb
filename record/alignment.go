package record

import "godb/utils"

const (
	LongAlignment    = 8
	ShortAlignment   = 2
	BooleanAlignment = 1
	DateAlignment    = 8
	VarcharAlignment = 1 // No alignment for strings, packed tightly
)

func alignmentRequirement(fieldType SchemaType) int {
	switch fieldType {
	case Integer:
		return utils.IntSize
	case Long:
		return LongAlignment
	case Short:
		return ShortAlignment
	case Boolean:
		return BooleanAlignment
	case Date:
		return DateAlignment
	case Varchar:
		return VarcharAlignment
	default:
		return 1 // Default to no alignment for unknown types
	}
}

func maxAlignment(fieldAlignments map[string]int) int {
	maxAlign := 1
	for _, align := range fieldAlignments {
		if align > maxAlign {
			maxAlign = align
		}
	}
	return maxAlign
}

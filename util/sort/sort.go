package sort

import (
	"sort"
)

//------------------------------------ byte -------------------------------------------------------

type byteSlice []byte

func (p byteSlice) Len() int           { return len(p) }
func (p byteSlice) Less(i, j int) bool { return p[i] < p[j] }
func (p byteSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func Bytes(a []byte, reverse bool) {
	if reverse {
		sort.Sort(sort.Reverse(byteSlice(a)))
	} else {
		sort.Sort(byteSlice(a))
	}
}

//------------------------------------ int8 -------------------------------------------------------

type int8Slice []int8

func (p int8Slice) Len() int           { return len(p) }
func (p int8Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p int8Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func Int8s(a []int8, reverse bool) {
	if reverse {
		sort.Sort(sort.Reverse(int8Slice(a)))
	} else {
		sort.Sort(int8Slice(a))
	}
}

//------------------------------------ uint8 ------------------------------------------------------

type uint8Slice []uint8

func (p uint8Slice) Len() int           { return len(p) }
func (p uint8Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p uint8Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func UInt8s(a []uint8, reverse bool) {
	if reverse {
		sort.Sort(sort.Reverse(uint8Slice(a)))
	} else {
		sort.Sort(uint8Slice(a))
	}
}

//------------------------------------ int --------------------------------------------------------

func Ints(a []int, reverse bool) {
	if reverse {
		sort.Sort(sort.Reverse(sort.IntSlice(a)))
	} else {
		sort.Ints(a)
	}
}

//------------------------------------ uint -------------------------------------------------------

type uintSlice []uint

func (p uintSlice) Len() int           { return len(p) }
func (p uintSlice) Less(i, j int) bool { return p[i] < p[j] }
func (p uintSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func UInts(a []uint, reverse bool) {
	if reverse {
		sort.Sort(sort.Reverse(uintSlice(a)))
	} else {
		sort.Sort(uintSlice(a))
	}
}

//------------------------------------ int32 ------------------------------------------------------

type int32Slice []int32

func (p int32Slice) Len() int           { return len(p) }
func (p int32Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p int32Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func Int32s(a []int32, reverse bool) {
	if reverse {
		sort.Sort(sort.Reverse(int32Slice(a)))
	} else {
		sort.Sort(int32Slice(a))
	}
}

//------------------------------------ uint32 -----------------------------------------------------

type uint32Slice []uint32

func (p uint32Slice) Len() int           { return len(p) }
func (p uint32Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p uint32Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func UInt32s(a []uint32, reverse bool) {
	if reverse {
		sort.Sort(sort.Reverse(uint32Slice(a)))
	} else {
		sort.Sort(uint32Slice(a))
	}
}

//------------------------------------ int64 ------------------------------------------------------

type int64Slice []int64

func (p int64Slice) Len() int           { return len(p) }
func (p int64Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p int64Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// Sort is a convenience method.
func (p int64Slice) Sort() { sort.Sort(p) }

func Int64s(a []int64, reverse bool) {
	if reverse {
		sort.Sort(sort.Reverse(int64Slice(a)))
	} else {
		sort.Sort(int64Slice(a))
	}
}

//------------------------------------ uint64 -----------------------------------------------------

type uint64Slice []uint64

func (p uint64Slice) Len() int           { return len(p) }
func (p uint64Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p uint64Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func UInt64s(a []uint64, reverse bool) {
	if reverse {
		sort.Sort(sort.Reverse(uint64Slice(a)))
	} else {
		sort.Sort(uint64Slice(a))
	}
}

//------------------------------------ float32 ----------------------------------------------------

type float32Slice []float32

func (p float32Slice) Len() int           { return len(p) }
func (p float32Slice) Less(i, j int) bool { return p[i] < p[j] || isNaN(p[i]) && !isNaN(p[j]) }
func (p float32Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// isNaN is a copy of math.IsNaN to avoid a dependency on the math package.
func isNaN(f float32) bool {
	return f != f
}

func Float32s(a []float32, reverse bool) {
	if reverse {
		sort.Sort(sort.Reverse(float32Slice(a)))
	} else {
		sort.Sort(float32Slice(a))
	}
}

//------------------------------------ float64 ----------------------------------------------------

func Float64s(a []float64, reverse bool) {
	if reverse {
		sort.Sort(sort.Reverse(sort.Float64Slice(a)))
	} else {
		sort.Float64s(a)
	}
}

//------------------------------------ string -----------------------------------------------------

func Strings(a []string, reverse bool) {
	if reverse {
		sort.Sort(sort.Reverse(sort.StringSlice(a)))
	} else {
		sort.Strings(a)
	}
}

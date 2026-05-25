package ir

type UseKind uint8

const (
	Uxxx UseKind = iota
	UPhi
	UIns
	UJmp
)

type Use struct {
	Kind UseKind
	Bid  uint32
	Ins  *Instruction
	Phi  *Phi
}

// Function represents a QBE IR function
type Function struct {
	Name     string
	Exported bool
	Start    *Block
	Blocks   []*Block // RPO or logical order

	Temps     []Temporary
	Constants []Constant

	// Stats
	NTmp uint32
	NCon uint32
	NBlk uint32 // Number of blocks, used for allocating arrays (Dom, etc.)

	RetTy int // Index in types if aggregate, -1 otherwise
	Types []Type

	// Physical callee-saved registers actually used; populated by the RA.
	CalleeSaved []int
}

// Temporary represents metadata for a temporary variable
type Temporary struct {
	Name string
	Cls  Class
	Slot int    // Stack slot if spilled
	Cost uint32 // Spill cost
	NDef uint32
	NUse uint32
	Def  *Instruction
	Uses []Use
}

// Constant represents a constant value
type Constant struct {
	Kind uint8
	Val  uint64 // Bits of the constant
}

func (f *Function) NewTmp(name string, cls Class) Ref {
	id := f.NTmp
	f.NTmp++
	f.Temps = append(f.Temps, Temporary{
		Name: name,
		Cls:  cls,
		Slot: -1,
	})
	return Ref{Kind: RTmp, Val: id}
}

func (f *Function) GetCon(val uint64) Ref {
	// Simple implementation: check if constant exists or create new
	for i, c := range f.Constants {
		if c.Val == val {
			return Ref{Kind: RCon, Val: uint32(i)}
		}
	}
	id := f.NCon
	f.NCon++
	f.Constants = append(f.Constants, Constant{Val: val})
	return Ref{Kind: RCon, Val: id}
}

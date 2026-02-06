package ir

import "fmt"

// RefKind defines the type of reference
type RefKind uint8

const (
	RTmp  RefKind = 0 // Temporary (SSA variable)
	RCon  RefKind = 1 // Constant
	RInt  RefKind = 2 // Immediate integer
	RType RefKind = 3 // Type reference
	RSlot RefKind = 4 // Stack slot
	RCall RefKind = 5 // Call descriptor
	RMem  RefKind = 6 // Memory address
	RReg  RefKind = 7 // Physical register
	RSym  RefKind = 8 // Global symbol
)

// Ref is a packed reference to a value in QBE.
type Ref struct {
	Kind RefKind
	Val  uint32
}

var (
	Undef = Ref{Kind: RCon, Val: 0}
	ConZ  = Ref{Kind: RCon, Val: 1}
)

func (r Ref) IsUndef() bool {
	return r.Kind == RCon && r.Val == 0
}

func (r Ref) IsTmp() bool {
	return r.Kind == RTmp
}

func (r Ref) IsCon() bool {
	return r.Kind == RCon
}

func (r Ref) String() string {
	switch r.Kind {
	case RTmp:
		return fmt.Sprintf("%%%d", r.Val)
	case RCon:
		if r.Val == 0 {
			return "undef"
		}
		return fmt.Sprintf("c%d", r.Val)
	case RInt:
		return fmt.Sprintf("%d", int32(r.Val))
	case RSlot:
		return fmt.Sprintf("s%d", r.Val)
	case RReg:
		return fmt.Sprintf("r%d", r.Val)
	default:
		return fmt.Sprintf("r%d:%d", r.Kind, r.Val)
	}
}

// NewTmp creates a new temporary reference
func NewTmp(id uint32) Ref {
	return Ref{Kind: RTmp, Val: id}
}

// NewCon creates a new constant reference
func NewCon(id uint32) Ref {
	return Ref{Kind: RCon, Val: id}
}

// NewSlot creates a new stack slot reference
func NewSlot(id uint32) Ref {
	return Ref{Kind: RSlot, Val: id}
}

// NewInt creates an immediate integer reference
func NewInt(val int32) Ref {
	return Ref{Kind: RInt, Val: uint32(val)}
}

// PhysicalReg creates a new physical register reference
func PhysicalReg(id int) Ref {
	return Ref{Kind: RReg, Val: uint32(id)}
}

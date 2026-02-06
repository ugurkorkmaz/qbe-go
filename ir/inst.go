package ir

// Opcode represents a QBE IR operation
type Opcode uint16

// Instruction represents a single IR instruction
type Instruction struct {
	Op  Opcode
	Cls Class
	To  Ref
	Arg [2]Ref
}

// Phi node represents an SSA Phi function
type Phi struct {
	To   Ref
	Cls  Class
	Args []Ref
	Blks []*Block
}

// Jump represents the control flow at the end of a block
type Jump struct {
	Type JumpType
	Arg  Ref
}

type JumpType uint8

const (
	Jxxx JumpType = iota
	Jretw
	Jretl
	Jrets
	Jretd
	Jretc
	Jjmp
	Jjnz
)

func (t JumpType) Class() Class {
	switch t {
	case Jretw:
		return Kw
	case Jretl:
		return Kl
	case Jrets:
		return Ks
	case Jretd:
		return Kd
	default:
		return Kx
	}
}

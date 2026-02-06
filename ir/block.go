package ir

import "github.com/ugurkorkmaz/qbe-go/util"

// Block represents a basic block in the CFG
type Block struct {
	Name string
	Phis []*Phi
	Ins  []Instruction
	Jmp  Jump

	S1   *Block   // Successor 1
	S2   *Block   // Successor 2
	Pred []*Block // Predecessors

	Id    uint32
	Loop  uint32 // Loop depth
	Visit uint32 // Used during graph traversals

	// Dominance information
	Idom  *Block   // Immediate dominator
	Dom   *Block   // Next block in dominator tree
	DLink *Block   // Link for dominator tree
	Fron  []*Block // Dominance frontier

	// Liveness information
	In   util.BitSet
	Out  util.BitSet
	Gen  util.BitSet
	Kill util.BitSet
}

func NewBlock(name string) *Block {
	return &Block{
		Name: name,
	}
}

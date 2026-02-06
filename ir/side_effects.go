package ir

// HasSideEffects returns true if the instruction must not be removed
// even if its result is not used.
func (ins Instruction) HasSideEffects() bool {
	switch ins.Op {
	case Ostoreb, Ostoreh, Ostorew, Ostorel, Ostores, Ostored:
		return true
	case Ocall:
		return true
	case Ovastart, Ovaarg:
		return true
	// Allocations change stack state, so they are usually kept
	// unless a more advanced escape analysis removes them.
	case Oalloc4, Oalloc8, Oalloc16:
		return true
	default:
		return false
	}
}

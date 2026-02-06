package ir

// Class represents the type class of a value (Width/Type)
type Class int8

const (
	Kx Class = -1 // No class or Top class
	Kw Class = 0  // 32-bit word
	Kl Class = 1  // 64-bit long
	Ks Class = 2  // 32-bit single-precision float
	Kd Class = 3  // 64-bit double-precision float
)

func (c Class) String() string {
	switch c {
	case Kw:
		return "w"
	case Kl:
		return "l"
	case Ks:
		return "s"
	case Kd:
		return "d"
	default:
		return "x"
	}
}

// IsWide returns true if the class is 64-bit (Kl or Kd)
func (c Class) IsWide() bool {
	return uint(c)&1 != 0
}

// IsFloat returns true if the class is a floating point (Ks or Kd)
func (c Class) IsFloat() bool {
	return c == Ks || c == Kd
}

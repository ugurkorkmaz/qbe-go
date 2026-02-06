package ir

// Type represents a QBE aggregate type structure
type Type struct {
	Name   string
	Size   uint64
	Align  uint
	Fields []Field
	IsDark bool // If true, only size and align are known
}

type Field struct {
	Type FieldKind
	Len  uint64 // Length or index into Types for FTyp
}

type FieldKind uint8

const (
	Fb   FieldKind = iota // byte
	Fh                    // half
	Fw                    // word
	Fl                    // long
	Fs                    // float
	Fd                    // double
	FTyp                  // nested type
	FPad                  // padding
	FEnd                  // end of fields
)

func (f *Function) GetTyp(id uint32) *Type {
	// In a real implementation, Types would be a global or per-module list
	// For now, let's assume we have a way to look them up.
	return nil
}

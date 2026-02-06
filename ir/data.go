package ir

// DataItem represents a single data directive within a data definition.
type DataItem struct {
	Type   string // "b", "h", "w", "l", "s", "d", "z" (zero)
	Label  string // If non-empty, this is a reference to a symbol
	Value  int64  // Numeric value (if Label is empty)
	String string // String content (if Type is "b" or "ascii")
}

// Data represents a global data definition (variable, string, array).
type Data struct {
	Label    string
	Exported bool
	Items    []DataItem
}

// Program represents the entire compilation unit.
type Program struct {
	Functions []*Function
	Data      []*Data
	Globals   []string // Symbol table (ID -> Name)
}

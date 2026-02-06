package ir

import "fmt"

const (
	Oxxx Opcode = iota

	// Arithmetic and Bits
	Oadd
	Osub
	Oneg
	Odiv
	Orem
	Oudiv
	Ourem
	Omul
	Oand
	Oor
	Oxor
	Osar
	Oshr
	Oshl

	// Comparisons
	Oceqw
	Ocnew
	Ocsgew
	Ocsgtw
	Ocslew
	Ocsltw
	Ocugew
	Ocugtw
	Oculew
	Ocultw

	Oceql
	Ocnel
	Ocsgel
	Ocsgtl
	Ocslel
	Ocsltl
	Ocugel
	Ocugtl
	Oculel
	Ocultl

	Oceqs
	Ocges
	Ocgts
	Ocles
	Oclts
	Ocnes
	Ocos
	Ocuos

	Oceqd
	Ocged
	Ocgtd
	Ocled
	Ocltd
	Ocned
	Ocod
	Ocuod

	// Memory
	Ostoreb
	Ostoreh
	Ostorew
	Ostorel
	Ostores
	Ostored

	Oloadsb
	Oloadub
	Oloadsh
	Oloaduh
	Oloadsw
	Oloaduw
	Oload

	// Extensions and Truncations
	Oextsb
	Oextub
	Oextsh
	Oextuh
	Oextsw
	Oextuw

	Oexts
	Otruncd
	Ostosi
	Ostoui
	Odtosi
	Odtoui
	Oswtof
	Ouwtof
	Osltof
	Oultof
	Ocast

	// Stack Allocation
	Oalloc4
	Oalloc8
	Oalloc16

	// Variadic Function Helpers
	Ovaarg
	Ovastart

	Ocopy
	Onop // Internal starts here

	// More internal ops as needed
	Oaddr
	Ocall
	Opar
	Oparc
	Oarg
	Oargc
	Oretc
	Omadd // dst = src1 * src2 + src3
	Omsub // dst = src3 - src1 * src2
)

var opNames = map[Opcode]string{
	Oadd: "add", Osub: "sub", Oneg: "neg", Odiv: "div", Orem: "rem",
	Oudiv: "udiv", Ourem: "urem", Omul: "mul", Oand: "and", Oor: "or",
	Oxor: "xor", Osar: "sar", Oshr: "shr", Oshl: "shl",
	Oceqw: "ceqw", Ocnew: "cnew", Ocsgew: "csgew", Ocsgtw: "csgtw",
	Oceql: "ceql", Ocnel: "cnel", Ocsgel: "csgel", Ocsgtl: "csgtl",
	Ostoreb: "storeb", Ostoreh: "storeh", Ostorew: "storew", Ostorel: "storel",
	Oloadsb: "loadsb", Oloadub: "loadub", Oloadsh: "loadsh", Oloaduh: "loaduh",
	Oloadsw: "loadsw", Oloaduw: "loaduw", Oload: "load",
	Ocopy: "copy", Onop: "nop", Ocall: "call",
	Omadd: "madd", Omsub: "msub",
}

func (o Opcode) String() string {
	if name, ok := opNames[o]; ok {
		return name
	}
	return fmt.Sprintf("op%d", o)
}

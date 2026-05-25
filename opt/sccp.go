package opt

import (
	"github.com/ugurkorkmaz/qbe-go/analysis"
	"github.com/ugurkorkmaz/qbe-go/ir"
)

type LatticeKind int8

const (
	LatTop LatticeKind = iota
	LatCon
	LatBot
)

type LatticeState struct {
	Kind LatticeKind
	Con  ir.Constant
}

func mergeLat(s1, s2 LatticeState) LatticeState {
	if s1.Kind == LatBot || s2.Kind == LatBot {
		return LatticeState{Kind: LatBot}
	}
	if s1.Kind == LatTop {
		return s2
	}
	if s2.Kind == LatTop {
		return s1
	}
	if s1.Con.Val == s2.Con.Val {
		return s1
	}
	return LatticeState{Kind: LatBot}
}

func SCCP(f *ir.Function) {
	analysis.FillUse(f)
	states := make([]LatticeState, f.NTmp)
	for i := range states {
		states[i] = LatticeState{Kind: LatTop}
	}

	flowWL := []*ir.Block{f.Start}
	ssaWL := make(map[uint32]bool)
	blockExec := make([]bool, len(f.Blocks))
	edgeExec := make(map[string]bool)

	for len(flowWL) > 0 || len(ssaWL) > 0 {
		if len(flowWL) > 0 {
			b := flowWL[0]
			flowWL = flowWL[1:]
			if blockExec[b.Id] {
				continue
			}
			blockExec[b.Id] = true

			for _, p := range b.Phis {
				visitPhi(f, p, states, edgeExec, ssaWL)
			}
			for i := range b.Ins {
				visitIns(f, &b.Ins[i], states, ssaWL)
			}
			visitJump(b, states, &flowWL, edgeExec)
		} else if len(ssaWL) > 0 {
			var tid uint32
			for k := range ssaWL {
				tid = k
				delete(ssaWL, k)
				break
			}
			for _, use := range f.Temps[tid].Uses {
				if !blockExec[use.Bid] {
					continue
				}
				switch use.Kind {
				case ir.UPhi:
					visitPhi(f, use.Phi, states, edgeExec, ssaWL)
				case ir.UIns:
					visitIns(f, use.Ins, states, ssaWL)
				case ir.UJmp:
					visitJump(f.Blocks[use.Bid], states, &flowWL, edgeExec)
				}
			}
		}
	}
	transform(f, states, blockExec)
}

func visitPhi(f *ir.Function, p *ir.Phi, states []LatticeState, edgeExec map[string]bool, ssaWL map[uint32]bool) {
	if !p.To.IsTmp() {
		return
	}
	res := LatticeState{Kind: LatTop}
	for _, arg := range p.Args {
		res = mergeLat(res, getLat(f, arg, states))
	}
	if res.Kind != states[p.To.Val].Kind || (res.Kind == LatCon && res.Con.Val != states[p.To.Val].Con.Val) {
		states[p.To.Val] = res
		ssaWL[p.To.Val] = true
	}
}

func getLat(f *ir.Function, r ir.Ref, states []LatticeState) LatticeState {
	if r.Kind == ir.RCon {
		return LatticeState{Kind: LatCon, Con: f.Constants[r.Val]}
	}
	if r.Kind == ir.RInt {
		return LatticeState{Kind: LatCon, Con: ir.Constant{Val: uint64(int32(r.Val))}}
	}
	if r.Kind == ir.RTmp {
		return states[r.Val]
	}
	return LatticeState{Kind: LatBot}
}

func visitIns(f *ir.Function, ins *ir.Instruction, states []LatticeState, ssaWL map[uint32]bool) {
	if !ins.To.IsTmp() {
		return
	}
	s1 := getLat(f, ins.Arg[0], states)
	var s2 LatticeState
	hasArg2 := !ins.Arg[1].IsUndef()
	if hasArg2 {
		s2 = getLat(f, ins.Arg[1], states)
	}

	var res LatticeState
	if s1.Kind == LatBot || (hasArg2 && s2.Kind == LatBot) {
		res = LatticeState{Kind: LatBot}
	} else if s1.Kind == LatTop || (hasArg2 && s2.Kind == LatTop) {
		res = LatticeState{Kind: LatTop}
	} else {
		c, ok := Fold(f, ins.Op, ins.Cls, s1.Con, s2.Con)
		if ok {
			res = LatticeState{Kind: LatCon, Con: c}
		} else {
			res = LatticeState{Kind: LatBot}
		}
	}

	if res.Kind != states[ins.To.Val].Kind || (res.Kind == LatCon && res.Con.Val != states[ins.To.Val].Con.Val) {
		states[ins.To.Val] = res
		ssaWL[ins.To.Val] = true
	}
}

func visitJump(b *ir.Block, states []LatticeState, flowWL *[]*ir.Block, edgeExec map[string]bool) {
	if b.S1 != nil {
		*flowWL = append(*flowWL, b.S1)
	}
	if b.S2 != nil {
		*flowWL = append(*flowWL, b.S2)
	}
}

func transform(f *ir.Function, states []LatticeState, blockExec []bool) {
	for _, b := range f.Blocks {
		if !blockExec[b.Id] {
			b.Ins = nil
			b.Phis = nil
			b.Jmp = ir.Jump{Type: ir.Jxxx}
			continue
		}
		for i := range b.Ins {
			ins := &b.Ins[i]
			for n := 0; n < 3; n++ {
				if ins.Arg[n].IsTmp() {
					s := states[ins.Arg[n].Val]
					if s.Kind == LatCon {
						ins.Arg[n] = f.GetCon(s.Con.Val)
					}
				}
			}
			if ins.To.IsTmp() && states[ins.To.Val].Kind == LatCon {
				ins.Op = ir.Onop
			}
		}
	}
}

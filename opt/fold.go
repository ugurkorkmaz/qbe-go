package opt

import (
	"math"

	"github.com/ugurkorkmaz/qbe-go/ir"
)

// Fold evaluates an operation on constant operands.
func Fold(f *ir.Function, op ir.Opcode, cls ir.Class, cl, cr ir.Constant) (ir.Constant, bool) {
	if cls == ir.Kw || cls == ir.Kl {
		return foldInt(op, cls == ir.Kl, cl, cr)
	}
	return foldFlt(op, cls == ir.Kd, cl, cr)
}

func foldInt(op ir.Opcode, wide bool, cl, cr ir.Constant) (ir.Constant, bool) {
	l := int64(cl.Val)
	r := int64(cr.Val)
	var res uint64

	switch op {
	case ir.Oadd:
		res = uint64(l + r)
	case ir.Osub:
		res = uint64(l - r)
	case ir.Oneg:
		res = uint64(-l)
	case ir.Omul:
		res = uint64(l * r)
	case ir.Odiv, ir.Orem, ir.Oudiv, ir.Ourem:
		if r == 0 {
			return ir.Constant{}, false
		}
		switch op {
		case ir.Odiv:
			res = uint64(l / r)
		case ir.Orem:
			res = uint64(l % r)
		case ir.Oudiv:
			res = uint64(uint64(l) / uint64(r))
		case ir.Ourem:
			res = uint64(uint64(l) % uint64(r))
		}
	case ir.Oand:
		res = uint64(l & r)
	case ir.Oor:
		res = uint64(l | r)
	case ir.Oxor:
		res = uint64(l ^ r)
	case ir.Oshl:
		res = uint64(l << (uint(r) & 63))
	case ir.Oshr:
		res = uint64(uint64(l) >> (uint(r) & 63))
	case ir.Osar:
		res = uint64(l >> (uint(r) & 63))
	case ir.Oceqw, ir.Oceql:
		if l == r {
			res = 1
		} else {
			res = 0
		}
	case ir.Ocnew, ir.Ocnel:
		if l != r {
			res = 1
		} else {
			res = 0
		}
	default:
		return ir.Constant{}, false
	}

	if !wide {
		res = uint64(uint32(res))
	}
	return ir.Constant{Val: res}, true
}

func foldFlt(op ir.Opcode, wide bool, cl, cr ir.Constant) (ir.Constant, bool) {
	if !wide {
		l := math.Float32frombits(uint32(cl.Val))
		r := math.Float32frombits(uint32(cr.Val))
		var res float32
		switch op {
		case ir.Oadd:
			res = l + r
		case ir.Osub:
			res = l - r
		case ir.Omul:
			res = l * r
		case ir.Odiv:
			if r == 0 {
				return ir.Constant{}, false
			}
			res = l / r
		default:
			return ir.Constant{}, false
		}
		return ir.Constant{Val: uint64(math.Float32bits(res))}, true
	} else {
		l := math.Float64frombits(cl.Val)
		r := math.Float64frombits(cr.Val)
		var res float64
		switch op {
		case ir.Oadd:
			res = l + r
		case ir.Osub:
			res = l - r
		case ir.Omul:
			res = l * r
		case ir.Odiv:
			if r == 0 {
				return ir.Constant{}, false
			}
			res = l / r
		default:
			return ir.Constant{}, false
		}
		return ir.Constant{Val: math.Float64bits(res)}, true
	}
}

// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package compiler

import (
	"fmt"

	"firefly-os.dev/tools/ruse/ast"
	"firefly-os.dev/tools/ruse/constant"
	"firefly-os.dev/tools/ruse/ssafir"
	"firefly-os.dev/tools/ruse/types"
)

func (c *compiler) CompileExpression(expr ast.Expression) (*ssafir.Value, error) {
	// Check if the expression has a constant
	// value and if so, resolve it.
	typ := c.info.Types[expr]
	if typ.Value != nil {
		var op ssafir.Op
		switch types.Underlying(typ.Type) {
		case types.Bool:
			op = ssafir.OpConstantBool
		case types.String, types.UntypedString:
			op = ssafir.OpConstantString
		case types.Int8:
			op = ssafir.OpConstantInt8
		case types.Int16:
			op = ssafir.OpConstantInt16
		case types.Int32:
			op = ssafir.OpConstantInt32
		case types.Int64:
			op = ssafir.OpConstantInt64
		case types.Int:
			switch c.arch.RegisterSize {
			case 4:
				op = ssafir.OpConstantInt32
			case 8:
				op = ssafir.OpConstantInt64
			default:
				panic(fmt.Sprintf("invalid architecture: register size %d", c.arch.RegisterSize))
			}
		case types.Uint8:
			op = ssafir.OpConstantUint8
		case types.Uint16:
			op = ssafir.OpConstantUint16
		case types.Uint32:
			op = ssafir.OpConstantUint32
		case types.Uint64:
			op = ssafir.OpConstantUint64
		case types.Uint:
			switch c.arch.RegisterSize {
			case 4:
				op = ssafir.OpConstantUint32
			case 8:
				op = ssafir.OpConstantUint64
			default:
				panic(fmt.Sprintf("invalid architecture: register size %d", c.arch.RegisterSize))
			}
		case types.UntypedInt:
			op = ssafir.OpConstantUntypedInt
		default:
			return nil, fmt.Errorf("%s: failed to compile %s (%T): unsupported constant type %s", c.fset.Position(expr.Pos()), expr.Print(), expr, typ)
		}

		var v *ssafir.Value
		switch types.Underlying(typ.Type) {
		case types.Bool:
			var extra int64
			if constant.BoolVal(typ.Value) {
				extra = 1
			}

			v = c.ValueInt(expr.Pos(), expr.End(), op, typ.Type, extra)
		case types.String, types.UntypedString:
			v = c.ValueExtra(expr.Pos(), expr.End(), op, typ.Type, constant.StringVal(typ.Value))
		case types.Int, types.Int8, types.Int16, types.Int32, types.Int64:
			num, ok := constant.Int64Val(typ.Value)
			if !ok {
				return nil, fmt.Errorf("%s: cannot use %s (%s) as %s value", c.fset.Position(expr.Pos()), expr.Print(), expr, typ)
			}

			v = c.ValueInt(expr.Pos(), expr.End(), op, typ.Type, int64(num))
		case types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64:
			num, ok := constant.Uint64Val(typ.Value)
			if !ok {
				return nil, fmt.Errorf("%s: cannot use %s (%s) as %s value", c.fset.Position(expr.Pos()), expr.Print(), expr, typ)
			}

			v = c.ValueInt(expr.Pos(), expr.End(), op, typ.Type, int64(num))
		case types.UntypedInt:
			v = c.ValueExtra(expr.Pos(), expr.End(), op, typ.Type, typ.Value)
		default:
			return nil, fmt.Errorf("%s: failed to compile %s %s into value: unrecognised underlying type: %v", c.fset.Position(expr.Pos()), expr, expr.Print(), types.Underlying(typ.Type))
		}

		return v, nil
	}

	switch x := expr.(type) {
	// Function call.
	case *ast.List:
		switch op := x.Elements[0].(type) {
		case *ast.Identifier:
			obj := c.info.Uses[op]
			typ := c.info.Types[x.Elements[0]]
			switch obj := obj.(type) {
			case *types.SpecialForm:
				sig := typ.Type.(*types.Signature)
				return c.CompileSpecialForm(x, obj, sig)
			case *types.Function:
				sig := obj.Type().(*types.Signature)
				if obj.Parent() == types.Universe {
					return c.CompileBuiltinFunction(x, obj, sig)
				}

				params := make([]*ssafir.Value, len(x.Elements[1:]))
				for i, elt := range x.Elements[1:] {
					v, err := c.CompileExpression(elt)
					if err != nil {
						return nil, err
					}

					if v == nil {
						panic(fmt.Sprintf("function param %d (%s %s) compiled to a nil value", i, elt, elt.Print()))
					}

					params[i] = v
				}

				v := c.ValueExtra(x.ParenOpen, x.ParenClose+1, ssafir.OpFunctionCall, sig.Result(), obj, params...)

				return v, nil
			case *types.TypeName:
				// Cast.
				v, err := c.CompileExpression(x.Elements[1])
				if err != nil {
					return nil, err
				}

				v = c.ValueExtra(x.ParenOpen, x.ParenClose+1, v.Op, typ.Type, v.Extra, v)

				return v, nil
			default:
				panic(fmt.Sprintf("bad identifier %T", obj))
			}
		case *ast.Qualified:
			ident := op.Y
			obj := c.info.Uses[ident]
			typ := c.info.Types[x.Elements[0]]
			switch obj := obj.(type) {
			case *types.SpecialForm:
				sig := typ.Type.(*types.Signature)
				return c.CompileSpecialForm(x, obj, sig)
			case *types.Function:
				sig := obj.Type().(*types.Signature)
				if obj.Parent() == types.Universe {
					return c.CompileBuiltinFunction(x, obj, sig)
				}

				params := make([]*ssafir.Value, len(x.Elements[1:]))
				for i, elt := range x.Elements[1:] {
					v, err := c.CompileExpression(elt)
					if err != nil {
						return nil, err
					}

					if v == nil {
						panic(fmt.Sprintf("function param %d (%s %s) compiled to a nil value", i, elt, elt.Print()))
					}

					params[i] = v
				}

				v := c.ValueExtra(x.ParenOpen, x.ParenClose+1, ssafir.OpFunctionCall, sig.Result(), obj, params...)

				return v, nil
			default:
				panic(fmt.Sprintf("bad identifier %T", obj))
			}
		}

	// Variable.
	case *ast.Identifier:
		if obj, ok := c.info.Definitions[x].(*types.Variable); ok {
			if v := c.vars[obj]; v != nil {
				return v, nil
			}
		}

		switch obj := c.info.Uses[x].(type) {
		case *types.Constant:
			var op ssafir.Op
			val := obj.Value()
			switch val.Kind() {
			case constant.Integer:
				op = ssafir.OpConstantUntypedInt
			case constant.String:
				op = ssafir.OpConstantString
			default:
				return nil, fmt.Errorf("%s: failed to compile %s (%T): unsupported expression type %s constant", c.fset.Position(expr.Pos()), expr.Print(), expr, val.Kind())
			}

			v := c.ValueExtra(x.Pos(), x.End(), op, obj.Type(), val)
			return v, nil
		case *types.Variable:
			if v := c.vars[obj]; v != nil {
				return v, nil
			}
		default:
			return nil, fmt.Errorf("%s: failed to compile %s (%T): unsupported expression type %T", c.fset.Position(expr.Pos()), expr.Print(), expr, obj)
		}
	case *ast.Qualified:
		ident := x.Y
		if obj, ok := c.info.Definitions[ident].(*types.Variable); ok {
			if v := c.vars[obj]; v != nil {
				return v, nil
			}
		}

		switch obj := c.info.Uses[ident].(type) {
		case *types.Constant:
			var op ssafir.Op
			val := obj.Value()
			switch val.Kind() {
			case constant.Integer:
				op = ssafir.OpConstantUntypedInt
			case constant.String:
				op = ssafir.OpConstantString
			default:
				return nil, fmt.Errorf("%s: failed to compile %s (%T): unsupported expression type %s constant", c.fset.Position(expr.Pos()), expr.Print(), expr, val.Kind())
			}

			v := c.ValueExtra(x.Pos(), x.End(), op, obj.Type(), val)
			return v, nil
		case *types.Variable:
			if v := c.vars[obj]; v != nil {
				return v, nil
			}
		default:
			return nil, fmt.Errorf("%s: failed to compile %s (%T): unsupported expression type %T", c.fset.Position(expr.Pos()), expr.Print(), expr, obj)
		}
	}

	return nil, fmt.Errorf("%s: failed to compile %s (%T): unsupported expression type %s", c.fset.Position(expr.Pos()), expr.Print(), expr, typ)
}

func (c *compiler) CompileBinaryOperation(args []ast.Expression, op ssafir.Op, typ types.Type) (v *ssafir.Value, err error) {
	values := make([]*ssafir.Value, len(args))
	for i, arg := range args {
		values[i], err = c.CompileExpression(arg)
		if err != nil {
			return nil, err
		}
	}

	v = c.Value(args[0].Pos(), args[1].End(), op, typ, values[0], values[1])
	for i := 2; i < len(args); i++ {
		v = c.ContinueValue(v, args[i-1].Pos(), args[i].End(), op, typ, v, values[i])
	}

	return v, nil
}

func (c *compiler) CompileBuiltinFunction(list *ast.List, fun *types.Function, sig *types.Signature) (*ssafir.Value, error) {
	selectWordSizeOp := func(four, eight ssafir.Op) ssafir.Op {
		size := c.sizes.SizeOf(types.Int)
		switch size {
		case 4:
			return four
		case 8:
			return eight
		}

		panic(fmt.Sprintf("%s: failed to compile %s (%T): unsupported size for int: %d", c.fset.Position(list.ParenOpen), list.Print(), fun, size))
	}

	selectWordSizeType := func(four, eight types.Type) types.Type {
		size := c.sizes.SizeOf(types.Int)
		switch size {
		case 4:
			return four
		case 8:
			return eight
		}

		panic(fmt.Sprintf("%s: failed to compile %s (%T): unsupported size for int: %d", c.fset.Position(list.ParenOpen), list.Print(), fun, size))
	}

	if types.CastFunctions[fun.Name()] != nil {
		var op ssafir.Op
		var typ types.Type
		switch fun.Name() {
		case "int->int8":
			typ = types.Int8
			op = selectWordSizeOp(ssafir.OpCastInt32ToInt8, ssafir.OpCastInt64ToInt8)
		case "int->int16":
			typ = types.Int16
			op = selectWordSizeOp(ssafir.OpCastInt32ToInt16, ssafir.OpCastInt64ToInt16)
		case "int->int32":
			typ = types.Int32
			op = selectWordSizeOp(ssafir.OpCopy, ssafir.OpCastInt64ToInt32)
		case "int->int64":
			typ = types.Int64
			op = selectWordSizeOp(ssafir.OpCastInt32ToInt64, ssafir.OpCopy)
		case "int->uint":
			op = ssafir.OpCopy
			typ = selectWordSizeType(types.Uint32, types.Uint64)
		case "int->uint8":
			typ = types.Uint8
			op = selectWordSizeOp(ssafir.OpCastInt32ToUint8, ssafir.OpCastInt64ToUint8)
		case "int->uint16":
			typ = types.Uint16
			op = selectWordSizeOp(ssafir.OpCastInt32ToUint16, ssafir.OpCastInt64ToUint16)
		case "int->uint32":
			typ = types.Uint32
			op = selectWordSizeOp(ssafir.OpCastInt32ToUint32, ssafir.OpCastInt64ToUint32)
		case "int->uint64":
			typ = types.Uint64
			op = selectWordSizeOp(ssafir.OpCastInt32ToUint64, ssafir.OpCastInt64ToUint64)
		case "int->uintptr":
			op = ssafir.OpCopy
			typ = selectWordSizeType(types.Uint32, types.Uint64)
		case "int8->int":
			typ = types.Int8
			op = selectWordSizeOp(ssafir.OpCastInt8ToInt32, ssafir.OpCastInt8ToInt64)
		case "int8->int16":
			op = ssafir.OpCastInt8ToInt16
			typ = types.Int16
		case "int8->int32":
			op = ssafir.OpCastInt8ToInt32
			typ = types.Int32
		case "int8->int64":
			op = ssafir.OpCastInt8ToInt64
			typ = types.Int64
		case "int8->uint":
			typ = types.Int8
			op = selectWordSizeOp(ssafir.OpCastInt8ToUint32, ssafir.OpCastInt8ToUint64)
		case "int8->uint8":
			op = ssafir.OpCastInt8ToUint8
			typ = types.Uint8
		case "int8->uint16":
			op = ssafir.OpCastInt8ToUint16
			typ = types.Uint16
		case "int8->uint32":
			op = ssafir.OpCastInt8ToUint32
			typ = types.Uint32
		case "int8->uint64":
			op = ssafir.OpCastInt8ToUint64
			typ = types.Uint64
		case "int8->uintptr":
			typ = types.Int8
			op = selectWordSizeOp(ssafir.OpCastInt8ToUint32, ssafir.OpCastInt8ToUint64)
		case "int16->int":
			typ = types.Int16
			op = selectWordSizeOp(ssafir.OpCastInt16ToInt32, ssafir.OpCastInt16ToInt64)
		case "int16->int8":
			op = ssafir.OpCastInt16ToInt8
			typ = types.Int8
		case "int16->int32":
			op = ssafir.OpCastInt16ToInt32
			typ = types.Int32
		case "int16->int64":
			op = ssafir.OpCastInt16ToInt64
			typ = types.Int64
		case "int16->uint":
			typ = types.Int16
			op = selectWordSizeOp(ssafir.OpCastInt16ToUint32, ssafir.OpCastInt16ToUint64)
		case "int16->uint8":
			op = ssafir.OpCastInt16ToUint8
			typ = types.Uint8
		case "int16->uint16":
			op = ssafir.OpCastInt16ToUint16
			typ = types.Uint16
		case "int16->uint32":
			op = ssafir.OpCastInt16ToUint32
			typ = types.Uint32
		case "int16->uint64":
			op = ssafir.OpCastInt16ToUint64
			typ = types.Uint64
		case "int16->uintptr":
			typ = types.Int16
			op = selectWordSizeOp(ssafir.OpCastInt16ToUint32, ssafir.OpCastInt16ToUint64)
		case "int32->int":
			typ = types.Int32
			op = selectWordSizeOp(ssafir.OpCopy, ssafir.OpCastInt32ToInt64)
		case "int32->int8":
			op = ssafir.OpCastInt32ToInt8
			typ = types.Int8
		case "int32->int16":
			op = ssafir.OpCastInt32ToInt16
			typ = types.Int16
		case "int32->int64":
			op = ssafir.OpCastInt32ToInt64
			typ = types.Int64
		case "int32->uint":
			typ = types.Int32
			op = selectWordSizeOp(ssafir.OpCopy, ssafir.OpCastInt32ToUint64)
		case "int32->uint8":
			op = ssafir.OpCastInt32ToUint8
			typ = types.Uint8
		case "int32->uint16":
			op = ssafir.OpCastInt32ToUint16
			typ = types.Uint16
		case "int32->uint32":
			op = ssafir.OpCastInt32ToUint32
			typ = types.Uint32
		case "int32->uint64":
			op = ssafir.OpCastInt32ToUint64
			typ = types.Uint64
		case "int32->uintptr":
			typ = types.Int32
			op = selectWordSizeOp(ssafir.OpCopy, ssafir.OpCastInt32ToUint64)
		case "int64->int":
			typ = types.Int64
			op = selectWordSizeOp(ssafir.OpCastInt64ToInt32, ssafir.OpCopy)
		case "int64->int8":
			op = ssafir.OpCastInt64ToInt8
			typ = types.Int8
		case "int64->int16":
			op = ssafir.OpCastInt64ToInt16
			typ = types.Int16
		case "int64->int32":
			op = ssafir.OpCastInt64ToInt32
			typ = types.Int32
		case "int64->uint":
			typ = types.Int64
			op = selectWordSizeOp(ssafir.OpCastInt64ToUint32, ssafir.OpCopy)
		case "int64->uint8":
			op = ssafir.OpCastInt64ToUint8
			typ = types.Uint8
		case "int64->uint16":
			op = ssafir.OpCastInt64ToUint16
			typ = types.Uint16
		case "int64->uint32":
			op = ssafir.OpCastInt64ToUint32
			typ = types.Uint32
		case "int64->uint64":
			op = ssafir.OpCastInt64ToUint64
			typ = types.Uint64
		case "int64->uintptr":
			typ = types.Int64
			op = selectWordSizeOp(ssafir.OpCastInt64ToUint32, ssafir.OpCopy)
		case "uint->int":
			op = ssafir.OpCopy
			typ = selectWordSizeType(types.Int32, types.Int64)
		case "uint->int8":
			typ = types.Int8
			op = selectWordSizeOp(ssafir.OpCastUint32ToInt8, ssafir.OpCastUint64ToInt8)
		case "uint->int16":
			typ = types.Int16
			op = selectWordSizeOp(ssafir.OpCastUint32ToInt16, ssafir.OpCastUint64ToInt16)
		case "uint->int32":
			typ = types.Int32
			op = selectWordSizeOp(ssafir.OpCastUint32ToInt32, ssafir.OpCastUint64ToInt32)
		case "uint->int64":
			typ = types.Int64
			op = selectWordSizeOp(ssafir.OpCastUint32ToInt64, ssafir.OpCastUint64ToInt64)
		case "uint->uint8":
			typ = types.Uint8
			op = selectWordSizeOp(ssafir.OpCastUint32ToUint8, ssafir.OpCastUint64ToUint8)
		case "uint->uint16":
			typ = types.Uint16
			op = selectWordSizeOp(ssafir.OpCastUint32ToUint16, ssafir.OpCastUint64ToUint16)
		case "uint->uint32":
			typ = types.Uint32
			op = selectWordSizeOp(ssafir.OpCopy, ssafir.OpCastUint64ToUint32)
		case "uint->uint64":
			typ = types.Uint64
			op = selectWordSizeOp(ssafir.OpCastUint32ToUint64, ssafir.OpCopy)
		case "uint->uintptr":
			op = ssafir.OpCopy
			typ = selectWordSizeType(types.Uint32, types.Uint64)
		case "uint8->int":
			typ = types.Uint8
			op = selectWordSizeOp(ssafir.OpCastUint8ToInt32, ssafir.OpCastUint8ToInt64)
		case "uint8->int8":
			op = ssafir.OpCastUint8ToInt8
			typ = types.Int8
		case "uint8->int16":
			op = ssafir.OpCastUint8ToInt16
			typ = types.Int16
		case "uint8->int32":
			op = ssafir.OpCastUint8ToInt32
			typ = types.Int32
		case "uint8->int64":
			op = ssafir.OpCastUint8ToInt64
			typ = types.Int64
		case "uint8->uint":
			typ = types.Uint8
			op = selectWordSizeOp(ssafir.OpCastUint8ToUint32, ssafir.OpCastUint8ToUint64)
		case "uint8->uint16":
			op = ssafir.OpCastUint8ToUint16
			typ = types.Uint16
		case "uint8->uint32":
			op = ssafir.OpCastUint8ToUint32
			typ = types.Uint32
		case "uint8->uint64":
			op = ssafir.OpCastUint8ToUint64
			typ = types.Uint64
		case "uint8->uintptr":
			typ = types.Uint8
			op = selectWordSizeOp(ssafir.OpCastUint8ToUint32, ssafir.OpCastUint8ToUint64)
		case "uint16->int":
			typ = types.Uint16
			op = selectWordSizeOp(ssafir.OpCastUint16ToInt32, ssafir.OpCastUint16ToInt64)
		case "uint16->int8":
			op = ssafir.OpCastUint16ToInt8
			typ = types.Int8
		case "uint16->int16":
			op = ssafir.OpCastUint16ToInt16
			typ = types.Int16
		case "uint16->int32":
			op = ssafir.OpCastUint16ToInt32
			typ = types.Int32
		case "uint16->int64":
			op = ssafir.OpCastUint16ToInt64
			typ = types.Int64
		case "uint16->uint":
			typ = types.Uint16
			op = selectWordSizeOp(ssafir.OpCastUint16ToUint32, ssafir.OpCastUint16ToUint64)
		case "uint16->uint8":
			op = ssafir.OpCastUint16ToUint8
			typ = types.Uint8
		case "uint16->uint32":
			op = ssafir.OpCastUint16ToUint32
			typ = types.Uint32
		case "uint16->uint64":
			op = ssafir.OpCastUint16ToUint64
			typ = types.Uint64
		case "uint16->uintptr":
			typ = types.Uint16
			op = selectWordSizeOp(ssafir.OpCastUint16ToUint32, ssafir.OpCastUint16ToUint64)
		case "uint32->int":
			typ = types.Uint32
			op = selectWordSizeOp(ssafir.OpCastUint32ToInt32, ssafir.OpCastUint32ToInt64)
		case "uint32->int8":
			op = ssafir.OpCastUint32ToInt8
			typ = types.Int8
		case "uint32->int16":
			op = ssafir.OpCastUint32ToInt16
			typ = types.Int16
		case "uint32->int32":
			op = ssafir.OpCastUint32ToInt32
			typ = types.Int32
		case "uint32->int64":
			op = ssafir.OpCastUint32ToInt64
			typ = types.Int64
		case "uint32->uint":
			typ = types.Uint32
			op = selectWordSizeOp(ssafir.OpCopy, ssafir.OpCastUint32ToUint64)
		case "uint32->uint8":
			op = ssafir.OpCastUint32ToUint8
			typ = types.Uint8
		case "uint32->uint16":
			op = ssafir.OpCastUint32ToUint16
			typ = types.Uint16
		case "uint32->uint64":
			op = ssafir.OpCastUint32ToUint64
			typ = types.Uint64
		case "uint32->uintptr":
			typ = types.Uint32
			op = selectWordSizeOp(ssafir.OpCopy, ssafir.OpCastUint32ToUint64)
		case "uint64->int":
			typ = types.Uint64
			op = selectWordSizeOp(ssafir.OpCastUint64ToInt32, ssafir.OpCastUint64ToInt64)
		case "uint64->int8":
			op = ssafir.OpCastUint64ToInt8
			typ = types.Int8
		case "uint64->int16":
			op = ssafir.OpCastUint64ToInt16
			typ = types.Int16
		case "uint64->int32":
			op = ssafir.OpCastUint64ToInt32
			typ = types.Int32
		case "uint64->int64":
			op = ssafir.OpCastUint64ToInt64
			typ = types.Int64
		case "uint64->uint":
			typ = types.Uint64
			op = selectWordSizeOp(ssafir.OpCastUint64ToUint32, ssafir.OpCopy)
		case "uint64->uint8":
			op = ssafir.OpCastUint64ToUint8
			typ = types.Uint8
		case "uint64->uint16":
			op = ssafir.OpCastUint64ToUint16
			typ = types.Uint16
		case "uint64->uint32":
			op = ssafir.OpCastUint64ToUint32
			typ = types.Uint32
		case "uint64->uintptr":
			typ = types.Uint64
			op = selectWordSizeOp(ssafir.OpCastUint64ToUint32, ssafir.OpCopy)
		case "uintptr->int":
			op = ssafir.OpCopy
			typ = selectWordSizeType(types.Int32, types.Int64)
		case "uintptr->int8":
			typ = types.Int8
			op = selectWordSizeOp(ssafir.OpCastUint32ToInt8, ssafir.OpCastUint64ToInt8)
		case "uintptr->int16":
			typ = types.Int16
			op = selectWordSizeOp(ssafir.OpCastUint32ToInt16, ssafir.OpCastUint64ToInt16)
		case "uintptr->int32":
			typ = types.Int32
			op = selectWordSizeOp(ssafir.OpCastUint32ToInt32, ssafir.OpCastUint64ToInt32)
		case "uintptr->int64":
			typ = types.Int64
			op = selectWordSizeOp(ssafir.OpCastUint32ToInt64, ssafir.OpCastUint64ToInt64)
		case "uintptr->uint8":
			typ = types.Uint8
			op = selectWordSizeOp(ssafir.OpCastUint32ToUint8, ssafir.OpCastUint64ToUint8)
		case "uintptr->uint16":
			typ = types.Uint16
			op = selectWordSizeOp(ssafir.OpCastUint32ToUint16, ssafir.OpCastUint64ToUint16)
		case "uintptr->uint32":
			typ = types.Uint32
			op = selectWordSizeOp(ssafir.OpCopy, ssafir.OpCastUint64ToUint32)
		case "uintptr->uint64":
			typ = types.Uint64
			op = selectWordSizeOp(ssafir.OpCastUint32ToUint64, ssafir.OpCopy)
		}

		if op != ssafir.OpInvalid && typ != nil {
			value, err := c.CompileExpression(list.Elements[1])
			if err != nil {
				return nil, err
			}

			v := c.Value(list.ParenOpen, list.ParenClose+1, op, typ, value)
			return v, nil
		}
	}

	return nil, fmt.Errorf("%s: failed to compile %s (%T): unsupported builtin function %s", c.fset.Position(list.ParenOpen), list.Print(), sig, fun.Name())
}

func (c *compiler) CompileSpecialForm(list *ast.List, form *types.SpecialForm, sig *types.Signature) (*ssafir.Value, error) {
	// Prepare common data.
	args := list.Elements[1:]
	var op ssafir.Op
	var ok bool

	switch form.ID() {
	case types.SpecialFormLen:
		// Handle calls with a constant value.
		// Constant expressions we've already resolved.
		if typeAndValue, ok := c.info.Types[list]; ok && typeAndValue.Value != nil {
			op := ssafir.OpConstantInt64 // TODO: Pick the constant size based on the architecture.
			v := c.ValueExtra(list.ParenOpen, list.ParenClose+1, op, types.Int, typeAndValue.Value)
			return v, nil
		}

		// Unresolved constant expressions.
		if typeAndValue, ok := c.info.Types[list.Elements[1]]; ok && typeAndValue.Value != nil && typeAndValue.Value.Kind() == constant.String {
			op := ssafir.OpConstantInt64 // TODO: Pick the constant size based on the architecture.
			str := constant.StringVal(typeAndValue.Value)
			v := c.ValueInt(list.ParenOpen, list.ParenClose+1, op, types.Int, int64(len(str)))
			return v, nil
		}

		// TODO: support more types in (len).
		value, err := c.CompileExpression(list.Elements[1])
		if err != nil {
			return nil, err
		}

		v := c.Value(list.ParenOpen, list.ParenClose+1, ssafir.OpStringLen, types.Int, value)
		return v, nil
	case types.SpecialFormLet:
		value, err := c.CompileExpression(list.Elements[2])
		if err != nil {
			return nil, err
		}

		// Find the identifier.
		var ident *ast.Identifier
		var lhs *types.Variable
		switch elt := list.Elements[1].(type) {
		case *ast.Identifier:
			// No need to emit actions for storing
			// to the nil identifier (`_`).
			if elt.Name == "_" {
				return nil, nil
			}

			ident = elt
			lhs = c.info.Definitions[ident].(*types.Variable)
		case *ast.List:
			ident = elt.Elements[0].(*ast.Identifier)
			lhs = c.info.Definitions[ident].(*types.Variable)
		default:
			return nil, fmt.Errorf("unexpected expression type for let left-hand side: %s %s", list.Elements[1], list.Elements[1].Print())
		}

		if isKeyword(ident.Name) {
			return nil, fmt.Errorf("cannot declare variable %q: %s is a keyword", ident.Name, ident.Name)
		}

		v := c.Value(list.ParenOpen, list.ParenClose+1, ssafir.OpCopy, value.Type, value)
		c.vars[lhs] = v

		return v, nil
	case types.SpecialFormAdd:
		args := list.Elements[1:]
		// Unary positive is essentially a no-op.
		if len(args) == 1 {
			return c.CompileExpression(args[0])
		}

		if underlying := types.Underlying(sig.Result()); underlying == types.String {
			op = ssafir.OpAddString
		} else {
			op, ok = c.pickIntegerOp(underlying, ssafir.OpAdd)
		}
	case types.SpecialFormSubtract:
		args := list.Elements[1:]
		if len(args) == 1 {
			value, err := c.CompileExpression(args[0])
			if err != nil {
				return nil, err
			}

			op, ok = c.pickSignedIntegerOp(sig.Result(), ssafir.OpNegate)
			if !ok {
				return nil, fmt.Errorf("%s: failed to compile %s (%T): invalid %s type %s", c.fset.Position(list.ParenOpen), list.Print(), sig, form.ID(), sig.Result())
			}

			v := c.Value(list.ParenOpen, list.ParenClose+1, op, value.Type, value)

			return v, nil
		}

		op, ok = c.pickIntegerOp(sig.Result(), ssafir.OpSubtract)
	case types.SpecialFormMultiply:
		op, ok = c.pickIntegerOp(sig.Result(), ssafir.OpMultiply)
	case types.SpecialFormDivide:
		op, ok = c.pickIntegerOp(sig.Result(), ssafir.OpDivide)
	case types.SpecialFormOr:
		op, ok = c.pickIntegerOp(sig.Result(), ssafir.OpBitwiseOr)
	case types.SpecialFormAnd:
		op, ok = c.pickIntegerOp(sig.Result(), ssafir.OpBitwiseAnd)
	case types.SpecialFormXor:
		op, ok = c.pickIntegerOp(sig.Result(), ssafir.OpBitwiseXor)
	case types.SpecialFormShiftLeft:
		op, ok = c.pickIntegerOp(sig.Result(), ssafir.OpShiftLeft)
	case types.SpecialFormShiftRight:
		op, ok = c.pickIntegerOp(sig.Result(), ssafir.OpShiftRight)
	case types.SpecialFormEqual:
		op, ok = c.pickIntegerOp(sig.Params()[0].Type(), ssafir.OpEqual)
	case types.SpecialFormNotEqual:
		op, ok = c.pickIntegerOp(sig.Params()[0].Type(), ssafir.OpNotEqual)
	case types.SpecialFormLessThan:
		op, ok = c.pickIntegerOp(sig.Params()[0].Type(), ssafir.OpLessThan)
	case types.SpecialFormLessThanOrEqual:
		op, ok = c.pickIntegerOp(sig.Params()[0].Type(), ssafir.OpLessThanOrEqual)
	case types.SpecialFormGreaterThan:
		op, ok = c.pickIntegerOp(sig.Params()[0].Type(), ssafir.OpGreaterThan)
	case types.SpecialFormGreaterThanOrEqual:
		op, ok = c.pickIntegerOp(sig.Params()[0].Type(), ssafir.OpGreaterThanOrEqual)
	default:
		return nil, fmt.Errorf("%s: failed to compile %s: unsupported special form %s", c.fset.Position(list.ParenOpen), list.Print(), form.ID())
	}

	if !ok {
		return nil, fmt.Errorf("%s: failed to compile %s (%T): invalid %s type %s", c.fset.Position(list.ParenOpen), list.Print(), sig, form.ID(), sig.Result())
	}

	return c.CompileBinaryOperation(args, op, sig.Result())
}

// pickIntegerOp is a helper function for the common
// case where we want to pick a sized operation based
// on the size of an integer type. The base operation
// is used to derive the correct operation.
//
// If typ does not have an underlying type that is a
// sized integer type, then pickIntegerOp returns
// 0, false.
func (c *compiler) pickIntegerOp(typ types.Type, base ssafir.Op) (op ssafir.Op, ok bool) {
	underlying := types.Underlying(typ)
	var size int
	switch underlying {
	case types.Int8:
		size = 8
		op = base + 1
	case types.Int16:
		size = 16
		op = base + 2
	case types.Int32:
		size = 32
		op = base + 3
	case types.Int64:
		size = 64
		op = base + 4
	case types.Uint8:
		size = 8
		op = base + 5
	case types.Uint16:
		size = 16
		op = base + 6
	case types.Uint32:
		size = 32
		op = base + 7
	case types.Uint64:
		size = 64
		op = base + 8
	case types.Int:
		// This depends on the architecture.
		switch c.arch.RegisterSize * 8 {
		case 32:
			size = 32
			op = base + 3
		case 64:
			size = 64
			op = base + 4
		default:
			panic(fmt.Sprintf("%s: unexpected register size %d", c.arch, c.arch.RegisterSize))
		}
	case types.Uint:
		// This depends on the architecture.
		switch c.arch.RegisterSize * 8 {
		case 32:
			size = 32
			op = base + 7
		case 64:
			size = 64
			op = base + 8
		default:
			panic(fmt.Sprintf("%s: unexpected register size %d", c.arch, c.arch.RegisterSize))
		}
	case types.Uintptr:
		// This depends on the architecture.
		switch c.arch.PointerSize * 8 {
		case 32:
			size = 32
			op = base + 7
		case 64:
			size = 64
			op = base + 8
		default:
			panic(fmt.Sprintf("%s: unexpected pointer size %d", c.arch, c.arch.PointerSize))
		}
	default:
		return 0, false
	}

	// Check we got a valid result.
	info := op.Info()
	if info.Group != base || info.Size != size {
		panic(fmt.Sprintf("internal error: picking operation for %s from base %s gave %s (group %s, size %d), expected group %s and size %s", underlying, base, op, info.Group, info.Size, base, size))
	}

	return op, true
}

// pickSignedIntegerOp is a helper function for the common
// case where we want to pick a sized operation based
// on the size of an integer type.
//
// pickSignedIntegerOp is the same as pickIntegerOp, but it
// only accepts signed integer types.
func (c *compiler) pickSignedIntegerOp(typ types.Type, base ssafir.Op) (op ssafir.Op, ok bool) {
	underlying := types.Underlying(typ)
	var size int
	switch underlying {
	case types.Int8:
		size = 8
		op = base + 1
	case types.Int16:
		size = 16
		op = base + 2
	case types.Int32:
		size = 32
		op = base + 3
	case types.Int64:
		size = 64
		op = base + 4
	case types.Int:
		// This depends on the architecture.
		switch c.arch.RegisterSize * 8 {
		case 32:
			size = 32
			op = base + 3
		case 64:
			size = 64
			op = base + 4
		default:
			panic(fmt.Sprintf("%s: unexpected register size %d", c.arch, c.arch.RegisterSize))
		}
	default:
		return 0, false
	}

	// Check we got a valid result.
	info := op.Info()
	if info.Group != base || info.Size != size {
		panic(fmt.Sprintf("internal error: picking operation for %s from base %s gave %s (group %s, size %d), expected group %s and size %s", underlying, base, op, info.Group, info.Size, base, size))
	}

	return op, true
}

// pickUnsignedIntegerOp is a helper function for the common
// case where we want to pick a sized operation based
// on the size of an integer type.
//
// pickUnsignedIntegerOp is the same as pickIntegerOp, but it
// only accepts unsigned integer types.
func (c *compiler) pickUnsignedIntegerOp(typ types.Type, base ssafir.Op) (op ssafir.Op, ok bool) {
	underlying := types.Underlying(typ)
	var size int
	switch underlying {
	case types.Uint8:
		size = 8
		op = base + 1
	case types.Uint16:
		size = 16
		op = base + 2
	case types.Uint32:
		size = 32
		op = base + 3
	case types.Uint64:
		size = 64
		op = base + 4
	case types.Uint:
		// This depends on the architecture.
		switch c.arch.RegisterSize * 8 {
		case 32:
			size = 32
			op = base + 3
		case 64:
			size = 64
			op = base + 4
		default:
			panic(fmt.Sprintf("%s: unexpected register size %d", c.arch, c.arch.RegisterSize))
		}
	case types.Uintptr:
		// This depends on the architecture.
		switch c.arch.PointerSize * 8 {
		case 32:
			size = 32
			op = base + 3
		case 64:
			size = 64
			op = base + 4
		default:
			panic(fmt.Sprintf("%s: unexpected pointer size %d", c.arch, c.arch.PointerSize))
		}
	default:
		return 0, false
	}

	// Check we got a valid result.
	info := op.Info()
	if info.Group != base || info.Size != size {
		panic(fmt.Sprintf("internal error: picking operation for %s from base %s gave %s (group %s, size %d), expected group %s and size %s", underlying, base, op, info.Group, info.Size, base, size))
	}

	return op, true
}

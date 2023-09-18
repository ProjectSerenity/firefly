// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Abstract instructions emitted by the compiler.
// These are then transformed into concrete
// instructions specific to the target architecture,
// which is described as 'lowering'.

package main

var AbstractOps = []OpInfo{
	// Arithmetic ops.
	// Unary arithmetic.
	{Name: "NegateInt8", Operands: 1},
	{Name: "NegateInt16", Operands: 1},
	{Name: "NegateInt32", Operands: 1},
	{Name: "NegateInt64", Operands: 1},
	// Binary arithmetic.
	{Name: "AddString", Operands: 2},
	{Name: "AddInt8", Operands: 2, Commutative: true},
	{Name: "AddInt16", Operands: 2, Commutative: true},
	{Name: "AddInt32", Operands: 2, Commutative: true},
	{Name: "AddInt64", Operands: 2, Commutative: true},
	{Name: "AddUint8", Operands: 2, Commutative: true},
	{Name: "AddUint16", Operands: 2, Commutative: true},
	{Name: "AddUint32", Operands: 2, Commutative: true},
	{Name: "AddUint64", Operands: 2, Commutative: true},
	{Name: "SubtractInt8", Operands: 2, Commutative: true},
	{Name: "SubtractInt16", Operands: 2, Commutative: true},
	{Name: "SubtractInt32", Operands: 2, Commutative: true},
	{Name: "SubtractInt64", Operands: 2, Commutative: true},
	{Name: "SubtractUint8", Operands: 2, Commutative: true},
	{Name: "SubtractUint16", Operands: 2, Commutative: true},
	{Name: "SubtractUint32", Operands: 2, Commutative: true},
	{Name: "SubtractUint64", Operands: 2, Commutative: true},
	{Name: "MultiplyInt8", Operands: 2, Commutative: true},
	{Name: "MultiplyInt16", Operands: 2, Commutative: true},
	{Name: "MultiplyInt32", Operands: 2, Commutative: true},
	{Name: "MultiplyInt64", Operands: 2, Commutative: true},
	{Name: "MultiplyUint8", Operands: 2, Commutative: true},
	{Name: "MultiplyUint16", Operands: 2, Commutative: true},
	{Name: "MultiplyUint32", Operands: 2, Commutative: true},
	{Name: "MultiplyUint64", Operands: 2, Commutative: true},
	{Name: "DivideInt8", Operands: 2, Commutative: true},
	{Name: "DivideInt16", Operands: 2, Commutative: true},
	{Name: "DivideInt32", Operands: 2, Commutative: true},
	{Name: "DivideInt64", Operands: 2, Commutative: true},
	{Name: "DivideUint8", Operands: 2, Commutative: true},
	{Name: "DivideUint16", Operands: 2, Commutative: true},
	{Name: "DivideUint32", Operands: 2, Commutative: true},
	{Name: "DivideUint64", Operands: 2, Commutative: true},
	// Constant values.
	{Name: "ConstantBool"},   // ExtraInt is 0 for false and 1 for true.
	{Name: "ConstantString"}, // Extra is the string value.
	{Name: "ConstantInt8"},
	{Name: "ConstantInt16"},
	{Name: "ConstantInt32"},
	{Name: "ConstantInt64"},
	{Name: "ConstantUint8"},
	{Name: "ConstantUint16"},
	{Name: "ConstantUint32"},
	{Name: "ConstantUint64"},
	{Name: "ConstantUntypedInt"}, // Extra is the constant.Value.
	// Memory operations.
	{Name: "Drop", Operands: 1}, // Used for debugging only.
	{Name: "Copy", Operands: 1}, // Output = operand 0.
	{Name: "MakeMemoryState", Virtual: true},
	{Name: "Parameter", Virtual: true}, // ExtraInt is the parameter index into Function.Type.Params.
	{Name: "MakeResult", Operands: -1},
	{Name: "FunctionCall", Operands: -1},
	// Strings.
	{Name: "StringPtr", Operands: 1},
	{Name: "StringLen", Operands: 1},
	// Numerical casts.
	{Name: "CastInt8ToInt16", Operands: 1},
	{Name: "CastInt8ToInt32", Operands: 1},
	{Name: "CastInt8ToInt64", Operands: 1},
	{Name: "CastInt8ToUint8", Operands: 1},
	{Name: "CastInt8ToUint16", Operands: 1},
	{Name: "CastInt8ToUint32", Operands: 1},
	{Name: "CastInt8ToUint64", Operands: 1},
	{Name: "CastInt16ToInt8", Operands: 1},
	{Name: "CastInt16ToInt32", Operands: 1},
	{Name: "CastInt16ToInt64", Operands: 1},
	{Name: "CastInt16ToUint8", Operands: 1},
	{Name: "CastInt16ToUint16", Operands: 1},
	{Name: "CastInt16ToUint32", Operands: 1},
	{Name: "CastInt16ToUint64", Operands: 1},
	{Name: "CastInt32ToInt8", Operands: 1},
	{Name: "CastInt32ToInt16", Operands: 1},
	{Name: "CastInt32ToInt64", Operands: 1},
	{Name: "CastInt32ToUint8", Operands: 1},
	{Name: "CastInt32ToUint16", Operands: 1},
	{Name: "CastInt32ToUint32", Operands: 1},
	{Name: "CastInt32ToUint64", Operands: 1},
	{Name: "CastInt64ToInt8", Operands: 1},
	{Name: "CastInt64ToInt16", Operands: 1},
	{Name: "CastInt64ToInt32", Operands: 1},
	{Name: "CastInt64ToUint8", Operands: 1},
	{Name: "CastInt64ToUint16", Operands: 1},
	{Name: "CastInt64ToUint32", Operands: 1},
	{Name: "CastInt64ToUint64", Operands: 1},
	{Name: "CastUint8ToInt8", Operands: 1},
	{Name: "CastUint8ToInt16", Operands: 1},
	{Name: "CastUint8ToInt32", Operands: 1},
	{Name: "CastUint8ToInt64", Operands: 1},
	{Name: "CastUint8ToUint16", Operands: 1},
	{Name: "CastUint8ToUint32", Operands: 1},
	{Name: "CastUint8ToUint64", Operands: 1},
	{Name: "CastUint16ToInt8", Operands: 1},
	{Name: "CastUint16ToInt16", Operands: 1},
	{Name: "CastUint16ToInt32", Operands: 1},
	{Name: "CastUint16ToInt64", Operands: 1},
	{Name: "CastUint16ToUint8", Operands: 1},
	{Name: "CastUint16ToUint32", Operands: 1},
	{Name: "CastUint16ToUint64", Operands: 1},
	{Name: "CastUint32ToInt8", Operands: 1},
	{Name: "CastUint32ToInt16", Operands: 1},
	{Name: "CastUint32ToInt32", Operands: 1},
	{Name: "CastUint32ToInt64", Operands: 1},
	{Name: "CastUint32ToUint8", Operands: 1},
	{Name: "CastUint32ToUint16", Operands: 1},
	{Name: "CastUint32ToUint64", Operands: 1},
	{Name: "CastUint64ToInt8", Operands: 1},
	{Name: "CastUint64ToInt16", Operands: 1},
	{Name: "CastUint64ToInt32", Operands: 1},
	{Name: "CastUint64ToInt64", Operands: 1},
	{Name: "CastUint64ToUint8", Operands: 1},
	{Name: "CastUint64ToUint16", Operands: 1},
	{Name: "CastUint64ToUint32", Operands: 1},
}

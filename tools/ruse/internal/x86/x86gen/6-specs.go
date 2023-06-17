// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Merge the mnemonic and operand encoding tables
// for one instruction into a set of instruction
// specs.

package main

import (
	"strings"
	"unicode"
)

func (l *Listing) Specs(stats *Stats) ([]Spec, error) {
	// First, we make the operand encoding
	// entries easy to find.
	used := make(map[string]bool)
	encodings := make(map[string]*OperandEncoding)
	for _, encoding := range l.OperandEncodingTable {
		if _, ok := encodings[encoding.Encoding]; ok {
			return nil, Errorf(encoding.Page, "found multiple operand encoding entries with identifier %q", encoding.Encoding)
		}

		open := new(OperandEncoding)
		*open = encoding

		used[encoding.Encoding] = false
		encodings[encoding.Encoding] = open
	}

	specs := make([]Spec, len(l.MnemonicTable))
	for i, mnemonic := range l.MnemonicTable {
		// We accept cases where there is not
		// operand encoding table, provided
		// each mnemonic has no operand encoding
		// identifier and no parameters.
		if mnemonic.OperandEncoding == "" && strings.IndexFunc(mnemonic.Instruction, unicode.IsSpace) < 0 {
			m := new(Mnemonic)
			*m = mnemonic
			specs[i].M = m
			continue
		}

		open, ok := encodings[mnemonic.OperandEncoding]
		if !ok {
			open, ok = missingEncodings[mnemonic.Instruction]
			if ok {
				stats.ListingError()
			}
		}

		if !ok {
			return nil, Errorf(mnemonic.Page, "instruction %s  %s has operand encoding %q, which was not found", mnemonic.Opcode, mnemonic.Instruction, mnemonic.OperandEncoding)
		}

		m := new(Mnemonic)
		*m = mnemonic

		used[mnemonic.OperandEncoding] = true
		specs[i] = Spec{M: m, E: open}
	}

	// Check that we've used all of the
	// declared operand encodings.
	for _, encoding := range l.OperandEncodingTable {
		if !used[encoding.Encoding] {
			return nil, Errorf(encoding.Page, "operand encoding %q was not used by any instructions", encoding.Encoding)
		}
	}

	return specs, nil
}

// missingEncodings includes operand encoding
// entries that are missing from the manual.
//
// The index into missingEncodings is the
// instruction description.
var missingEncodings = map[string]*OperandEncoding{
	"FADD m32fp":            {Operands: [4]string{"ModRM:r/m"}},
	"FADD m64fp":            {Operands: [4]string{"ModRM:r/m"}},
	"FADD ST(0), ST(i)":     {Operands: [4]string{"None", "ST(i)"}},
	"FADD ST(i), ST(0)":     {Operands: [4]string{"ST(i)", "None"}},
	"FADDP ST(i), ST(0)":    {Operands: [4]string{"ST(i)", "None"}},
	"FIADD m32int":          {Operands: [4]string{"ModRM:r/m"}},
	"FIADD m16int":          {Operands: [4]string{"ModRM:r/m"}},
	"FBLD m80bcd":           {Operands: [4]string{"ModRM:r/m"}},
	"FBSTP m80bcd":          {Operands: [4]string{"ModRM:r/m"}},
	"FCMOVB ST(0), ST(i)":   {Operands: [4]string{"None", "ST(i)"}},
	"FCMOVE ST(0), ST(i)":   {Operands: [4]string{"None", "ST(i)"}},
	"FCMOVBE ST(0), ST(i)":  {Operands: [4]string{"None", "ST(i)"}},
	"FCMOVU ST(0), ST(i)":   {Operands: [4]string{"None", "ST(i)"}},
	"FCMOVNB ST(0), ST(i)":  {Operands: [4]string{"None", "ST(i)"}},
	"FCMOVNE ST(0), ST(i)":  {Operands: [4]string{"None", "ST(i)"}},
	"FCMOVNBE ST(0), ST(i)": {Operands: [4]string{"None", "ST(i)"}},
	"FCMOVNU ST(0), ST(i)":  {Operands: [4]string{"None", "ST(i)"}},
	"FCOM m32fp":            {Operands: [4]string{"ModRM:r/m"}},
	"FCOM m64fp":            {Operands: [4]string{"ModRM:r/m"}},
	"FCOM ST(i)":            {Operands: [4]string{"ST(i)"}},
	"FCOMP m32fp":           {Operands: [4]string{"ModRM:r/m"}},
	"FCOMP m64fp":           {Operands: [4]string{"ModRM:r/m"}},
	"FCOMP ST(i)":           {Operands: [4]string{"ST(i)"}},
	"FCOMI ST, ST(i)":       {Operands: [4]string{"None", "ST(i)"}},
	"FCOMIP ST, ST(i)":      {Operands: [4]string{"None", "ST(i)"}},
	"FUCOMI ST, ST(i)":      {Operands: [4]string{"None", "ST(i)"}},
	"FUCOMIP ST, ST(i)":     {Operands: [4]string{"None", "ST(i)"}},
	"FDIV m32fp":            {Operands: [4]string{"ModRM:r/m"}},
	"FDIV m64fp":            {Operands: [4]string{"ModRM:r/m"}},
	"FDIV ST(0), ST(i)":     {Operands: [4]string{"None", "ST(i)"}},
	"FDIV ST(i), ST(0)":     {Operands: [4]string{"ST(i)", "None"}},
	"FDIVP ST(i), ST(0)":    {Operands: [4]string{"ST(i)", "None"}},
	"FIDIV m16int":          {Operands: [4]string{"ModRM:r/m"}},
	"FIDIV m32int":          {Operands: [4]string{"ModRM:r/m"}},
	"FIDIV m64int":          {Operands: [4]string{"ModRM:r/m"}},
	"FDIVR m32fp":           {Operands: [4]string{"ModRM:r/m"}},
	"FDIVR m64fp":           {Operands: [4]string{"ModRM:r/m"}},
	"FDIVR ST(0), ST(i)":    {Operands: [4]string{"None", "ST(i)"}},
	"FDIVR ST(i), ST(0)":    {Operands: [4]string{"ST(i)", "None"}},
	"FDIVRP ST(i), ST(0)":   {Operands: [4]string{"ST(i)", "None"}},
	"FIDIVR m16int":         {Operands: [4]string{"ModRM:r/m"}},
	"FIDIVR m32int":         {Operands: [4]string{"ModRM:r/m"}},
	"FIDIVR m64int":         {Operands: [4]string{"ModRM:r/m"}},
	"FFREE ST(i)":           {Operands: [4]string{"ST(i)"}},
	"FICOM m16int":          {Operands: [4]string{"ModRM:r/m"}},
	"FICOM m32int":          {Operands: [4]string{"ModRM:r/m"}},
	"FICOMP m16int":         {Operands: [4]string{"ModRM:r/m"}},
	"FICOMP m32int":         {Operands: [4]string{"ModRM:r/m"}},
	"FILD m16int":           {Operands: [4]string{"ModRM:r/m"}},
	"FILD m32int":           {Operands: [4]string{"ModRM:r/m"}},
	"FILD m64int":           {Operands: [4]string{"ModRM:r/m"}},
	"FIST m16int":           {Operands: [4]string{"ModRM:r/m"}},
	"FIST m32int":           {Operands: [4]string{"ModRM:r/m"}},
	"FISTP m16int":          {Operands: [4]string{"ModRM:r/m"}},
	"FISTP m32int":          {Operands: [4]string{"ModRM:r/m"}},
	"FISTP m64int":          {Operands: [4]string{"ModRM:r/m"}},
	"FISTTP m16int":         {Operands: [4]string{"ModRM:r/m"}},
	"FISTTP m32int":         {Operands: [4]string{"ModRM:r/m"}},
	"FISTTP m64int":         {Operands: [4]string{"ModRM:r/m"}},
	"FLD m32fp":             {Operands: [4]string{"ModRM:r/m"}},
	"FLD m64fp":             {Operands: [4]string{"ModRM:r/m"}},
	"FLD m80fp":             {Operands: [4]string{"ModRM:r/m"}},
	"FLD ST(i)":             {Operands: [4]string{"ST(i)"}},
	"FLDCW m2byte":          {Operands: [4]string{"ModRM:r/m"}},
	"FLDENV m14/28byte":     {Operands: [4]string{"ModRM:r/m"}},
	"FMUL m32fp":            {Operands: [4]string{"ModRM:r/m"}},
	"FMUL m64fp":            {Operands: [4]string{"ModRM:r/m"}},
	"FMUL ST(0), ST(i)":     {Operands: [4]string{"None", "ST(i)"}},
	"FMUL ST(i), ST(0)":     {Operands: [4]string{"ST(i)", "None"}},
	"FMULP ST(i), ST(0)":    {Operands: [4]string{"ST(i)", "None"}},
	"FIMUL m16int":          {Operands: [4]string{"ModRM:r/m"}},
	"FIMUL m32int":          {Operands: [4]string{"ModRM:r/m"}},
	"FRSTOR m94/108byte":    {Operands: [4]string{"ModRM:r/m"}},
	"FSAVE m94/108byte":     {Operands: [4]string{"ModRM:r/m"}},
	"FNSAVE m94/108byte":    {Operands: [4]string{"ModRM:r/m"}},
	"FST m32fp":             {Operands: [4]string{"ModRM:r/m"}},
	"FST m64fp":             {Operands: [4]string{"ModRM:r/m"}},
	"FST m80fp":             {Operands: [4]string{"ModRM:r/m"}},
	"FST ST(i)":             {Operands: [4]string{"ST(i)"}},
	"FSTP m32fp":            {Operands: [4]string{"ModRM:r/m"}},
	"FSTP m64fp":            {Operands: [4]string{"ModRM:r/m"}},
	"FSTP m80fp":            {Operands: [4]string{"ModRM:r/m"}},
	"FSTP ST(i)":            {Operands: [4]string{"ST(i)"}},
	"FSTCW m2byte":          {Operands: [4]string{"ModRM:r/m"}},
	"FNSTCW m2byte":         {Operands: [4]string{"ModRM:r/m"}},
	"FSTENV m14/28byte":     {Operands: [4]string{"ModRM:r/m"}},
	"FNSTENV m14/28byte":    {Operands: [4]string{"ModRM:r/m"}},
	"FSTSW m2byte":          {Operands: [4]string{"ModRM:r/m"}},
	"FSTSW AX":              {Operands: [4]string{"None"}},
	"FNSTSW m2byte":         {Operands: [4]string{"ModRM:r/m"}},
	"FNSTSW AX":             {Operands: [4]string{"None"}},
	"FSUB m32fp":            {Operands: [4]string{"ModRM:r/m"}},
	"FSUB m64fp":            {Operands: [4]string{"ModRM:r/m"}},
	"FSUB ST(0), ST(i)":     {Operands: [4]string{"None", "ST(i)"}},
	"FSUB ST(i), ST(0)":     {Operands: [4]string{"ST(i)", "None"}},
	"FSUBP ST(i), ST(0)":    {Operands: [4]string{"ST(i)", "None"}},
	"FISUB m16int":          {Operands: [4]string{"ModRM:r/m"}},
	"FISUB m32int":          {Operands: [4]string{"ModRM:r/m"}},
	"FSUBR m32fp":           {Operands: [4]string{"ModRM:r/m"}},
	"FSUBR m64fp":           {Operands: [4]string{"ModRM:r/m"}},
	"FSUBR ST(0), ST(i)":    {Operands: [4]string{"None", "ST(i)"}},
	"FSUBR ST(i), ST(0)":    {Operands: [4]string{"ST(i)", "None"}},
	"FSUBRP ST(i), ST(0)":   {Operands: [4]string{"ST(i)", "None"}},
	"FISUBR m16int":         {Operands: [4]string{"ModRM:r/m"}},
	"FISUBR m32int":         {Operands: [4]string{"ModRM:r/m"}},
	"FISUBR m64int":         {Operands: [4]string{"ModRM:r/m"}},
	"FUCOM ST(i)":           {Operands: [4]string{"ST(i)"}},
	"FUCOMP ST(i)":          {Operands: [4]string{"ST(i)"}},
	"FXCH ST(i)":            {Operands: [4]string{"ST(i)"}},
	"POP DS":                {Operands: [4]string{"None"}},
	"POP ES":                {Operands: [4]string{"None"}},
	"POP FS":                {Operands: [4]string{"None"}},
	"POP GS":                {Operands: [4]string{"None"}},
	"POP SS":                {Operands: [4]string{"None"}},
	"POP CS":                {Operands: [4]string{"None"}},
	"PUSH CS":               {Operands: [4]string{"None"}},
	"PUSH DS":               {Operands: [4]string{"None"}},
	"PUSH ES":               {Operands: [4]string{"None"}},
	"PUSH FS":               {Operands: [4]string{"None"}},
	"PUSH GS":               {Operands: [4]string{"None"}},
	"PUSH SS":               {Operands: [4]string{"None"}},
	"INT 3":                 {Operands: [4]string{"None"}},
}

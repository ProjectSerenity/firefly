// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package ssafir implements a single static assignment (SSA) form
// intermediate representation (IR) for the Ruse language.
package ssafir

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"firefly-os.dev/tools/ruse/binary"
	"firefly-os.dev/tools/ruse/sys"
	"firefly-os.dev/tools/ruse/token"
	"firefly-os.dev/tools/ruse/types"
)

// MemoryState is a fake type which is used to represent
// the state of a program. Specifically, these states are
// used to ensure that operations that affect wider memory
// are not reordered in a way that would break causality.
//
// Like any other value in the SSAIR, each memory state
// is only created once but can be read many times. Each
// function begins by creating a new memory state in case
// one is needed later in the function. Any instruction
// that consumes a memory state should produce a memory
// state for operations that occur afterwards.
//
// Blocks that return from a function should consume the
// last memory state created in the function (in the control
// flow that leads to that function return) so that the
// instruction that produced it is not erroneously deemed
// unused.
type MemoryState struct{}

var _ types.Type = MemoryState{}

func (s MemoryState) Underlying() types.Type { return s }
func (s MemoryState) String() string         { return "memory state" }

// Result is a fake type which is used to represent a
// value returned from a function, plus the final memory
// state of the function. See MemoryState for more
// details.
type Result struct {
	Value  types.Type
	Memory MemoryState
}

var _ types.Type = Result{}

func (r Result) Underlying() types.Type { return r }
func (r Result) String() string         { return "result" }

// Op describes what operation an intermediate representation
// instruction will perform.
type Op int

// Info returns a rich description of the operation.
func (op Op) Info() *OpInfo {
	if int(op) >= len(opInfo) {
		return nil
	}

	return &opInfo[op]
}

func (op Op) String() string {
	if int(op) >= len(opInfo) {
		return fmt.Sprintf("Op(%d)", op)
	}

	return opInfo[op].Name
}

// OpInfo gives rich information about an operation.
type OpInfo struct {
	Name        string
	Abstract    bool // Not machine-specific.
	Virtual     bool // Not executed on the machine.
	Operands    int  // Number of arguments (or -1 if variadic).
	Commutative bool // The first two arguments can be reordered without effect.
}

// ID uniquely identifies a value or block within a single
// function. Blocks and values have their own namespace, so
// a block and a value may have the same ID, but no two blocks
// will share an ID within the same function, nor will any
// two values.
type ID int

// idAllocator returns monotonically increasing positive ID
// values.
type idAllocator struct {
	last ID
}

func (a *idAllocator) Next() ID {
	next := a.last + 1
	if next >= math.MaxInt32 {
		panic("function has too many values/blocks")
	}

	a.last = next
	return next
}

// Value represents a single entry in the intermediate
// representation. A Value describes the result of a
// single instruction and may be referenced by subsequent
// Values.
//
// Once a Value has been created, its ID and Type must
// not be modified. Its other fields can be modified,
// provided its semantics remain consistent.
type Value struct {
	// The Value's unique identifier within its parent
	// function.
	ID ID

	// The operation that produces the value.
	Op Op

	// The type of the value.
	Type types.Type

	// Extra information about the instruction.
	//
	// When the extra info is an int, it is stored
	// in ExtraInt.
	Extra    any
	ExtraInt int64

	// The arguments that are used to produce this
	// value.
	Args []*Value

	// The basic block to which this value belongs.
	Block *Block

	// The start and end of the source code that results
	// in this value.
	Pos token.Pos
	End token.Pos

	// The number of times this value is used elsewhere,
	// such as in the Args of another value or the
	// Controls of a block.
	Uses int
}

// Print returns a textual representation of v.
//
// The output takes the form:
//
//	v{ID} := op({args}) type [extra={extra}] [(names)]
func (v *Value) Print() string {
	return v.print(1)
}

func (v *Value) print(maxID ID) string {
	var buf strings.Builder
	idWidth := int(math.Log10(float64(maxID))) + 1
	fmt.Fprintf(&buf, "v%-*d := (%s", idWidth, v.ID, v.Op)
	for _, arg := range v.Args {
		buf.WriteByte(' ')
		buf.WriteString(arg.String())
	}

	if v.Extra != nil {
		fmt.Fprintf(&buf, " (extra %v)", v.Extra)
	}

	buf.WriteString(") ")
	buf.WriteString(v.Type.String())

	var names []string
	for name, values := range v.Block.Function.NamedValues {
		for _, value := range values {
			if value == v {
				names = append(names, name.Name())
				break
			}
		}
	}

	if len(names) > 0 {
		sort.Strings(names)
		fmt.Fprintf(&buf, " (%s)", strings.Join(names, ", "))
	}

	return buf.String()
}

// String returns the value's ID with a 'v' prefix.
func (v *Value) String() string {
	return fmt.Sprintf("v%d", v.ID)
}

// Block represents a single basic block within a
// function's control flow graph.
//
// A block takes one of the following forms:
//
//	Kind    | Controls        | Successors
//	--------+-----------------+-------------
//	Normal  | []              | [next]
//	If      | [boolean Value] | [then, else]
//	Exit    | [memory Value]  | []
type Block struct {
	// The Block's unique identifier within its parent
	// function.
	ID ID

	// The kind of basic block.
	Kind BlockKind

	// The likelihood that the first branch will be taken.
	//
	// If BranchLikely, Successors[0] is expected.
	// If BranchUnlikely, Successors[1] is expected.
	// Ignored if len(Successors) < 2.
	// Fatal error if len(Successors) > 2 and not BranchUnknown.
	Likely BranchPrediction

	// Subsequent blocks in the control flow graph, if any.
	Successors []Edge

	// Previous blocks in the control flow graph, if any.
	Predecessors []Edge

	// A value that will determine how this block
	// exits, depending on its kind. For example, an
	// If block will have a control, which will be a
	// boolean value. A Return block will have a
	// control, which will be a result containing the
	// last memory state created on that control flow,
	// plus any returned value.
	Control *Value

	// The function to which this block belongs.
	Function *Function

	// The start and end of the source code that results
	// in this block.
	Pos token.Pos
	End token.Pos

	// The set of values produced in this block.
	Values []*Value
}

// Print returns a textual representation of b.
//
// The output takes the form:
//
//	kind({control}) [-> successors [(likelihood)]]
func (b *Block) Print() string {
	var buf strings.Builder
	fmt.Fprintf(&buf, "(%s", b.Kind)
	if b.Control != nil {
		buf.WriteByte(' ')
		buf.WriteString(b.Control.String())
	}

	buf.WriteByte(')')

	if len(b.Successors) > 0 {
		buf.WriteString(" -> ")
		for i, block := range b.Successors {
			if i > 0 {
				buf.WriteString(", ")
			}

			buf.WriteString(block.Block().String())
		}

		if b.Likely == BranchLikely {
			buf.WriteString(" (likely)")
		} else if b.Likely == BranchUnlikely {
			buf.WriteString(" (unlikely)")
		}
	}

	return buf.String()
}

// String returns the value's ID with a 'b' prefix.
func (b *Block) String() string {
	return fmt.Sprintf("b%d", b.ID)
}

// NewValue creates a new value at the given position
// and operation, adds it to the block, and returns it.
//
// Any number of arguments can be provided.
func (b *Block) NewValue(pos, end token.Pos, op Op, typ types.Type, args ...*Value) *Value {
	v := b.Function.newValue(op, typ, b, pos, end)
	v.Args = args
	for _, arg := range args {
		arg.Uses++
	}

	return v
}

// NewValueInt creates a new value at the given position
// and operation, adds it to the block, and returns it.
//
// An integer extra and ay number of arguments can be
// provided.
func (b *Block) NewValueInt(pos, end token.Pos, op Op, typ types.Type, extra int64, args ...*Value) *Value {
	v := b.Function.newValue(op, typ, b, pos, end)
	v.Extra = extra
	v.ExtraInt = extra
	v.Args = args
	for _, arg := range args {
		arg.Uses++
	}

	return v
}

// NewValueExtra creates a new value at the given position
// and operation, adds it to the block, and returns it.
//
// An extra and ay number of arguments can be provided.
func (b *Block) NewValueExtra(pos, end token.Pos, op Op, typ types.Type, extra any, args ...*Value) *Value {
	v := b.Function.newValue(op, typ, b, pos, end)
	v.Extra = extra
	v.Args = args
	for _, arg := range args {
		arg.Uses++
	}

	return v
}

// BlockKind describes the role a basic block takes
// in the control flow graph of a function.
type BlockKind int

const (
	BlockInvalid BlockKind = iota
	BlockNormal
	BlockIf
	BlockReturn
	BlockReturnJump
)

var blockKindString = [...]string{
	BlockInvalid:    "BlockInvalid",
	BlockNormal:     "Normal",
	BlockIf:         "If",
	BlockReturn:     "Return",
	BlockReturnJump: "ReturnJump",
}

func (k BlockKind) String() string { return blockKindString[k] }

// BranchPrediction indicates the likelihood that an
// if branch will be taken. That is, if the prediction
// is BranchLikely, Block.Successors[0] is expected to
// happen, whereas if BranchUnlikely, Block.Successors[1]
// is expected.
type BranchPrediction int8

const (
	BranchUnlikely BranchPrediction = -1 + iota
	BranchUnknown
	BranchLikely
)

// Edge describes the connection between two basic blocks.
//
// The edge identifies the target block and the index into
// that block's Predecessors where an edge will point back
// to the origin block.
type Edge struct {
	// The target block.
	b *Block

	// The index in b.Successors that
	// will point back to the origin
	// block.
	i int
}

// Block returns the destination block.
func (e Edge) Block() *Block { return e.b }

// Index returns the index in e.Block().Predecessors at
// which the edge pointing to the origin block will appear.
func (e Edge) Index() int { return e.i }

func (e Edge) String() string { return fmt.Sprintf("{%v, %d}", e.b, e.i) }

// Link describes a case where the runtime
// address of a symbol in a Ruse programme
// must be inserted into the code of a
// function during the linking process.
type Link struct {
	Pos     token.Pos // The position of the symbol in the Ruse source.
	Name    string    // The absolute symbol name.
	Type    LinkType  // The method for writing the address.
	Size    uint8     // The address size in bits.
	Offset  int       // The offset into the function code where the symbol must be inserted.
	Address uintptr   // The offset into the function code used to calculate relative addresses.
}

// LinkType defines how a symbol address is
// written into a Ruse binary.
type LinkType uint8

const (
	LinkFullAddress     LinkType = iota // Copy the address in full.
	LinkRelativeAddress                 // Copy the address, relative to the origin.
)

func (t LinkType) String() string {
	switch t {
	case LinkFullAddress:
		return "full address"
	default:
		return fmt.Sprintf("LinkType(%d)", t)
	}
}

// Perform completes the link action, writing
// the address into the binary using the
// information in `l` and `fun`, which is the
// symbol for the function into which `address`
// is written.
func (l *Link) Perform(arch *sys.Arch, object []byte, fun *binary.Symbol, address uintptr) error {
	switch l.Type {
	case LinkFullAddress:
		offset := int(fun.Offset) + l.Offset
		switch l.Size {
		case 32:
			arch.ByteOrder.PutUint32(object[offset:], uint32(address))
		case 64:
			arch.ByteOrder.PutUint64(object[offset:], uint64(address))
		default:
			return fmt.Errorf("cannot link %s at offset %d: bad link size: %d", l.Name, l.Offset, l.Size)
		}
	case LinkRelativeAddress:
		offset := int(fun.Offset) + l.Offset
		base := fun.Address + l.Address
		rel := address - base
		switch l.Size {
		case 16:
			arch.ByteOrder.PutUint16(object[offset:], uint16(rel))
		case 32:
			arch.ByteOrder.PutUint32(object[offset:], uint32(rel))
		case 64:
			arch.ByteOrder.PutUint64(object[offset:], uint64(rel))
		default:
			return fmt.Errorf("cannot link %s at offset %d: bad link size: %d", l.Name, l.Offset, l.Size)
		}
	default:
		return fmt.Errorf("cannot link %s at offset %d: unrecognised link type %s", l.Name, l.Offset, l.Type)
	}

	return nil
}

// Function represents a single Ruse function.
//
// Each function is compiled separately.
type Function struct {
	Name   string           // The function name.
	Type   *types.Signature // The funciton signature.
	Blocks []*Block         // The basic blocks in this function's control flow graph.
	Entry  *Block           // The basic block that begins the control flow graph.

	Extra any // Extra info for the assembler.

	// ID allocators.
	blocks idAllocator
	values idAllocator

	// Parameters to the function and the values
	// they become.
	NamedValues map[*types.Variable][]*Value

	// Linking actions, if any.
	Links []*Link
}

// Print returns a textual representation for f.
func (f *Function) Print() string {
	var buf strings.Builder
	fmt.Fprintf(&buf, "%s %s\n", f.Name, f.Type)
	for _, b := range f.Blocks {
		fmt.Fprintf(&buf, "%s:\n", b)
		// TODO: Sort values into dependency order.
		for _, v := range b.Values {
			fmt.Fprintf(&buf, "\t%s\n", v.print(f.values.last))
		}

		fmt.Fprintf(&buf, "\t%s\n", b.Print())
	}

	return buf.String()
}

// NewBlock creates a new basic block of the given kind,
// assigns it a unique identifier within this function,
// appends it to f.Blocks, and returns it.
func (f *Function) NewBlock(pos token.Pos, kind BlockKind) *Block {
	b := &Block{
		ID:       f.blocks.Next(),
		Kind:     kind,
		Function: f,
		Pos:      pos,
		End:      pos, // Overwritten later.
	}

	f.Blocks = append(f.Blocks, b)

	return b
}

func (f *Function) newValue(op Op, typ types.Type, b *Block, pos, end token.Pos) *Value {
	v := &Value{
		ID:    f.values.Next(),
		Op:    op,
		Type:  typ,
		Block: b,
		Pos:   pos,
		End:   end,
	}

	b.Values = append(b.Values, v)

	return v
}

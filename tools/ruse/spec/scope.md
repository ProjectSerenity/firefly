# Ruse scope

The scope of an identifier in Ruse is determined by its block.

## Blocks

A _block_ is a possibly empty sequence of declarations and statements within matching brace brackets.

```
Block         = "{" StatementList "}" .
StatementList = { Statement ";" } .
```

In addition to explicit blocks in the source code, there are implicit blocks:

- The _universe block_ encompasses all Ruse source text.
- Each [package](#Packages) has a _package block_ containing all Ruse source text for that package.
- Each file has a _file block_ containing all Ruse source text in that file.

Blocks nest and influence [scoping](#Declarations_and_scope).


## Declarations and scope

A _declaration_ binds a non-[blank](#Blank_identifier) identifier to a [constant](#Constant_declarations), [variable](#Variable_declarations), [function](#Function_declarations), [package](#Import_declarations). Every identifier in a program must be declared. No identifier may be declared twice in the same block, and no identifier may be declared in both the file and package block.

The [blank identifier](#Blank_identifier) may be used like any other identifier in a declaration, but it does not introduce a binding and thus is not declared.

```
Declaration   = ConstDecl | VarDecl .
TopLevelDecl  = Declaration | FunctionDecl .
```

The _scope_ of a declared identifier is the extent of source text in which the identifier denotes the specified type, variable, function, or package.

Ruse is lexically scoped using [blocks](#Blocks):

- The scope of a [predeclared identifier](#Predeclared_identifiers) is the universe block.
- The scope of an identifier denoting a constant, variable, or function declared at top level (outside any function) is the package block.
- The scope of the package name of an imported package is the file block of the file containing the import declaration.
- The scope of an identifier denoting a function parameter is the function body.
- The scope of a constant or variable identifier declared inside a function begins at the end of the ConstSpec or VarSpec and ends at the end of the innermost containing block.

An identifier declared in a block may be redeclared in an inner block. While the identifier of the inner declaration is in scope, it denotes the entity declared by the inner declaration.

The [package clause](#Package_clause) is not a declaration; the package name does not appear in any scope. Its purpose is to identify the files belonging to the same [package](#Packages) and to specify the default package name for import declarations.

### Label scopes

Labels are declared by [labeled statements](#Labeled_statements) and are used in the ["break"](#Break_statements), ["continue"](#Continue_statements), and ["goto"](#Goto_statements) statements. It is illegal to define a label that is never used. In contrast to other identifiers, labels are not block scoped and do not conflict with identifiers that are not labels. The scope of a label is the body of the function in which it is declared and excludes the body of any nested function.

### Blank identifier

The _blank identifier_ is represented by the underscore character `_`. It serves as an anonymous placeholder instead of a regular (non-blank) identifier and has special meaning in [declarations](#Declarations_and_scope), as an [operand](#Operands), and in [assignment statements](#Assignment_statements).


### Predeclared identifiers

The following identifiers are implicitly declared in the [universe block](#Blocks):

```
Types:
	bool byte
	int int8 int16 int32 int64 string
	uint uint8 uint16 uint32 uint64 uintptr

Constants:
	true false

Functions:
	len
```

### Exported identifiers

An identifier may be _exported_ to permit access to it from another package. An identifier is exported if both:

- the first character of the identifier's name is a Unicode uppercase letter (Unicode character category Lu); and
- the identifier is declared in the [package block](#Blocks).

All other identifiers are not exported.

### Uniqueness of identifiers

Given a set of identifiers, an identifier is called _unique_ if it is _different_ from every other in the set. Two identifiers are different if they are spelled differently, or if they appear in different [packages](#Packages) and are not [exported](#Exported_identifiers). Otherwise, they are the same.

### Constant declarations

Constants are declared using a `let` statement, as described in the [grammar](./grammar.md).

### Function declarations

Functions are declared using a `func` statement, as described in the [grammar](./grammar.md)

### Assembly function declarations

Assembly functions are declared using an `asm-func` statement, as described in the [grammar](./grammar.md)

#### Assembly function annotations

Assembly functions support attached annotations, which may vary depending on the target architecture. Common annotations include:

- `'(arch [arch identifier])`: Specify the target architecture. Assembly functions that specify a different architecture than the current compile target will be ignored.
- `'(mode [mode integer])`: Specify the target CPU mode. This affects the assembler's encoding process and may constrain the set of instructions available. If omitted, the target architecture's default CPU mode is assumed.
- `'(section [section reference])`: Specify which program section should own the assembled function. If omitted, "sections/Code" is assumed.

```
'(arch x86-64)              ; Target x86-64.
'(mode 32)                  ; Override the CPU mode from 64 to 32 bits.
'(section sections.Strings) ; Store the assmebled function in the Strings section. Note that this would likely fail to execute, as Strings is not executable.
(asm-func (main)
	(syscalls.Exit 0))
```

#### Assembly function macros

Assembly functions can use macros to reference data outside the function. Supported macros include:

- `(@ [reference])`: Substitute the address of the referenced object. Supports functions, array constants, and string constants.
- '(string-pointer [string reference])`: Substitute a string's data pointer.

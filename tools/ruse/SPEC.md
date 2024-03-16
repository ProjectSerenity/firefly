# Ruse

Ruse is a Lisp-like programming language, designed for low-level programming without an operating system. It is strongly typed and provides memory safety with low runtime overhead. Programs are constructed from _packages_, whose properties allow efficient management of dependencies.

The syntax is radically simple, allowing a consistent grammar to be built on top.

## Notation

The syntax is specified using a [variant](https://en.wikipedia.org/wiki/Wirth_syntax_notation) of Extended Backus-Naur Form (EBNF):

```
Syntax      = { Production } .
Production  = production_name "=" [ Expression ] "." .
Expression  = Term { "|" Term } .
Term        = Factor { Factor } .
Factor      = production_name | token [ "…" token ] | Group | Option | Repetition .
Group       = "(" Expression ")" .
Option      = "[" Expression "]" .
Repetition  = "{" Expression "}" .
```

Productions are expressions constructed from terms and the following operators, in increasing precedence:

```
|   alternation
()  grouping
[]  option (0 or 1 times)
{}  repetition (0 to n times)
```

Lowercase production names are used to identify lexical (terminal) tokens. Non-terminals are in CamelCase. Lexical tokens are enclosed in double quotes `""` or back quotes `\`\``.

The form `a … b` represents the set of characters from `a` to `b` as alternatives. The horizontal ellipsis `…` is also used elsewhere in the spec to informally denote various enumerations or code snippets that are not further specified. The character `…` (as opposed to the three characters `...`) is not a token of the Ruse language.

## Source code representation

Source code is Unicode text encoded in [UTF-8](https://en.wikipedia.org/wiki/UTF-8). The text is not canonicalised, so a single accented code point is distinct from the same character constructed from combining an accent and a letter; those are treated as two code points.  For simplicity, this document will use the unqualified term _character_ to refer to a Unicode code point in the source text.

Each code point is distinct; for instance, uppercase and lowercase letters are different characters.

Implementation restriction: For compatibility with other tools, a compiler may disallow the NUL character (U+0000) in the source text.

Implementation restriction: For compatibility with other tools, a compiler may ignore a UTF-8-encoded byte order mark (U+FEFF) if it is the first Unicode code point in the source text. A byte order mark may be disallowed anywhere else in the source.

### Characters

The following terms are used to denote specific Unicode character categories:

```
newline        = /* the Unicode code point U+000A */ .
unicode_char   = /* an arbitrary Unicode code point except newline */ .
unicode_letter = /* a Unicode code point categorized as "Letter" */ .
unicode_digit  = /* a Unicode code point categorized as "Number, decimal digit" */ .
```

In [The Unicode Standard 8.0](https://www.unicode.org/versions/Unicode8.0.0/), Section 4.5 "General Category" defines a set of character categories. Ruse treats all characters in any of the Letter categories Lu, Ll, Lt, Lm, or Lo as Unicode letters, and those in the Number category Nd as Unicode digits.

### Letters and digits

The underscore character `_` (U+005F) is considered a lowercase letter.

```
letter        = unicode_letter | "_" .
decimal_digit = "0" … "9" .
binary_digit  = "0" | "1" .
hex_digit     = "0" … "9" | "A" … "F" | "a" … "f" .
```

## Lexical elements

### Comments

Comments serve as program documentation. _Line comments_ start with a semicolon (`;`) and stop at the end of the line.

A comment cannot start inside a [string literal](#String_literals), or inside a comment. A comment acts like a newline.

### Tokens

Tokens form the vocabulary of the Ruse language. There are three classes: _identifiers_, _punctuation_, and _literals_. _White space_, formed from spaces (U+0020), horizontal tabs (U+0009), carriage returns (U+000D), and newlines (U+000A), is ignored except as it separates tokens that would otherwise combine into a single token. While breaking the input into tokens, the next token is the longest sequence of characters that form a valid token.

### Identifiers

Identifiers name program entities such as variables and types. An identifier is a sequence of one or more letters and digits, or a single plus or minus. The first character in an identifier must be a letter or punctuation.

```
identifier_initial    = letter | "!" | "$" | "%" | "&" | "*" | "/" | ":" | "<" | "=" | ">" | "?" | "~" | "_" | "^" | "|" .
identifier_subsequent = letter | unicode_digit | "+" | "-" .
identifier            = "+" | "-" | identifier_initial { identifier_subsequent } .
```

```
a
+
_x9
ThisVariableIsExported
αβ
kebab-case-is-fine/good
```

Some identifiers are [predeclared](#Predeclared_identifiers).

### Keywords

The following keywords are reserved and may not be used as identifiers:

- `asm-func`
- `func`
- `import`
- `let`
- `package`

### Punctuation

The following characters are punctuation:

- `(`
- `.`
- `'`
- `)`

### Integer literals

An integer literal is a sequence of digits representing an [integer constant](#Constants). An optional prefix sets a non-decimal base: `0b` for binary and `0x` for hexadecimal. A single `0` is considered a decimal zero. In hexadecimal literals, letters `a` to `f` and `A` to `F` represent values 10 through 15.

For readability, an underscore character `_` may appear after a base prefix or between successive digits; such underscores do not change the literal's value.

```
int_lit        = decimal_lit | binary_lit | hex_lit .
decimal_lit    = "0" | ( "1" … "9" ) [ [ "_" ] decimal_digits ] .
binary_lit     = "0b" [ "_" ] binary_digits .
hex_lit        = "0x" [ "_" ] hex_digits .

decimal_digits = decimal_digit { [ "_" ] decimal_digit } .
binary_digits  = binary_digit { [ "_" ] binary_digit } .
hex_digits     = hex_digit { [ "_" ] hex_digit } .
```

```
42
4_2
0xBadFace
0xBad_Face
0x_67_7a_2f_cc_40_c6
170141183460469231731687303715884105727
170_141183_460469_231731_687303_715884_105727

_42         ; an identifier, not an integer literal
42_         ; invalid: _ must separate successive digits
4__2        ; invalid: only one _ at a time
0_xBadFace  ; invalid: _ must separate successive digits
```

### String literals

A string literal represents a [string constant](#Constants) obtained from concatenating a sequence of characters. String literals are character sequences between double quotes, as in `"bar"`. Within the quotes, any character may appear except newline and unescaped double quote. The text between the quotes forms the value of the literal, while multi-character sequences beginning with a backslash encode values in various formats.

Several backslash escapes allow arbitrary values to be encoded as ASCII text. There are three ways to insert a numeric constant into the string content: `\x` followed by exactly two hexadecimal digits; `\u` followed by exactly four hexadecimal digits, and a `\U` followed by exactly eight hexadecimal digits. In each case the value of the literal is the value represented by the digits in the corresponding base.

Although these representations all result in an integer being inserted into the string content, they have different valid ranges. The escapes `\u` and `\U` represent Unicode code points so within them some values are illegal, in particular those above `0x10FFFF` and surrogate halves.

After a backslash, certain single-character escapes represent special values:

```
\a   U+0007 alert or bell
\b   U+0008 backspace
\f   U+000C form feed
\n   U+000A line feed or newline
\r   U+000D carriage return
\t   U+0009 horizontal tab
\v   U+000B vertical tab
\\   U+005C backslash
\"   U+0022 double quote
```

An unrecognized character following a backslash in a string literal is illegal.

```
string_lit       = `"` { unicode_value | byte_value } `"` .
unicode_value    = unicode_char | little_u_value | big_u_value | escaped_char .
byte_value       = hex_byte_value .
hex_byte_value   = `\` "x" hex_digit hex_digit .
little_u_value   = `\` "u" hex_digit hex_digit hex_digit hex_digit .
big_u_value      = `\` "U" hex_digit hex_digit hex_digit hex_digit
                           hex_digit hex_digit hex_digit hex_digit .
escaped_char     = `\` ( "a" | "b" | "f" | "n" | "r" | "t" | "v" | `\` | `"` ) .
```

```
"\n"
"\""                 ; same as `"`
"Hello, world!\n"
"日本語"
"\u65e5本\U00008a9e"
"\xff\u00FF"
"\uD800"             ; illegal: surrogate half
"\U00110000"         ; illegal: invalid Unicode code point
```

These examples all represent the same string:

```
"日本語"                                 ; UTF-8 input text
"\u65e5\u672c\u8a9e"                    ; the explicit Unicode code points
"\U000065e5\U0000672c\U00008a9e"        ; the explicit Unicode code points
"\xe6\x97\xa5\xe6\x9c\xac\xe8\xaa\x9e"  ; the explicit UTF-8 bytes
```

If the source code represents a character as two code points, such as a combining form involving an accent and a letter, the result will appear as two code points if placed in a string literal.

## Constants

There are _boolean constants_, _integer constants_, and _string constants_.

A constant value is represented by an [integer](#Integer_literals) or [string](#String_literals) literal, an identifier denoting a constant, a [constant expression](#Constant_expressions), a [conversion](#Conversions) with a result that is a constant, or the result value of some built-in functions such as `len` applied to constant arguments. The boolean truth values are represented by the predeclared constants `true` and `false`.

Numeric constants represent exact values of arbitrary precision and do not overflow. Consequently, there are no constants denoting the IEEE-754 negative zero, infinity, and not-a-number values.

Constants may be [typed](#Types) or _untyped_. Literal constants, `true`, `false`, and certain [constant expressions](#Constant_expressions) containing only untyped constant operands are untyped.

A constant may be given a type explicitly by a [constant declaration](#Constant_declarations) or [conversion](#Conversions), or implicitly when used in a [variable declaration](#Variable_declarations) or an [assignment statement](#Assignment_statements) or as an operand in an [expression](#Expressions). It is an error if the constant value cannot be [represented](#Representability) as a value of the respective type.

An untyped constant has a _default type_ which is the type to which the constant is implicitly converted in contexts where a typed value is required, for instance, in a declaration such as `(let i 0)` where there is no explicit type. The default type of an untyped constant is `bool`, `int`, or `string` respectively, depending on whether it is a boolean, integer, or string constant.

Implementation restriction: Although numeric constants have arbitrary
precision in the language, a compiler may implement them using an
internal representation with limited precision. That said, every
implementation must:

- Represent integer constants with at least 256 bits.
- Give an error if unable to represent an integer constant precisely.

These requirements apply both to literal constants and to the result of evaluating [constant expressions](#Constant_expressions).


## Variables

A variable is a storage location for holding a _value_. The set of permissible values is determined by the variable's [_type_](#Types).

A [variable declaration](#Variable_declarations) or, for function parameters and results, the signature of a [function declaration](#Function_declarations) or [function literal](#Function_literals) reserves storage for a named variable.

_Structured_ variables of [array](#Array_types) types have elements. Each such element acts like a variable.

The _static type_ (or just _type_) of a variable is the type given in its declaration or the type of an element of a structured variable.

A variable's value is retrieved by referring to the variable in an [expression](#Expressions); it is the most recent value [assigned](#Assignment_statements) to the variable.

## Types

A type determines a set of values. A type may be denoted by a _type name_, if it has one. A type may also be specified using a _type literal_, which composes a type from existing types.

```
Type      = TypeName | TypeLit | "(" Type ")" .
TypeName  = identifier | QualifiedIdent .
TypeLit   = ArrayType .
```

The language [predeclares](#Predeclared_identifiers) certain type names. Others are introduced with [type declarations](#Type_declarations). _Composite types_ - array, and function types - may be constructed using type literals.

Predeclared types and defined types are called _named types_.

### Boolean types

A _boolean type_ represents the set of Boolean truth values denoted by the predeclared constants `true` and `false`. The predeclared boolean type is `bool`; it is a [defined type](#Type_definitions).

# Integer types

An _integer_ type represents the set of integer values. The predeclared architecture-independent integer types are:

```
uint8       the set of all unsigned  8-bit integers (0 to 255)
uint16      the set of all unsigned 16-bit integers (0 to 65535)
uint32      the set of all unsigned 32-bit integers (0 to 4294967295)
uint64      the set of all unsigned 64-bit integers (0 to 18446744073709551615)

int8        the set of all signed  8-bit integers (-128 to 127)
int16       the set of all signed 16-bit integers (-32768 to 32767)
int32       the set of all signed 32-bit integers (-2147483648 to 2147483647)
int64       the set of all signed 64-bit integers (-9223372036854775808 to 9223372036854775807)

byte        alias for uint8
```

The value of an _n_-bit integer is _n_ bits wide and represented using [two's complement arithmetic](https://en.wikipedia.org/wiki/Two's_complement).

There is also a set of predeclared integer types with implementation-specific sizes:

```
uint     either 32 or 64 bits
int      same size as uint
uintptr  an unsigned integer large enough to store the uninterpreted bits of a pointer value
```

To avoid portability issues all integer types are [defined types](#Type_definitions) and thus distinct except `byte`, which is an [alias](#Alias_declarations) for `uint8`. Explicit conversions are required when different integer types are mixed in an expression or assignment. For instance, `int32` and `int` are not the same type even though they may have the same size on a particular architecture.

### String types

A _string type_ represents the set of string values. A string value is a (possibly empty) sequence of bytes. The number of bytes is called the length of the string and is never negative. Strings are immutable: once created, it is impossible to change the contents of a string. The predeclared string type is `string`; it is a [defined type](#Type_definitions).

The length of a string `s` can be discovered using the built-in function [`len`](#Length_and_capacity). The length is a compile-time constant if the string is a constant. A string's bytes can be accessed by integer [indices](#Index_expressions) 0 to `(- (len s) 1)`.


### Array types

An array is a numbered sequence of elements of a single type, called the element type. The number of elements is called the length of the array and is never negative.

```
ArrayType   = "array/" ( ArrayLength "/" ) ElementType .
ArrayLength = Expression .
ElementType = Type .
```

The length is part of the array's type; it must evaluate to a non-negative [constant](#Constants) [representable](#Representability) by a value of type `int`. The length of array `a` can be discovered using the built-in function [`len`](#Length_and_capacity). The elements can be addressed by integer [indices](#Index_expressions) 0 to `(- (len a) 1)`. Array types are always one-dimensional but may be composed to form multi-dimensional types.

```
array/32/byte
array/3/array/5/int
```

### Function types

A function type denotes the set of all functions with the same parameter and result types.

```
FunctionType   = "(" "func" Signature ")" .
Signature      = Parameters [ Result ] .
Parameters     = { "(" identifier type ")" } .
Result         = type .
```

Within the parameters, each name stands for one item of the specified type and all non-[blank](#Blank_identifier) names in the signature must be [unique](#Uniqueness_of_identifiers).

```
(func)                  ; A function with no parameters or result.
(func (x int) int)      ; A function with one integer parameter and an integer result.
(func (a int) (b bool)) ; A function with one integer and one boolean parameter and no result.
(func (func int))       ; A function that returns a function that returns an integer.
```

## Properties of types and values

### Underlying types

Each type `T` has an _underlying type_: If `T` is one of the predeclared boolean, integer, or string types, or a type literal, the corresponding underlying type is `T` itself. Otherwise, `T`'s underlying type is the underlying type of the type to which `T` refers in its declaration.

### Assignability

A value `x` of type `V` is _assignable_ to a [variable](#Variables) of type `T` ("`x` is assignable to `T`") if one of the following conditions applies:

- `V` and `T` are identical.
- `V` and `T` have identical [underlying types](#Underlying_types) and at least one of `V` or `T` is not a [named type](#Types).
- `x` is an untyped [constant](#Constants) [representable](#Representability) by a value of type `T`.

### Representability

A [constant](#Constants) `x` is _representable_ by a value of type `T`, if one of the following conditions applies:

- `x` is in the set of values [determined](#Types) by `T`.

```
x                   T           x is representable by a value of T because

"foo"               string      "foo" is in the set of string values
1024                int16       1024 is in the set of 16-bit integers
```

```
x                   T           x is not representable by a value of T because

0                   bool        0 is not in the set of boolean values
1024                byte        1024 is not in the set of unsigned 8-bit integers
-1                  uint16      -1 is not in the set of unsigned 16-bit integers
```

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

A constant declaration binds an identifier (the name of the constant) to the value of a [constant expression](#Constant_expressions).

```
ConstDecl      = "let" IdentifierDecl Expression .
IdentifierDecl = identifier | "(" identifier type ")" .
```

If the type is present, the constant takes the type specified, and the expressions must be [assignable](#Assignability) to that type. If the type is omitted, the constant takes the type of the expression. If the expression value is an untyped [constant](#Constants), the declared constant remains untyped and the constant identifier denotes the constant value.

```
(let zero 0)                         ; untyped integer constant
(let (size int64) 1024)              ; typed integer constant
(let bools (array/bool true false))  ; array constant
```

### Function declarations

A function declaration binds an identifier, the _function name_, to a function.

```
FunctionDecl = "(" "func" "(" FunctionName Signature ")" FunctionBody .
FunctionName = identifier .
FunctionBody = Block .
```

If the function's [signature](#Function_types) declares a result, the function body's statement list must end in a [terminating statement](#Terminating_statements).

### Assembly function declarations

An assembly function declaration binds an identifier, the _function name_, to a function implemented in architecture-specific assembly.

```
AsmFunctionDecl = "(" "asm-func" "(" FunctionName Signature ")" AsmFunctionBody .
AsmFunctionBody = AsmBlock .
```

## Expressions

An expression specifies the computation of a value by applying functions to operands.

### Operands

Operands denote the elementary values in an expression. An operand may be a literal, or a (possibly [qualified](#Qualified_identifiers)) non-[blank](#Blank_identifier) identifier denoting a [constant](#Constant_declarations), [variable](#Variable_declarations), or [function](#Function_declarations).

```
Operand     = Literal | OperandName .
Literal     = BasicLit | FunctionLit .
BasicLit    = int_lit | string_lit .
OperandName = identifier | QualifiedIdent .
```

The [blank identifier](#Blank_identifier) may appear as an operand only on the left-hand side of an [assignment statement](#Assignment_statements).

### Qualified identifiers

A _qualified identifier_ is an identifier qualified with a package name prefix. Both the package name and the identifier must not be [blank](#Blank_identifier).

```
QualifiedIdent = PackageName "." identifier .
```

A qualified identifier accesses an identifier in a different package, which must be [imported](#Import_declarations). The identifier must be [exported](#Exported_identifiers) and declared in the [package block](#Blocks) of that package.

```
math.Sin // denotes the Sin function in package math
```

### Calls

Given an expression `f` with a type `F` of [function type](#Function_types),

```
(f a1 a2 … an)
```

calls `f` with arguments `a1, a2, … an`. Arguments must be single-valued expressions [assignable](#Assignability) to the parameter types of `F` and are evaluated before the function is called. The type of the expression is the result type of `F`.

```
(math.Atan2 x y)
```

In a function call, the function value and arguments are evaluated in [the usual order](#Order_of_evaluation). After they are evaluated, the parameters of the call are passed by value to the function and the called function begins execution. The return parameters of the function are passed by value back to the caller when the function returns.

### Order of evaluation

At package level, [initialization dependencies](#Package_initialization) determine the evaluation order of individual initialization expressions in [variable declarations](#Variable_declarations). Otherwise, when evaluating the [operands](#Operands) of an expression, assignment, or [return statement](#Return_statements), all function calls are evaluated in lexical left-to-right order.

## Built-in functions

Built-in functions are [predeclared](#Predeclared_identifiers). They are called like any other function but some of them accept a type instead of an expression as the first argument.

The built-in functions do not have standard Ruse types, so they can only appear in [call expressions](#Calls); they cannot be used as function values.

### Length and capacity

The built-in function `len` takes arguments of various types and returns an untyped integer result. The implementation guarantees that the result always fits into an `int`.

```
Call      Argument type    Result

(len s)   string type      string length in bytes
          array/n/T        array length (== n)
```

The expression `(len s)` is [constant](#Constants) if `s` is a string constant. The expression `(len s)` is constant if the type of `s` is an array and the expression `s` does not contain [function calls](#Calls); in this case `s is not evaluated. Otherwise, invocations of `len` are not constant and `s` is evaluated.

## Packages

Ruse programs are constructed by linking together _packages_. A package in turn is constructed from one or more source files that together declare constants, variables and functions belonging to the package and which are accessible in all files of the same package. Those elements may be [exported](#Exported_identifiers) and used in another package.

### Source file organization

Each source file consists of a package clause defining the package to which it belongs, followed by a possibly empty set of import declarations that declare packages whose contents it wishes to use, followed by a possibly empty set of declarations of functions, variables, and constants.

```
SourceFile       = PackageClause { ImportDecl } { TopLevelDecl } .
```

### Package clause

A package clause begins each source file and defines the package to which the file belongs.

```
PackageClause  = "(" "package" PackageName ")" .
PackageName    = identifier .
```

The PackageName must not be the [blank identifier](#Blank_identifier).

```
(package math)
```

A set of files sharing the same PackageName form the implementation of a package. An implementation may require that all source files for a package inhabit the same directory.

### Import declarations

An import declaration states that the source file containing the declaration depends on functionality of the _imported_ package ([§Program initialization and execution](#Program_initialization_and_execution)) and enables access to [exported](#Exported_identifiers) identifiers of that package. The import names an identifier (PackageName) to be used for access and an ImportPath that specifies the package to be imported.

```
ImportDecl       = "(" "import" ( ImportSpec | ImportSpecList ) .
ImportSpec       = [ PackageName ] ImportPath .
ImportPath       = string_lit .
ImportSpecList   = "(" ImportSpec ")" { ImportSpecList } .
```

The PackageName is used in [qualified identifiers](#Qualified_identifiers) to access exported identifiers of the package within the importing source file. It is declared in the [file block](#Blocks). If the PackageName is omitted, it defaults to the identifier specified in the [package clause](#Package_clause) of the imported package.

The interpretation of the ImportPath is implementation-dependent but it is typically a substring of the full file name of the compiled package and may be relative to a repository of installed packages.

Implementation restriction: A compiler may restrict ImportPaths to non-empty strings using only characters belonging to [Unicode's](https://www.unicode.org/versions/Unicode6.3.0/) L, M, N, P, and S general categories (the Graphic characters without spaces) and may also exclude the characters `!"#$%&'()*,:;<=>?[\]^\`{|}` and the Unicode replacement character U+FFFD.

Consider a compiled a package containing the package clause `(package math)`, which exports function `Sin`, and installed the compiled package in the file identified by `"lib/math"`. This table illustrates how `Sin` is accessed in files that import the package after the various types of import declaration.

```
Import declaration          Local name of Sin

(import   "lib/math")         math.Sin
(import m "lib/math")         m.Sin
```

An import declaration declares a dependency relation between the importing and imported package. It is illegal for a package to import itself, directly or indirectly, or to directly import a package without referring to any of its exported identifiers.


### An example package

Here is a complete Ruse package that implements Hello World in x86-64 assembly for the Linux ABI.

```
(package main)

(import "syscalls")

(func (main)
	(syscalls.Print 1 "Hello, World!\n")
	(syscalls.Exit 0))
```

### Program execution

A complete program is created by linking a single, unimported package called the _main package_ with all the packages it imports, transitively. The main package must have package name `main` and declare a function `main` that takes no arguments and returns no value.

```
(func (main) … )
```

The `main` function may be an assembly function.

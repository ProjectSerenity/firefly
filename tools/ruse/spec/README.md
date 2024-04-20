# Ruse

Ruse is a Lisp-like programming language, designed for low-level programming without an operating system. It is strongly typed and provides memory safety with low runtime overhead. Programs are constructed from _packages_, whose properties allow efficient management of dependencies.

## Lexical elements

See the Ruse language's [syntax](./syntax.md) and [grammar](./grammar.md).

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

See the Ruse language's [types](./types.md).

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

The blank identifier may appear as an operand only on the left-hand side of an [assignment statement](#Assignment_statements).

### Scope and blocks

See [scope and blocks](./scope.md).

### Order of evaluation

At package level, [initialization dependencies](#Package_initialization) determine the evaluation order of individual initialization expressions in [variable declarations](#Variable_declarations). Otherwise, when evaluating the [operands](#Operands) of an expression, assignment, or [return statement](#Return_statements), all function calls are evaluated in lexical left-to-right order.

## Special forms

See the Ruse language's [special forms](./special-forms.md).

## Packages

Ruse programs are constructed by linking together _packages_. A package in turn is constructed from one or more source files that together declare constants, variables and functions belonging to the package and which are accessible in all files of the same package. Those elements may be [exported](#Exported_identifiers) and used in another package.

### Program execution

A complete program is created by linking a single, unimported package called the _main package_ with all the packages it imports, transitively. The main package must have package name `main` and declare a function `main` that takes no arguments and returns no value.

```
(func (main) … )
```

The `main` function may be an assembly function.

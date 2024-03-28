# Ruse types

Ruse uses a static type system.

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

### Integer types

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

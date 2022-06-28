# Plan interface description language

Plan is a minimal Lisp-like language used to define the system call ABI of an operating system.
With an OS's ABI defined in a Plan document, compatible code can be generated in other programming
languages. This ensures that user and kernel code do not disagree on the structure or layout of
the ABI.

Sample:

```scheme
(structure
	(name file info)
	(docs
		"Information about a file in the filesystem")
	(field
		(name name pointer)
		(docs
			"A pointer to the name of the file (UTF-8 encoded).")
		(type *constant byte))
	(field
		(name name length)
		(docs
			"The length in bytes of the file name.")
		(type uint64)))


(enumeration
	(name error)
	(docs
		"A common error that has been encountered "
		"while responding to a system call.")
	(type uint64)
	(value
		(name no error)
		(docs
			"The system call was successful and no error occurred."))
	(value
		(name bad syscall)
		(docs
			"The system call specified does not exist, or has not "
			"been implemented."))
	(value
		(name illegal parameter)
		(docs
			"An invalid or malformed parameter was provided to "
			"the system calls.")))


(syscall
	(name print message)
	(docs
		"Prints a message to the process's standard output.")
	(arg1
		(name pointer)
		(docs
			"The pointer to readable memory where the message resides. "
			"No restrictions are placed on the contents pointed to by "
			"`pointer`. For example, the contents do not need to be UTF-8 "
			"encoded and NUL bytes have no special effects.")
		(type *constant byte))
	(arg2
		(name length)
		(docs
			"The number of bytes that will be read and printed from `pointer`.")
		(type uint64))
	(result1
		(name written)
		(docs
			"The number of bytes that were read and printed from `pointer`.")
		(type uint64))
	(result2
		(name error)
		(docs
			"Any error encountered while printing the message.")
		(type error)))
```

## Comments

Plan documents can use line comments on otherwise bare lines for copyright statements,
license declarations, and delineating sections within the document. Note that this means a
comment cannot be used on the same line as ABI declarations. Comments start with a semicolon
and at least one space.

Syntax examples:

```scheme
; Single-line comment

; Multi-line comment split over multiple
; lines.
```

## Subtypes

Plan has many common subtypes, which are used to define larger types. None of these subtypes
can be used directly in the ABI, they exist purely to help define a complete type. This section
lists each of the common subtypes, along with their syntax and some examples.

### Name

Every type, field, and value is defined with a name, with all type names existing in a single
namespace. That is, no two types can have the same name, even if they have different types.
Similarly, a type that has fields or values may not have multiple fields or values with the same
name. However, it is allowed for a type to have a field/value with the same name as another type,
or another type's field/value. That is, if types `foo` and `bar` exist, either may have a field or
value called `foo` or `bar`, but no other types may be called `foo` or `bar`.

Names are always lowercase and consist of one or more words. Words must start with a letter
but can contain or end with numbers. When a type is generated in a target language, its name
is localised into that language to conform with that language's naming scheme. For example, a
type with the name `file info` might be localised into `FileInfo`, `file_info`, `fileInfo`, or
some other case format. In documentation and Plan documents, Plan names are always expressed
as spaced lowercase format (`file info` in the previous example).

Syntax examples:

```scheme
(name foo)
(name example)
(name file info)
```

### Docs

Every type, field, and value is defined with documentation. This is to ensure that the ABI's
intent is clear and easy to explain. Plan uses a flexible notation for defining rich text
documentation. The documentation on each type and value is included in any generated code, and
the documentation on each type, field, and value is included in any generated ABI documentation.
Best effort is made to retain rich text when documentation is generated into another format. For
example, a reference to another part of the ABI will be translated to a hyperlink if the target
format can express one.

Documentation consists of one or more documentation items, which combine together as necessary.
The defined documentation item types are:

- Text: plain text.
- Code: source code, which should be displayed in a monospace typeface, preserving whitespace.
- Reference: a link to another part of the ABI, such as a type or syscall.
- Newline: a line break, automatically inserted in place of an empty text item.

If two consecutive documentation items are separated by an empty text item, the empty text item
will be replaced by a newline item. Otherwise, two consecutive items will have an extra text
item inserted between them, containing a single space. This allows documentation items to be
written more intuitively without sacrificing aesthetics in the resulting documentation.

Syntax examples:

```scheme
; Documentation containing the text "An example doc string".
(docs "An example" "doc" "string")

; Documentation containing the text "A <code>code sample</code> string".
(docs "A" (code "code sample") "string")

; Documentation with a reference to the ABI item `file info`.
(docs "A pointer to a" (reference file info))

; Documentation containing the text "A doc string\nwith a newline".
(docs "A doc string" "" "with a newline")
```

### Field

A field is an item within a composite type, which has both a name and a type. The most common
example of a field is an entry in a structure. Alternatively, a field can be a region of
padding in memory to ensure the alignment of the wider type. The contents of a padding field
should be ignored by both sides of the ABI. Note that a field can either declare its type, or
declare that it is padding; no field can have both.

Syntax examples:

```scheme
(field
	(name field name)
	(docs "The field's documentation.")
	(type uint64))

(field
	(name padding field)
	(docs "3 bytes of padding within the parent type.")
	(padding 3))
```

### Value

A value is an item within a type, which has a name and an inferred value and type. The most
common example of a value is an entry in an enumeration. The meaning of a value is determined
by the parent type.

Syntax examples:

```scheme
(value
	(name first value)
	(docs "The first value in a parent type."))
(value
	(name second value)
	(docs "The second value in a parent type."))
```

### Parameter

A parameter is a value passed directly in the ABI, typically using a CPU register. A parameter
cannot typically contain a composite type, but this is covered in more detail below in the
section on types. A parameter has a name and type, much like a field. Unlike a field, parameters
cannot be padding. The other significant difference between parameters and fields is that a
parameter is declared with its purpose, rather than the term `parameter`. This is covered in
more detail in the section on syscalls. A parameter is either an argument passed with a syscall,
or a result, returned by a syscall. In either case, the parameter is numbered, starting at 1.

Syntax examples:

```scheme
(arg1
	(name first argument)
	(docs "The first argument to a syscall.")
	(type uint64))
(result2
	(name second result)
	(docs "The second result from a syscall.")
	(type byte))
```

## Types

Plan has several data structure types that can be declared in a Plan document and used in the
ABI produced from that document. This section lists each of the types, along with their syntax,
any pre-declared instances of the type, and the type's layout in memory. Each type also has a
layout table, which specifies three properties; the type's alignment, size, and suitability to
be passed in a syscall parameter.

The alignment of a type is the memory alignment of the type on the target architecture. That is,
the offset in bytes into a structure at which a field of the type appears must be an exact multiple
of its alignment. For example, if type `A` has an alignment of 8, then any structure fields of
type `A` must have an offset into the structure that is an exact multiple of 8. Alignment values
are larger than 0 and are an exact power of 2.

The size of a type is the number of bytes that a value of the type will consume in memory. The
size of a type is always an exact multiple of its alignment.

A type may, or may not, be suitable to be passed directly in a syscall parameter. Typically,
primitive unary values can be passed as a parameter and composite types cannot. If a type is not
suitable to be passed as a syscall parameter directly, it would normally be passed via a pointer,
or in another composite type (which itself, will likely be passed by pointer).

### Array

The array type declares a fixed-size sequence of one or more contiguous elements, all of which
have the same element type. While an array can be embedded in another type directly, it is more
common to use a pointer to an array. An array declares its size (which must be a positive integer
larger than zero) and its type. There are no restrictions on an array's type.

Syntax examples:

```scheme
(array
	(name sequence of bytes)
	(docs "A fixed array of 12 bytes.")
	(size 12)
	(type byte))

(array
	(name array of arrays)
	(docs "An array of arrays.")
	(size 2)
	(type sequence of bytes))

(array
	(name array of pointers to sequences of bytes)
	(docs "An array of read-only pointers.")
	(size 6)
	(type *constant sequence of bytes))
```

Layout:

| Alignment | Size              | Parameter |
| --------- | ----------------- | --------- |
| `.type`   | `.type` × `.size` | No        |

An array's alignment is equal to the alignment of its element type. Its size is equal to the
size of its element type, multiplied by the number of elements.

### Bitfield

A bitfield is an integer type, with separate values for each bit in the integer. A bitfield
cannot have more values than the number of bits in its underlying type. For example, a bitfield
with the underlying type `uint8` cannot have more than 8 values. The first declared value
represents the least significant bit, the second declared value represents the second least
significant bit, and so on. That is, the numerical representation of each successive value will
be successive powers of 2, starting at 2^0 (1). An instance of a bitfield can have multiple
values set simultaneously, unlike an enumeration.

Syntax examples:

```scheme
(bitfield
	(name permissions)
	(docs "The set of operations that can be performed on a file.")
	(type uint8)
	(value
		(name execute)
		(docs "The file can be executed (value 1)."))
	(value
		(name write)
		(docs "The file can be written (value 2)."))
	(value
		(name read)
		(docs "The file can be read (value 4).")))
```

Layout:

| Alignment | Size    | Parameter |
| --------- | ------- | --------- |
| `.type`   | `.type` | Yes       |

A bitfield inherits its alignment and size from its underlying integer type.

### Enumeration

An enumeration is an integer type, with separate values for each number that can be represented
by the integer. An enumeration cannot have more values than the number of non-negative integers
that can be represented. For example, an enumeration with the underlying type `sint8` cannot
have more than 128 values. The first declared value represents 0, the second represents 1, and
so on. An instance of an enumeration represents exactly one value, unlike a bitfield.

Enumerations can be declared using embedding to include the values from one or more other
enumerations. When values are embedded from another enumeration, they are inserted into the new
enumeration, exactly as if the embed declaration was textually replaced by the corresponding
value declarations from the embedded enumeration. Note that any value or embed declarations
before an embed declaration will mean that the embedded values will have a different numerical
value from the parent enumeration. The values from an enumeration of one underlying type can
be embedded into an enumeration of another underlying type, provided the number of values in
either enumeration does not exceed the usual constraint on the number of values for a given
type.

Error enumerations are a special case for enumerations used to describe the set of errors that
can occur while handling a syscall. Any enumeration whose name ends in the word `error` is
eligible to be an error enumeration. There is a set of values that are required for any error
enumeration. This is to ensure that there is a base set of error values that will be encoded
identically in a parameter, irrespective of the specific error type. The expectation is that
there will be an error enumeration with the name `error` that contains only this required set
of values. Aside from the name and value requirements unique to error enumerations, an error
enumeration behaves as a normal enumeration.

Syntax examples:

```scheme
(enumeration
	(name layer3 protocol)
	(docs "An internet network protocol.")
	(type uint8)
	(value
		(name ipv4)
		(docs "The internet protocol, version 4."))
	(value
		(name ipv6)
		(docs "The internet protocol, version 6.")))

(enumeration
	(name layer4 protocol)
	(docs "A transport network protocol.")
	(type uint8)
	(value
		(name tcp)
		(docs "The transmission control protocol."))
	(value
		(name udp)
		(docs "The user datagram protocol.")))

(enumeration
	(name protocols)
	(docs "Any network protocol.")
	(type uint16)
	(value
		(name ethernet)
		(docs "The ethernet protocol."))
	(embed layer3 protocol)
	(embed layer4 protocol)
	(value
		(name http)
		(docs "The hypertext transfer protoco.")))

(enumeration
	(name error)
	(docs
		"A common error that has been encountered while responding to"
		"a system call.")
	(type uint64)
	(value
		(name no error)
		(docs "The system call was successful and no error occurred."))
	(value
		(name bad syscall)
		(docs "The system call specified does not exist, or has not been implemented."))
	(value
		(name illegal arg1)
		(docs "An invalid or malformed argument 1 was provided to the system call."))
	(value
		(name illegal arg2)
		(docs "An invalid or malformed argument 2 was provided to the system call."))
	(value
		(name illegal arg3)
		(docs "An invalid or malformed argument 3 was provided to the system call."))
	(value
		(name illegal arg4)
		(docs "An invalid or malformed argument 4 was provided to the system call."))
	(value
		(name illegal arg5)
		(docs "An invalid or malformed argument 5 was provided to the system call."))
	(value
		(name illegal arg6)
		(docs "An invalid or malformed argument 6 was provided to the system call.")))
```

Layout:

| Alignment | Size    | Parameter |
| --------- | ------- | --------- |
| `.type`   | `.type` | Yes       |

An enumeration inherits its alignment and size from its underlying integer type.

### Integer

An integer represents a fixed-size integral number, which is either signed or unsigned. Plan
includes the following predefined integer types:

| Integer  | Bits | Sign     | Alignment | Size | Parameter |
| -------- | ---- | -------- | --------- | ---- | --------- |
| `byte`   | 8    | unsigned | 1         | 1    | Yes       |
| `uint8`  | 8    | unsigned | 1         | 1    | Yes       |
| `uint16` | 16   | unsigned | 2         | 2    | Yes       |
| `uint32` | 32   | unsigned | 4         | 4    | Yes       |
| `uint64` | 64   | unsigned | 8         | 8    | Yes       |
| `sint8`  | 8    | signed   | 1         | 1    | Yes       |
| `sint16` | 16   | signed   | 2         | 2    | Yes       |
| `sint32` | 32   | signed   | 4         | 4    | Yes       |
| `sint64` | 64   | signed   | 8         | 8    | Yes       |

Additional integer types can be defined, declaring the underlying type. Where possible, the
generated code will make new integer types distinct from their underlying type. Types that
use an integer underlying type, such as bitfield and enumeration, may only use the predefined
integer types as their underlying type.

Syntax examples:

```scheme
(integer
	(name port number)
	(docs "The number for a TCP or UDP port.")
	(type uint16))
```

Layout:

| Alignment | Size    | Parameter |
| --------- | ------- | --------- |
| `.type`   | `.type` | Yes       |

A new integer inherits its alignment and size from its underlying integer type.

### Pointer

A pointer is the numerical address of another type in memory. A pointer identifies whether
the underlying data is mutable. Mutable data can be modified by the recipient, whereas
constant data can only be read. It is the responsibility of the recipient to ensure that a
pointer value references an acceptable region of memory before dereferencing it. A pointer
value may not be null (have the value 0).

Syntax examples:

```
; A pointer to read-only data.
*constant byte

; A pointer to writable data.
*mutable uint32
```

Layout:

| Alignment | Size | Parameter |
| --------- | ---- | --------- |
| 8         | 8    | Yes       |

### Structure

A structure is a composite type containing one or more fields in a single contiguous
region of memory. Unlike most Plan types, a structure is not inherently aligned, in that
it is easy to express a structure definition that would not be aligned, such as a structure
consisting of two fields, one of type `uint16` and one of type `uint8`. However, Plan
requires that structures be defined in a manner that results in every field and the
structure as a whole being correctly aligned. This can be performed using padding fields
and is checked by the Plan compiler.

Syntax examples:

```scheme
(structure
	(name file info)
	(docs "Information about a file on disk.")
	(field
		(name name pointer)
		(docs "A pointer to the file's name.")
		(type *constant byte))
	(field
		(name name size)
		(docs "The number of bytes that should be read from" (code "name pointer") ".")
		(type uint64))
	(field
		(name file size)
		(docs "The size of the file in bytes.")
		(type uint64))
	(field
		(name permissions)
		(docs "The actions that may be performed on the file.")
		(type permissions))
	(field
		(name trailing padding)
		(docs "Padding to keep the structure aligned.")
		(padding 7)))
```

Layout:

| Alignment               | Size                          | Parameter |
| ----------------------- | ----------------------------- | --------- |
| Largest field alignment | Sum of all fields and padding | No        |

## Syscall

A syscall describes a single function in the ABI. A syscall consists of a name,
documentation, between zero and six argument parameters, and between one and two
result parameters. Whereas Plan types define the layout of some data in memory,
a syscall defines the mechanism used to exchange data between a user process and
the kernel. The specific CPU mechanism used to invoke a syscall will depend on
the target archetecture.

When syscalls are defined, they are added to an implicit enumeration of syscall
numbers, which is called `syscalls`. Each syscall definition adds a value to the
enumeration with the same name as the syscall. This enumeration is then used to
identify which syscall is being invoked. This means that the first syscall defined
is identified with the value 0, the second with value 1, and so on.

As described above in the section on parameters, argumetn and result parameters
are declared with their parameter type and number. This ensures that each parameter's
purpose is unambiguous. This is illustrated in the syntax examples below.

A syscall can have up to six argument parameters, which must all have a type
that is suitable for a parameter. A syscall has either one or two result parameters,
which must all have a type that is suitable for a parameter. Furthermore, the
final parameter must be an error enumeration, as described in the section on
enumerations. Even if there is only one result parameter, it is passed in the
second result parameter slot in the ABI.

Syntax examples:

```scheme
(syscall
	(name exit thread)
	(docs
		"Exits the current thread, ceasing execution."
		""
		"Unless it has been disabled,"
		(reference exit thread)
		"will not return.")
	(result1
		(name error)
		(docs "An error if" (reference exit thread) "has been disabled.")
		(type error)))

(syscall
	(name print message)
	(docs "Prints a message to the process's standard output.")
	(arg1
		(name pointer)
		(docs "The pointer to readable memory where the message resides.")
		(type *constant byte))
	(arg2
		(name size)
		(docs
			"The number of bytes that will be read and printed from"
			(code "pointer")
			".")
		(type uint64))
	(result1
		(name written)
		(docs
			"The number of bytes that were read and printed from"
			(code "pointer")
			".")
		(type uint64))
	(result2
		(name error)
		(docs "Any error encountered while printing the message.")
		(type error)))
```

## Group

A group is primarily a documentation construct. A group is used to declare a
set of ABI details with a common theme. For example, a group might include all
syscalls, structures, and other types used for networking. A group is declared
by specifying its name and documentation, plus one or more items, declared by
their type followed by their name.

Groups are primarily used in documentation. However, each group that contains
syscalls is included in the generated code as a list of those syscalls. The
types in a group are not included in this list. The intent behind this feature
is that it may be helpful when denying a process access to certain categories
of syscalls.

Syntax examples:

```scheme
(group
	(name debugging)
	(docs "The set of functionality used for debugging the kernel and the ABI.")
	(structure registers)
	(syscall debug abi registers)
	(syscall debug abi errors)
	(syscall debug abi bounds))
```

## Layout summary

| Type             | Alignment     | Size              | Parameter |
| ---------------- | ------------- | ----------------- | --------- |
| Array            | `.type`       | `.type` × `.size` | No        |
| Bitfield         | `.type`       | `.type`           | Yes       |
| Enumeration      | `.type`       | `.type`           | Yes       |
| Integer `byte`   | 1             | 1                 | Yes       |
| Integer `uint8`  | 1             | 1                 | Yes       |
| Integer `uint16` | 2             | 2                 | Yes       |
| Integer `uint32` | 4             | 4                 | Yes       |
| Integer `uint64` | 8             | 8                 | Yes       |
| Integer `sint8`  | 1             | 1                 | Yes       |
| Integer `sint16` | 2             | 2                 | Yes       |
| Integer `sint32` | 4             | 4                 | Yes       |
| Integer `sint64` | 8             | 8                 | Yes       |
| New integer      | `.type`       | `.type`           | Yes       |
| Pointer          | 8             | 8                 | Yes       |
| Structure        | Largest field | Sum of fields     | No        |

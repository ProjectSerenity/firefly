# Ruse grammar

The Ruse grammar is a series of rules for constructing combinations of [syntax](./syntax.md) elements.

## Common grammar clauses

There are several units of grammar that are used widely in Ruse. This section introduces them.

### Annotations

Quoted lists are widely used as annotations to influence the behaviour of other grammar. Annotations consist of one or more sequential quoted lists, followed by an unquoted list.

There must be no blank lines between the quoted lists, or between the final quoted list and the unquoted list. However, other whitespace between them is allowed.

Each quoted list's first element must be an identifier, which is used to indicate the kind of annotation. It is illegal for multiple annotations in a group to have the same leading identifier.

The relative order of the quoted lists is not significant, but the convention is to order them alphabetically according to each annotation's leading identifier.

```
'(arch x86-64)          ; An annotation with the leading identifier 'arch'.
'(mode 32)              ; An annotation with the leading identifier 'mode'.
(asm-func (foo) (ret))  ; The list to which the above annotations are attached.

'(likely)(foo)          ; An annotation on the same line as its list.

'(arch arm64)

(asm-func (foo) (ret))  ; illegal: There is a blank line between the annotation and its list.

'(arch x86-64)
'(arch arm64)
'(mode 32)
(asm-func (foo) (ret))  ; illegal: Two annotations have the same leading identifier.
```

### Function calls

Most Ruse code is structured grammatically as function calls. These calls consist of a list, where the first element references a function and any remaining elements form the arguments to the function call. When resolving a function call, the list's elements are resolve sequentially, from left to right.

```
(+ 2 3)                 ; A function call to '+' (arithmetic addition), with the arguments '2' and '3'.
(* (len "foo") (- 3 1)) ; A function call to '*' (arithmetic multiplication), with the arguments 3 (the result of calling the 'len' function on the string '"foo"') and 2 (the result of calling subtraction on '3' and '1').
```

## File grammar

The grammar of a Ruse source file includes some structural requirements.

### Package declaration

Aside from comments and whitespace, the first element in a Ruse source file must be the package declaration. This consists of a list with two elements, both of which are identifiers. The first identifier must be `package`. The second is the package name. Ruse source files must have exactly one package declaration.

When compiling multiple files into a package, they must all use the same package name in their package declarations.

When compiling a package, exactly one file may apply annotations to its package declaration.

The set of annotations supported on a package declaration is:

- `base-address`: This annotation may only be used on package main. It sets the memory address that should be used for the `main` function. The annotation must have one other element, which must resolve to an unsigned integer constant.
- `sections`: This annotation may only be used on package main. It sets the set of program sections that will be included in the compiled binary. The annotation must have one or more other elements, each of which must resolve to a section object. The order in which the sections appear will govern the order in which the compiled program sections will appear.

```
(package foo)                          ; Package delcaration for package 'foo'.

(+ 1 2)
(package bar)                          ; illegal: The package delcaration must be the first grammar clause.
```

### Import declarations

Any number of import declarations can follow the package declaration, before any statements. Import declarations are not allowed after any statements. Import declarations are used to load other Ruse packages and add their objects to the file namespace.

Import declarations support both individual and group forms. The individual form performs a single import, whereas the group form performs multiple imports.

Each import consists of a fully qualified package path in a string literal, optionally prefixed with a binding identifier. If a binding identifier is provided, then that identifier can be used to reference the object namespace of the imported package. If no binding identifier is provided, then the package's object namespace is referenced using the identifier in the imported package's package declaration.

The interpretation of the import path is implementation-dependent but it is typically a substring of the full file name of the compiled package and may be relative to a repository of installed packages.

Implementation restriction: A compiler may restrict import paths to non-empty strings using only characters belonging to [Unicode's](https://www.unicode.org/versions/Unicode6.3.0/) L, M, N, P, and S general categories (the Graphic characters without spaces) and may also exclude the characters `!"#$%&'()*,:;<=>?[\]^\`{|}` and the Unicode replacement character U+FFFD.

The single import form is a list containing either two or three elements. The first element is the identifier `import`. The optional second element is the binding identifier. The final element is a string literal specifying the import path.

The group import form is a list containing at least two elements. The first element is the identifier `import`. The subsequent elements are lists, each containing either one or two elements. The optional first element is the binding identifier. The final element is a string literal specifying the import path.

Each file can only import each package path once.

There are no valid annotations for import declarations.

```
; Package path "example.com/foo".
(package foo)
(let Name "Smith")

; Package path "example.com/bar".
(package bar)

(package baz)
(import foobar "example.com/foo")      ; Individual import: package 'foo' is imported and bound to the explicit binding identifier 'foobar'.
(import "example.com/bar")             ; Individual import: ackage 'bar' is imported and bound to the implicit binding identifier 'bar' as indicated in package 'bar's package declaration.
(let Name foo.Name)                    ; Qualified identifier to select the 'Name' object within the 'foo' package's object namespace.

(package baz)
(import                                ; Group import with the same effect as the two individual imports above.
	(foobar "example.com/foo")
	("example.com/bar")
```

## Statements

After the package declaration and any import declarations, a Ruse source file may contain any number of statements. Each top-level statement declares a new object in the package scope.

There are three kinds of statement:

1. `let` statements declare a constant.
2. `func` statements declare a function written in Ruse.
3. `asm-func` statements declare a function written in Ruse assembly.

### `Let` statements

A `let` statement declares a constant and binds it to an identifier.

The statement may specify the type of the identifier. If no type is specified, the type of the constant is used.

If the type of the identifier is specified, the constant's type must be assignable to the identifier type.

The `let` statement is a list containing three or more elements. The first element is the identifier `let`. The final element is an expression resolving to one or more constant values. These constant values are evaluated and assigned to preceeding identifiers. The remaining elements are each either an identifier (representing the name to which the constant is bound) or a list of two identifiers. In this second form, the first identifier is the name to which the corresponding constant is bound and the second identifier specifies its type.

The set of annotations supported on a `let` statement is:

- `align`: This annotation can be specified to apply an alignment constraint on the memory address of the constant. That is, the address of the constant will be an exact multiple of the given alignment value. The annotation must have one other element, which must resolve to an unsigned integer constant.
- `arch`: This annotation can be specified to constrain the architectures for which the statement should be considered. The annotation must have one or more other elements, each of which must be an identifier specifying an architecture. If the package is being compiled for an architecture not specified in the annotation, the statement is ignored. The architectures currently supported are `x86`, `x86-64`.
- `section`: This annotation can be specified to select the program section into which this constant should be linked. The annotation must have one other element, which must be an identifier or qualified identifier resolving to a section object.

```
(let a 1)                              ; Assign the untyped integer constant '1' to the identifier 'a', with inherited type 'untyped integer'.
(let (a int) 1)                        ; Assign the untyped integer constant '1' to the identifier 'a', with explicit type 'int'.
(let (a int) (+ 3 4))                  ; Assign the untyped integer constant '7' to the identifier 'a', with explicit type 'int'.

(func (check (x int) bool bool)        ; `check` takes one integer and returns wither it is negative and whether it is zero.
	(let negative (< x 0))
	(let zero (= x 0))
	(return negative zero))
(let negative zero (check 3))          ; Assign the first result from `check` to `negative` and the second result to `zero`. Both have implicit type `bool`.

(let (a int) "foo")                    ; illegal: The untyped string '"foo"' is not assignable to type 'int'.
```

### `func` statements

A `func` statement declares a function and its signature and binds them to an identifier.

The `func` statement is a list containing three or more elements. The first element is the identifier `func`. The second element is a list specifying the function's name and type signature. The remaining elements comprise the function body. These are executed when the function executes.

The function name and signature is a list consisting of an identifier (to which the function is bound), zero or more lists of two identifiers (specifying the function's parameters), and zero or more identifiers (specifying the function's return types).

The set of annotations supported on a `func` statement is:

- `abi`: This annotation can be specified to select the function's binary interface (ABI). A function must have an explicit ABI to be called from an assembly function. The annotation must have one other element, which must resolve to an ABI object.
- `align`: This annotation can be specified to apply an alignment constraint on the memory address of the function. That is, the address of the function will be an exact multiple of the given alignment value. The annotation must have one other element, which must resolve to an unsigned integer constant.
- `arch`: This annotation can be specified to constrain the architectures for which the statement should be considered. The annotation must have one or more other elements, each of which must be an identifier specifying an architecture. If the package is being compiled for an architecture not specified in the annotation, the statement is ignored. The architectures currently supported are `x86`, `x86-64`.
- `section`: This annotation can be specified to select the program section into which this function should be linked. The annotation must have one other element, which must be an identifier or qualified identifier resolving to a section object.

```
(func (foo) (+ 1 2))                     ; A function called 'foo', with no parameters, no return type, and the function body '(+ 1 2)'.
(func (foo (a int)) (+ a 2))             ; A function called 'foo', with one parameter 'a' of type 'int', no return type, and the function body '(+ a 2)'.
(func (foo (a int) (b int) int) (+ a b)) ; A function called 'foo', with parameters 'a' and 'b' of type 'int', return type 'int', and the function body '(+ a b)'.
(func (foo int bool) (return 1 true))    ; A function called 'foo', with no parameters, return types 'int' and 'bool' and the function body '(return 1 true)'.
```

### `asm-func` statements

An `asm-func` statement declares a function implemented in architecture-specific assembly language and its signature and binds them to an identifier.

The `asm-func` statement is a list containing three or more elements. The first element is the identifier `asm-func`. The second element is a list specifying the function's name and type signature. The remaining elements comprise the function body. These are executed when the function executes.

The function name and signature is a list consisting of an identifier (to which the function is bound), zero or more lists of two identifiers (specifying the function's parameters), and zero or more identifiers (specifying the function's return types).

The set of annotations supported on an `asm-func` statement is:

- `abi`: This annotation can be specified to select the function's binary interface (ABI). A function implemented in assembly must have an explicit ABI to be callable from an assembly function. The annotation must have one other element, which must resolve to an ABI object.
- `align`: This annotation can be specified to apply an alignment constraint on the memory address of the function. That is, the address of the function will be an exact multiple of the given alignment value. The annotation must have one other element, which must resolve to an unsigned integer constant.
- `arch`: This annotation can be specified to constrain the architectures for which the statement should be considered. The annotation must have one or more other elements, each of which must be an identifier specifying an architecture. If the package is being compiled for an architecture not specified in the annotation, the statement is ignored. The architectures currently supported are `x86`, `x86-64`.
- `mode`: This annotation can be specified to select the CPU mode targeted by the function's assembly instructions. This may affect which instructions are available and how they will be encoded by the assembler. The annotation must have one other element, which must be an unsigned integer literal. The valid `mode` values will depend on the target architecture. Valid values for architecture `x86` are `16` and `32` (the default). Valid values for architecture `x86-64` are `16`, `32`, and `64` (the default).
- `section`: This annotation can be specified to select the program section into which this function should be linked. The annotation must have one other element, which must be an identifier or qualified identifier resolving to a section object.

```
(asm-func (foo) (mov eax ebx))                     ; A function called 'foo', with no parameters, no return type, and the function body '(mov eax ebx)'.

'(abi (abi (params ebx)))
(asm-func (foo (a int)) (mov eax ebx))             ; A function called 'foo', with one parameter 'a' of type 'int', no return type, and the function body '(mov eax ebx)'.

'(abi (abi (params eax ebx) (result eax)))
(asm-func (foo (a int) (b int) int) (mov eax ebx)) ; A function called 'foo', with parameters 'a' and 'b' of type 'int', return type 'int', and the function body '(mov eax ebx)'.
```

## An example package

Here is a complete Ruse package that implements Hello World in x86-64 assembly for the Linux ABI.

```
(package main)

(import "syscalls")

(func (main)
	(syscalls.Print 1 "Hello, World!\n")
	(syscalls.Exit 0))
```

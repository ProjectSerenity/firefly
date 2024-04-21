# Special forms

Ruse has various special forms. Each behaves like a function. However, special forms do not have a static function signature like normal functions. Some special forms also have an irregular grammar.

## Table of contents

- [`!=`](#-0)
- [`+`](#-1)
- [`-`](#-)
- [`<`](#-2)
- [`<=`](#-3)
- [`<<`](#-4)
- [`=`](#-5)
- [`>`](#-6)
- [`>=`](#-7)
- [`>>`](#-8)
- [`×`](#-9)
- [`÷`](#-10)
- [`abi`](#abi)
- [`and`](#and)
- [`asm-func`](#asm-func)
- [`do`](#do)
- [`func`](#func)
- [`len`](#len)
- [`let`](#let)
- [`or`](#or)
- [`return`](#return)
- [`section`](#section)
- [`size-of`](#size-of)
- [`xor`](#xor)

## `!=`

The `!=` form ('not equal') takes two arguments of compatible types and returns an untyped bool indicating whether the arguments are not equal.

Currently, `!=` only supports numerical arguments.

If both of `!=`'s arguments are constants, its value is calculated at compile time, producing a constant result.

```
(!= 3 4)  ; true
(!= 3 3)  ; false
```

## `+`

The `+` form ('add') takes two or more arguments of compatible types and returns a value of the same type containing their sum.

Calling `+` on number types returns their arithmetic sum. Calling `+` on string types returns the concatenation of the inputs.

If all of `+`'s arguments are constants, its value is calculated at compile time, producing a constant result.

```
(+ 3 4)          ; 7
(+ 3 4 10)       ; 17
(+ "foo" "bar")  ; "foobar"
```

## `-`

The `-` form ('subtract') takes two or more arguments of compatible types, subtracts each value from the value before it, and returns a value of the same type.

Calling `-` on a single signed integer value returns its negation.

If all of `-`'s arguments are constants, its value is calculated at compile time, producing a constant result.

```
(- 3)       ; -3
(- 3 4)     ; -1
(- 10 3 1)  ; 6
```

## `<`

The `<` form ('less than') takes two arguments of compatible types and returns an untyped bool indicating whether the first argument is less than the second.

`<` only supports numerical arguments.

If both of `<`'s arguments are constants, its value is calculated at compile time, producing a constant result.

```
(< 3 4)   ; true
(< 3 3)   ; false
(< 10 4)  ; false
```

## `<=`

The `<=` form ('less than or equal') takes two arguments of compatible types and returns an untyped bool indicating whether the first argument is less than or equal to the second.

`<=` only supports numerical arguments.

If both of `<=`'s arguments are constants, its value is calculated at compile time, producing a constant result.

```
(<= 3 4)   ; true
(<= 3 3)   ; true
(<= 10 4)  ; false
```

## `<<`

The `<<` form ('shift left') takes two arguments. The first argument is any numerical type, the second is an unsigned integer of any size. The result is the same type as the first argument and contains that argument shifted left by the second argument number of bits.

If both of `<<`'s arguments are constants, its value is calculated at compile time, producing a constant result.

```
(<< 3 4)  ; 48
(<< 3 0)  ; 3
```

## `=`

The `=` form ('equal') takes two arguments of compatible types and returns an untyped bool indicating whether the arguments are equal.

Currently, `=` only supports numerical arguments.

If both of `=`'s arguments are constants, its value is calculated at compile time, producing a constant result.

```
(= 3 4)  ; false
(= 3 3)  ; true
```

## `>`

The `>` form ('greater than') takes two arguments of compatible types and returns an untyped bool indicating whether the first argument is greater than the second.

`>` only supports numerical arguments.

If both of `>`'s arguments are constants, its value is calculated at compile time, producing a constant result.

```
(> 3 4)   ; false
(> 3 3)   ; false
(> 10 4)  ; true
```

## `>=`

The `>=` form ('greater than or equal') takes two arguments of compatible types and returns an untyped bool indicating whether the first argument is greater than or equal to the second.

`>=` only supports numerical arguments.

If both of `>=`'s arguments are constants, its value is calculated at compile time, producing a constant result.

```
(>= 3 4)   ; false
(>= 3 3)   ; true
(>= 10 4)  ; true
```

## `>>`

The `>>` form ('shift right') takes two arguments. The first argument is any numerical type, the second is an unsigned integer of any size. The result is the same type as the first argument and contains that argument shifted right by the second argument number of bits.

If both of `>>`'s arguments are constants, its value is calculated at compile time, producing a constant result.

```
(>> 5 1)  ; 2
(>> 3 0)  ; 3
```

## `×`

The `×` form ('multiply') takes two or more arguments of compatible types and returns a value of the same type containing their product.

`×` only supports numerical arguments.

If all of `×`'s arguments are constants, its value is calculated at compile time, producing a constant result.

```
(× 3 4)     ; 12
(× 3 4 10)  ; 120
```

## `÷`

The `÷` form ('divide') takes two arguments of compatible types and returns a value of the same type containing the first argument divided by the second.

`÷` only supports numerical arguments.

If both of `÷`'s arguments are constants, its value is calculated at compile time, producing a constant result.

```
(÷ 13 4)  ; 3
(÷ 12 4)  ; 3
```

## `abi`

The `abi` form ('define ABI') is used to define an ABI construct. The arguments to the `abi` form are a series of lists, each of which is a key followed by values. The lists can be included in any order. The defined keys are:

- `params`: zero or more identifiers representing CPU registers in the target microarchitecture. The resulting ABI will place the fisrt 'n' arguments in the given sequence of registers. Any subsequent values are passed on the stack.
- `result`: zero or more identifiers representing CPU registers in the target microarchitecture. The resulting ABI will place the first 'n' results in the given sequence of registers. Any subsequent values are passed on the stack.
- `scratch`: zero or more identifiers representing CPU registers in the target microarchitecture. The resulting ABI will treat the given registers as caller-saved and freely usable by the function.
- `unused`: zero or more identifiers representing CPU registers in the target microarchitecture. The resulting ABI will treat the given registers as callee-saved.

If no `unused` list is provided, it is calculated automatically from the other lists. Any other omitted list is treated as an empty list.

It is a compile time error if any list is included more than once.

It is a compile time error if the `unused` list contains any registers included in any other lists.

```
(let System-V-x86-64 (abi
	(params rdi rsi rdx rcx r8 r9)
	(result rax rdx)
	(scratch rax rdi rsi rdx rcx r8 r9 r10 r11)
	(unused rbx rsp rbp r12 r13 r14 r15)))
```

## `and`

The `and` form takes two or more arguments of compatible types.

If the arguments are numerical types, the result has the same type and contains the bitwise AND of the arguments. If the arguments are boolean types, the result is an untyped bool and contains the logical AND of the arguments.

If all of `and`'s arguments are constants, its value is calculated at compile time, producing a constant result.

```
(and 3 7 10)           ; 3
(and true (= 3 4))     ; false
(and (= 3 3) (< 3 4))  ; true
```

## `asm-func`

The `asm-func` form ('define assembly function') takes two or more arguments and defines a new function, written in Ruse assembly. For more details, see the [grammar](./grammar.md).

```
(asm-func (foo) (mov eax ebx))                      ; A function called 'foo', with no parameters, no return type, and the function body '(mov eax ebx)'.

'(abi (abi (params ebx)))
(asm-func (foo (a int)) (mov eax ebx))              ; A function called 'foo', with one parameter 'a' of type 'int', no return type, and the function body '(mov eax ebx)'.

'(abi (abi (params eax ebx) (result eax)))
(asm-func (foo (a int) (b int) int) (mov eax ebx))  ; A function called 'foo', with parameters 'a' and 'b' of type 'int', return type 'int', and the function body '(mov eax ebx)'.
```

## `do`

The `do` form ('inline block') takes one or more arguments, each of which is an expression. The arguments are evaluated as normal, returning the result of the final expression.

Each `do` form makes another scope, so declarations made within a `do` are only in scope within that `do`.

The `do` form allows multiple expressions to be used in a place where only one expression is allowed.

```
(func (foo (x int) int)
	(let temp (do
		(let sum (+ x x))      ; Note that `sum` is only in scope for the rest of the `do` form.
		(let product (* x x))
		(<< product sum)))
	(+ temp x))
```

## `func`

The `func` form ('define function') takes two or more arguments and defines a new function. For more details, see the [grammar](./grammar.md).

```
(func (foo) (+ 1 2))                      ; A function called 'foo', with no parameters, no return type, and the function body '(+ 1 2)'.
(func (foo (a int)) (+ a 2))              ; A function called 'foo', with one parameter 'a' of type 'int', no return type, and the function body '(+ a 2)'.
(func (foo (a int) (b int) int) (+ a b))  ; A function called 'foo', with parameters 'a' and 'b' of type 'int', return type 'int', and the function body '(+ a b)'.
(func (foo int bool) (return 1 true))     ; A function called 'foo', with no parameters, return types 'int' and 'bool' and the function body '(return 1 true)'.
```

## `len`

The `len` form ('length') takes one argument of string/array type and returns an utyped integer containing the length of the argument.

If `len`'s argument is an array or a constant string, its value is calculated at compile time, producing a constant result.

```
(len "foo")  ; 3

(let x (array/uint8 1 17))
(len x)  ; 2
```

## `let`

The `let` form ('define constant') takes two arguments and defines a new constant. For more details, see the [grammar](./grammar.md).

```
(let a 1)              ; Assign the untyped integer constant '1' to the identifier 'a', with inherited type 'untyped integer'.
(let (a int) 1)        ; Assign the untyped integer constant '1' to the identifier 'a', with explicit type 'int'.
(let (a int) (+ 3 4))  ; Assign the untyped integer constant '7' to the identifier 'a', with explicit type 'int'.
```

## `or`

The `or` form takes two or more arguments of compatible types.

If the arguments are numerical types, the result has the same type and contains the bitwise OR of the arguments. If the arguments are boolean types, the result is an untyped bool and contains the logical OR of the arguments.

If all of `or`'s arguments are constants, its value is calculated at compile time, producing a constant result.

```
(or 3 7 10)           ; 15
(or true (= 3 4))     ; true
(or (= 3 3) (< 3 4))  ; true
```

## `return`

The `return` form takes zero or one arguments. It can be used to return early from the innermost parent function.

If the innermost parent function has a result type, then one argument with a compatible type must be provided to `return`. If the innermost parent function has no result type, then there must be no argument to `return`.

```
(func (foo int)
	(return 3))
```

## `section`

The `section` form ('define section') is used to define a program section construct. The arguments to the `section` form are a series of lists, each of which is a key followed by values. The lists can be included in any order. The defined keys are:

- `name`: a string literal specifying the section's name.
- `fixed-address`: an integer value specifying the section's address in memory. If no `fixed-address` list is included, the section is placed after the previous section.
- `permissions`: an identifier specifying the permissions applied to the section. The identifier consists of three characters (`rwx`) to indicate each permission provided. Any letter can be replaced with an underscore to deny the corresponding permission.

It is a compile time error if any list is included more than once.

It is a compile time error if the `name` or `permissions` lists are absent.

```
(let Code (section
	(name "code")
	(permissions r_x)))  ; read and execute but not write permissions.
```

## `size-of`

The `size-of` form ('size of') takes one argument of any type or a type name and returns an untyped integer containing the value/type's size in bytes.

`size-of` is always a constant expression calculated at compile time, producing a constant result.

```
(size-of uint8)           ; 1
(size-of array/3/uint16)  ; 6
(let (x uint32) (+ 1 2))
(size-of x)               ; 4
```

## `xor`

The `xor` form takes two or more arguments of compatible types.

If the arguments are numerical types, the result has the same type and contains the bitwise XOR of the arguments. If the arguments are boolean types, the result is an untyped bool and contains the logical XOR of the arguments.

If all of `xor`'s arguments are constants, its value is calculated at compile time, producing a constant result.

```
(xor 3 7 10)           ; 14
(xor true (= 3 4))     ; true
(xor (= 3 3) (< 3 4))  ; false
```

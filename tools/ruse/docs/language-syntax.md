# Language syntax

## Syntax basics

The Ruse language uses a LISP-like syntax, consisting of lists, atoms, and annotations. A commented example follows (comments starting with a semicolon, `;`):

```
'(base-address 10_000)   ; An annotation on the package statement, using an integer constant with an underscore separator. Note the leading quote to turn the list into an annotation.
(package main)           ; A package statement, which is mandatory. Annotations bind onto to list that immediately follows it, in this case the package statement.

; hello-world is an untyped string constant.
;
; Note that it contains the terminating newline. Ruse
; strings are length-prefixed and not NULL terminated.
;
(let hello-world "Hello, world!\n")    ; A let statement declares a new package-scope constant. In this case, the type is inferred from the type.

; print-hello-world is a function written in x86-64
; assembly. We don't specify the architecture explicitly
; here, so the compiler will attempt to target whichever
; architecture is currently being targeted.
;
; A smarter version of this function would use the ABI
; functionality to make the initial MOV instructions
; explicit parameters, managed by the compiler. As is,
; the type signature is `(func)`; a nullary function.
;
; Note that we declare the architecture explicitly.
;
'(arch x86-64)
(asm-func (print-hello-world)
	(mov eax 1)                            ; sys_write
	(mov rdi 1)                            ; unsigned int fd stdout
	(mov rsi (string-pointer hello-world)) ; const char *buf
	(mov rdx (len hello-world))            ; size_t count
	(syscall)                              ; write(1, "Hello, world!\n", 14)

	(ret))

'(abi (abi (params rdi)))
(asm-func (Exit (code int))
	(mov eax 60) ; sys_exit
	(syscall)    ; exit(code)

	(ret))

; main is the entry point for an executable package.
;
; Unlike print-hello-world, main is written in Ruse.
;
(func (main)
	; At link-time, the linker will insert the final
	; address of print-hello-world into this instruction
	; as a relative address.
	(print-hello-world)

	(exit 0))    ; exit(0)

```

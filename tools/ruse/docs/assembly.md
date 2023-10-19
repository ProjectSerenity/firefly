# Assembly code

## Assembly functions

Ruse supports writing functions in architecture-specific assembly language, using an `asm-func` function. This is immediately customised using several annotations:

- `abi` is used to document the ABI that an assembly function uses.
- `arch` specifies which architecture the function targets. If the package is being compiled for another architecture, then the function will be ignored.
- `mode` specifies the CPU mode that the assembler should target when assembling the function.
- `section` specifies in which program section the assembled function should be included by the linker.

The assembly syntax is a light variant of the architecture-specific syntax. For example, a simple x86-64 function might look like this:

```
'(abi (abi
	(params rdi rsi rdx r10 r8 r9)
	(result rax)))
'(arch x86-64)
(asm-func (exit (code int))
	'loop         ; A label, which we can jump to.
	(mov eax 60)  ; Syscall 60 (sys_exit).
	(syscall)

	(jmp 'loop)   ; Loop forever if sys_exit fails.
	(ret))
```

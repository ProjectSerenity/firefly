//; Implement context switching between threads.

//; The Rust signature of this function is:
//;
//;     fn switch_stack(current_stack_pointer: *mut u64, new_stack_pointer: *mut u64);
//;
//; switch_stack saves the current thread context
//; into the current stack and then loads the new
//; thread context from the new stack.
//;
//; The thread context consists of the following
//; details, as specified in the System V ABI
//; (https://www.uclibc.org/docs/psABI-x86_64.pdf,
//; page 21):
//;
//; - The instruction Pointer (saved and restored by CALL/RET)
//; - The stack sointer (RSP)
//; - The callee-saved registers (RBP, RBX, R12-R15)
//; - RFLAGS
//;
//; When switch_stack is called to start a new thread,
//; it will 'return' to one of the following:
//;
//; - start_kernel_thread
//;
.global switch_stack
switch_stack:
	//; Save the callee-saved registers into the current stack.
	push rbp
	push rbx
	push r12
	push r13
	push r14
	push r15
	pushfq

	//; Swap stacks.
	mov [rdi], rsp   //; Save the old stack pointer into the current stack.
	mov rsp, [rsi]   //; Load the new stack pointer (given as a parameter).

	//; Load the callee-saved registers from the new stack.
	popfq
	pop r15
	pop r14
	pop r13
	pop r12
	pop rbx
	pop rbp

	//; Resume the new thread
	ret

//; The Rust signature of this function is:
//;
//;     fn replace_stack(new_stack_pointer: *mut u64);
//;
//; replace_stack loads the new thread context from
//; the new stack.
//;
//; This is identical to switch_stack, but without
//; saving or using the current stack. This is for
//; when we switch away from an exiting thread. We
//; drop the thread's resources immediately before
//; calling replace_stack, so it's important that
//; we don't use the current stack, as there's a
//; remote possibility that it could be allocated
//; to another thread on another CPU and being used.
//; However, we don't unmap old stacks, so there's
//; no risk of a page fault.
//;
//; The thread context consists of the following
//; details, as specified in the System V ABI
//; (https://www.uclibc.org/docs/psABI-x86_64.pdf,
//; page 21):
//;
//; - The instruction Pointer (saved and restored by CALL/RET)
//; - The stack sointer (RSP)
//; - The callee-saved registers (RBP, RBX, R12-R15)
//; - RFLAGS
//;
//; When switch_stack is called to start a new thread,
//; it will 'return' to one of the following:
//;
//; - start_kernel_thread
//;
.global replace_stack
replace_stack:
	//; Load the new stack pointer (given as a parameter).
	mov rsp, [rdi]

	//; Load the callee-saved registers from the new stack.
	popfq
	pop r15
	pop r14
	pop r13
	pop r12
	pop rbx
	pop rbp

	//; Resume the new thread
	ret

//; The Rust signature of this function is:
//;
//;     fn start_kernel_thread() -> !;
//;
//; start_kernel_thread pops the entry point off the
//; stack and calls it. If the entry point function
//; returns, an invalid instruction exception will
//; be triggered.
//;
.global start_kernel_thread
start_kernel_thread:
	//; Enable interrupts.
	sti

	//; Clear the frame pointer so that any debuggers
	//; will treat this as the root of the stack trace.
	xor rbp, rbp

	//; Pop and call the entry point. The entry point
	//; should never return.
	pop rax
	call rax

	//; If the entry point returned, we trigger an
	//; invalid instruction exception so the bug gets
	//; found and fixed quickly. Anything else would
	//; cause more problems in the long run.
	//;
	//; The ud2 instruction just triggers an invalid
	//; instruction exception and is otherwise a NOP.
	ud2

//; The Rust signature of this function is:
//;
//;     fn start_user_thread() -> !;
//;
//; start_user_thread pops the entry point off the
//; stack and calls it using sysexit. If the entry
//; point function returns, an invalid instruction
//; exception will be triggered.
//;
.global start_user_thread
start_user_thread:
	//; Clear the frame pointer so that any debuggers
	//; will treat this as the root of the stack trace.
	xor rbp, rbp

	//; Pop the entry point and RSP. The entry point
	//; should never return.
	pop rdx //; RIP
	pop rcx //; RSP
	sysexitq

	//; If the entry point returned, we trigger an
	//; invalid instruction exception so the bug gets
	//; found and fixed quickly. Anything else would
	//; cause more problems in the long run.
	//;
	//; The ud2 instruction just triggers an invalid
	//; instruction exception and is otherwise a NOP.
	ud2

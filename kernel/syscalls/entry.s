//; Implement the syscall entry point.

//; Constants for offsets into the CPU-local data,
//; which is offset from the GS register. If these
//; offsets change in the CPU-local data, we need
//; to update them here too.
.set SYSCALL_STACK, 8  //; syscall_stack_pointer
.set USER_STACK, 16    //; user_stack_pointer

//; This is the entry point, which is called when a user process
//; uses the SYSCALL instruction.
//;
//; Firefly uses the following ABI:
//;
//; - RAX: Syscall number
//; - RDI: Argument 1
//; - RSI: Argument 2
//; - RDX: Argument 3
//; - R10: Argument 4
//; - R8:  Argument 5
//; - R9:  Argument 6
//; - RCX: Return address (set by the SYSCALL instruction)
//; - R11: User RFLAGS (set by the SYSCALL instruction)
//;
//; - The user values in RCX and R11 are destroyed by SYSCALL.
//; - The user values in RAX and RDX are not preserved.
//; - All other registers are preserved by the kernel.
//;
//; - RAX: Returned value
//; - RDX: Returned error
//;
.global syscall_entry
syscall_entry:
	//; Interrupts have been disabled using the RFLAGS
	//; mask in IA32_FMASK.
	//;
	//; Switch to the syscall stack after saving
	//; our current stack pointer, which we restore
	//; just before we return to user space.
	mov gs:[USER_STACK], rsp
	mov rsp, gs:[SYSCALL_STACK]

	//; Store the saved registers onto the stack, so
	//; they can then be passed to the syscall handler.
	//;
	//; The values are pushed in reverse order to fill
	//; a SavedRegisters structure. Note that we cannot
	//; save the original values in RAX or R11, as the
	//; SYSCALL instruction has already destroyed them.
	push r11  // RFLAGS
	push r15
	push r14
	push r13
	push r12
	//; R11 already destroyed.
	push r10
	push r9
	push r8
	push gs:[USER_STACK]  // RSP
	push rcx              // RIP
	push rbp
	push rdi
	push rsi
	push rdx
	//; RCX already destroyed.
	push rbx
	push rax

	//; Prepare the arguments for syscall_handler,
	//; according to the System V 64-bit ABI, as
	//; documented in https://www.uclibc.org/docs/psABI-x86_64.pdf,
	//; page 21:
	//;
	//;     arg1: RDI
	//;     arg2: RSI
	//;     arg3: RDX
	//;     arg4: RCX
	//;     arg5: R8
	//;     arg6: R9
	//;     syscall_num: RSP-0
	//;     registers: RSP-8

	//; arg1 is already in RDI.
	//; arg2 is already in RSI.
	//; arg3 is already in RDX.
	mov rcx, r10  //; Copy arg4 from R10 to RCX.
	//; arg5 is already in R8.
	//; arg6 is already in R9.
	push rsp     //; Push a pointer to the saved registers.
	push rax     //; Push the syscall number.

	call syscall_handler

	//; Restore registers from the stack.
	//;
	//; The values are popped in order from the
	//; SavedRegisters structure on the stack we
	//; prepared earlier.
    //;
	//; syscall_handler places its return value
	//; in RAX and its return error in RDX. We
	//; return these to the user thread in the
	//; same way, so we do not restore RAX or
	//; RDX.
	//;
	//; We also skip RCX and R11, as they are
	//; used to restore the user instruction
	//; pointer and RFLAGS with SYSRET.
	add rsp, 16  //; Skip over the saved registers pointer and syscall number.
	add rsp, 8   //; Skip RAX (which has the return value).
	pop rbx
	//; RCX is not preserved.
	add rsp, 8   //; Skip RDX (which has the return error).
	pop rsi
	pop rdi
	pop rbp
	pop rcx      //; RIP.
	add rsp, 8   //; Skip RSP, which is restored from CPU-local data below.
	pop r8
	pop r9
	pop r10
	//; R11 is not preserved.
	pop r12
	pop r13
	pop r14
	pop r15
	pop r11      //; RFLAGS.

	//; Restore the user stack.
	//;
	//; We need to ensure interrupts are disabled
	//; or an interrupt between restoring the
	//; stack and calling SYSRET would result in
	//; the interrupt handler using the userspace
	//; stack, which could leak kernel data to
	//; userspace.
	cli
	mov rsp, gs:[USER_STACK]

	//; Return to userspace.
	sysretq

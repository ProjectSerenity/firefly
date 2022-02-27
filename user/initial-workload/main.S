//; Copyright 2022 The Firefly Authors.
//;
//; Use of this source code is governed by a BSD 3-clause
//; license that can be found in the LICENSE file.

.intel_syntax noprefix

.global main
main:
	//; Call exit_thread (syscall 0).
	xor rax, rax
	syscall

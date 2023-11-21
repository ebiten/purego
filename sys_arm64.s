// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebitengine Authors

//go:build darwin || freebsd || linux || windows

#include "textflag.h"
#include "go_asm.h"
#include "funcdata.h"

#define STACK_SIZE 64
#define PTR_ADDRESS (STACK_SIZE - 8)

// syscall12X calls a function in libc on behalf of the syscall package.
// syscall12X takes a pointer to a struct like:
// struct {
//	fn    uintptr
//	a1    uintptr
//	a2    uintptr
//	a3    uintptr
//	a4    uintptr
//	a5    uintptr
//	a6    uintptr
//	a7    uintptr
//	a8    uintptr
//	a9    uintptr
//	a10    uintptr
//	a11    uintptr
//	a12    uintptr
//	r1    uintptr
//	r2    uintptr
//	err   uintptr
// }
// syscall12X must be called on the g0 stack with the
// C calling convention (use libcCall).
GLOBL ·syscall12XABI0(SB), NOPTR|RODATA, $8
DATA ·syscall12XABI0(SB)/8, $syscall12X(SB)
TEXT syscall12X(SB), NOSPLIT, $0
	SUB  $STACK_SIZE, RSP     // push structure pointer
	MOVD R0, PTR_ADDRESS(RSP)
	MOVD R0, R9

	FMOVD syscall12Args_f1(R9), F0 // f1
	FMOVD syscall12Args_f2(R9), F1 // f2
	FMOVD syscall12Args_f3(R9), F2 // f3
	FMOVD syscall12Args_f4(R9), F3 // f4
	FMOVD syscall12Args_f5(R9), F4 // f5
	FMOVD syscall12Args_f6(R9), F5 // f6
	FMOVD syscall12Args_f7(R9), F6 // f7
	FMOVD syscall12Args_f8(R9), F7 // f8

	MOVD syscall12Args_a1(R9), R0 // a1
	MOVD syscall12Args_a2(R9), R1 // a2
	MOVD syscall12Args_a3(R9), R2 // a3
	MOVD syscall12Args_a4(R9), R3 // a4
	MOVD syscall12Args_a5(R9), R4 // a5
	MOVD syscall12Args_a6(R9), R5 // a6
	MOVD syscall12Args_a7(R9), R6 // a7
	MOVD syscall12Args_a8(R9), R7 // a8

	MOVD syscall12Args_a9(R9), R10
	MOVD R10, 0(RSP)                // push a9 onto stack
	MOVD syscall12Args_a10(R9), R10
	MOVD R10, 8(RSP)                // push a10 onto stack
	MOVD syscall12Args_a11(R9), R10
	MOVD R10, 16(RSP)               // push a11 onto stack
	MOVD syscall12Args_a12(R9), R10
	MOVD R10, 24(RSP)               // push a12 onto stack

	MOVD syscall12Args_fn(R9), R10 // fn
	BL   (R10)

	MOVD  PTR_ADDRESS(RSP), R2     // pop structure pointer
	ADD   $STACK_SIZE, RSP
	MOVD  R0, syscall12Args_r1(R2) // save r1
	FMOVD F0, syscall12Args_r2(R2) // save r2
	RET

// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebitengine Authors

//go:build darwin || linux

/*
trampoline for emulating required C functions for cgo in go (see cgo.go)
(we convert cdecl calling convention to go and vice-versa)

Since we're called from go and call into C we can cheat a bit with the calling conventions:
 - in go all the registers are caller saved
 - in C we have a couple of callee saved registers

=> we can use BX, R12, R13, R14, R15 instead of the stack

C Calling convention cdecl used here (we only need integer args):
1. arg: DI
2. arg: SI
3. arg: DX
4. arg: CX
5. arg: R8
6. arg: R9
We don't need floats with these functions -> AX=0
return value will be in AX
*/
#include "textflag.h"
#include "go_asm.h"

// these trampolines map the gcc ABI to Go ABI and then calls into the Go equivalent functions.

TEXT x_cgo_init_trampoline(SB), NOSPLIT, $16
	MOVQ DI, 0(SP)
	MOVQ SI, 8(SP)
	CALL ·x_cgo_init(SB)
	RET

TEXT x_cgo_thread_start_trampoline(SB), NOSPLIT, $8
	MOVQ DI, 0(SP)
	CALL ·x_cgo_thread_start(SB)
	RET

TEXT x_cgo_setenv_trampoline(SB), NOSPLIT, $8
	MOVQ DI, 0(SP)
	CALL ·x_cgo_setenv(SB)
	RET

TEXT x_cgo_unsetenv_trampoline(SB), NOSPLIT, $8
	MOVQ DI, 0(SP)
	CALL ·x_cgo_unsetenv(SB)
	RET

TEXT x_cgo_notify_runtime_init_done_trampoline(SB), NOSPLIT, $0
	CALL ·x_cgo_notify_runtime_init_done(SB)
	RET

// func setg_trampoline(setg uintptr, g uintptr)
TEXT ·setg_trampoline(SB), NOSPLIT, $0-16
	MOVQ G+8(FP), DI
	MOVQ setg+0(FP), AX
	CALL AX
	RET

TEXT threadentry_trampoline(SB), NOSPLIT, $16
	MOVQ DI, 0(SP)
	CALL ·threadentry(SB)
	MOVQ 8(SP), AX
	RET

TEXT ·call5(SB), NOSPLIT, $0-0
	MOVQ fn+0(FP), AX
	MOVQ a1+8(FP), DI
	MOVQ a2+16(FP), SI
	MOVQ a3+24(FP), DX
	MOVQ a4+32(FP), CX
	MOVQ a5+40(FP), R8
	CALL AX
	MOVQ AX, ret+48(FP)
	RET

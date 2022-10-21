// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebitengine Authors

//go:build darwin || linux
// +build darwin linux

#include "textflag.h"
#include "go_asm.h"

// these trampolines map the gcc ABI to Go ABI and then calls into the Go equivalent functions.

// TODO: put <> to make these private
TEXT x_cgo_init_trampoline(SB), NOSPLIT, $0-0
	MOVD R0, 8(RSP)
	MOVD R1, 16(RSP)
	CALL ·x_cgo_init(SB)
	RET

TEXT x_cgo_thread_start_trampoline(SB), NOSPLIT, $0-0
	MOVD R0, 8(RSP)
	CALL ·x_cgo_thread_start(SB)
	RET

TEXT x_cgo_setenv_trampoline(SB), NOSPLIT, $0-0
	MOVD R0, 8(RSP)
	CALL ·x_cgo_setenv(SB)
	RET

TEXT x_cgo_unsetenv_trampoline(SB), NOSPLIT, $0-0
	MOVD R0, 8(RSP)
	CALL ·x_cgo_unsetenv(SB)
	RET

TEXT x_cgo_notify_runtime_init_done_trampoline(SB), NOSPLIT, $0-0
	CALL ·x_cgo_notify_runtime_init_done(SB)
	RET

// func setg_trampoline(setg uintptr, g uintptr)
TEXT ·setg_trampoline(SB), NOSPLIT, $0-16
	MOVD G+8(FP), R0
	MOVD setg+0(FP), R1
	CALL R1
	RET

TEXT threadentry_trampoline(SB), NOSPLIT, $0-0
	MOVD R0, 8(RSP)
	CALL ·threadentry(SB)
	MOVD $0, R0           // TODO: get the return value from threadentry
	RET

TEXT ·call5(SB), NOSPLIT, $0-0
	MOVD fn+0(FP), R6
	MOVD a1+8(FP), R0
	MOVD a2+16(FP), R1
	MOVD a3+24(FP), R2
	MOVD a4+32(FP), R3
	MOVD a5+40(FP), R4
	CALL R6
	MOVD R0, ret+48(FP)
	RET

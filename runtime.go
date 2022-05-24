// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebiten Authors

//go:build darwin
// +build darwin

package purego

import (
	"unsafe"
)

//go:linkname syscall_syscallX syscall.syscallX
func syscall_syscallX(fn, a1, a2, a3 uintptr) (r1, r2, err uintptr)

//go:linkname runtime_libcCall runtime.libcCall
//go:linkname runtime_entersyscall runtime.entersyscall
//go:linkname runtime_exitsyscall runtime.exitsyscall
func runtime_libcCall(fn, arg unsafe.Pointer) int32 // from runtime/sys_libc.go
func runtime_entersyscall()                         // from runtime/proc.go
func runtime_exitsyscall()                          // from runtime/proc.go

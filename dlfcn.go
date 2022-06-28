// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebiten Authors

//go:build darwin
// +build darwin

package purego

import (
	"runtime"
	"unsafe"
)

const RTLD_GLOBAL = 0x8

const RTLD_DEFAULT = ^uintptr(1)

// hasSuffix tests whether the string s ends with suffix.
func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func cString(name string) *byte {
	if hasSuffix(name, "\x00") {
		return &(*(*[]byte)(unsafe.Pointer(&name)))[0]
	}
	var b = make([]byte, len(name)+1)
	copy(b, name)
	return &b[0]
}

func Dlopen(name string, mode int) uintptr {
	bs := cString(name)
	ret, _, _ := SyscallN(dlopenABI0, uintptr(unsafe.Pointer(bs)), uintptr(mode), 0)
	runtime.KeepAlive(bs)
	return ret
}

func Dlsym(handle uintptr, name string) uintptr {
	bs := cString(name)
	ret, _, _ := SyscallN(dlsymABI0, handle, uintptr(unsafe.Pointer(bs)), 0)
	runtime.KeepAlive(bs)
	return ret
}

// these functions exist in dlfcn_stubs.s and are calling C functions linked to in dlfcn_GOOS.go
var dlopenABI0 uintptr
var dlsymABI0 uintptr

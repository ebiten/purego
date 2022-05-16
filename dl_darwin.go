// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebiten Authors

package purego

import (
	"runtime"
	"strings"
	"unsafe"
)

const RTLD_DEFAULT = ^uintptr(1)

func cString(name string) *byte {
	if strings.HasSuffix(name, "\x00") {
		return &[]byte(name)[0]
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

//go:cgo_import_dynamic _dlopen dlopen "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic _dlsym dlsym "/usr/lib/libSystem.B.dylib"

var dlopenABI0 uintptr

func dlopen() // implemented in assembly

var dlsymABI0 uintptr

func dlsym() // implemented in assembly

// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebitengine Authors

//go:build darwin
// +build darwin

package purego

import (
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego/internal/strings"
)

const RTLD_GLOBAL = 0x8

const RTLD_DEFAULT = ^uintptr(1)

// Dlopen examines the dynamic library or bundle file specified by path. If the file is compatible
// with the current process and has not already been loaded into the
// current process, it is loaded and linked. After being linked, if it contains
// any initializer functions, they are called, before Dlopen
// returns. It returns a handle that can be used with Dlsym and Dlclose.
// A second call to Dlopen with the same path will return the same handle, but the internal
// reference count for the handle will be incremented. Therefore, all
// Dlopen calls should be balanced with a Dlclose call.
func Dlopen(path string, mode int) uintptr {
	bs := strings.CString(path)
	ret, _, _ := SyscallN(dlopenABI0, uintptr(unsafe.Pointer(bs)), uintptr(mode), 0)
	runtime.KeepAlive(bs)
	return ret
}

// Dlsym takes a "handle" of a dynamic library returned by Dlopen and the symbol name.
// It returns the address where that symbol is loaded into memory. If the symbol is not found,
// in the specified library or any of the libraries that were automatically loaded by Dlopen
// when that library was loaded, Dlsym returns zero.
func Dlsym(handle uintptr, name string) uintptr {
	bs := strings.CString(name)
	ret, _, _ := SyscallN(dlsymABI0, handle, uintptr(unsafe.Pointer(bs)), 0)
	runtime.KeepAlive(bs)
	return ret
}

// Dlerror returns a human-readable string describing the most recent error that
// occurred from Dlopen, Dlsym or Dlclose since the last call to Dlerror. It
// returns an empty string if no errors have occurred since initialization or
// since it was last called.
func Dlerror() string {
	// msg is only valid until the next call to Dlerror
	// which is why it gets copied into a Go string
	msg, _, _ := SyscallN(dlerrorABI0)
	return strings.GoString(msg)
}

// Dlclose decrements the reference count on the dynamic library handle.
// If the reference count drops to zero and no other loaded libraries
// use symbols in it, then the dynamic library is unloaded.
// Dlclose returns false on success, and true on error.
func Dlclose(handle uintptr) bool {
	ret, _, _ := SyscallN(dlcloseABI0, handle)
	return ret != 0
}

// these functions exist in dlfcn_stubs.s and are calling C functions linked to in dlfcn_GOOS.go
// the indirection is necessary because a function is actually a pointer to the pointer to the code.
// sadly, I do not know of anyway to remove the assembly stubs entirely because //go:linkname doesn't
// appear to work if you link directly to the C function on darwin arm64.

//go:linkname dlopen dlopen
var dlopen uintptr
var dlopenABI0 = uintptr(unsafe.Pointer(&dlopen))

//go:linkname dlsym dlsym
var dlsym uintptr
var dlsymABI0 = uintptr(unsafe.Pointer(&dlsym))

//go:linkname dlclose dlclose
var dlclose uintptr
var dlcloseABI0 = uintptr(unsafe.Pointer(&dlclose))

//go:linkname dlerror dlerror
var dlerror uintptr
var dlerrorABI0 = uintptr(unsafe.Pointer(&dlerror))

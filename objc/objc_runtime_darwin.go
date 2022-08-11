// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebitengine Authors

// Package objc is a low-level pure Go objective-c runtime. This package is easy to use incorrectly, so it is best
// to use a wrapper that provides the functionality you need in a safer way.
package objc

import (
	"fmt"
	"reflect"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/internal/strings"
)

//TODO: support try/catch?
//https://stackoverflow.com/questions/7062599/example-of-how-objective-cs-try-catch-implementation-is-executed-at-runtime

var (
	objc = purego.Dlopen("/usr/lib/libobjc.A.dylib", purego.RTLD_GLOBAL)

	objc_msgSend              = purego.Dlsym(objc, "objc_msgSend")
	objc_msgSendSuper2        = purego.Dlsym(objc, "objc_msgSendSuper2")
	objc_getClass             = purego.Dlsym(objc, "objc_getClass")
	objc_allocateClassPair    = purego.Dlsym(objc, "objc_allocateClassPair")
	objc_registerClassPair    = purego.Dlsym(objc, "objc_registerClassPair")
	sel_registerName          = purego.Dlsym(objc, "sel_registerName")
	class_getSuperclass       = purego.Dlsym(objc, "class_getSuperclass")
	class_getInstanceVariable = purego.Dlsym(objc, "class_getInstanceVariable")
	class_addMethod           = purego.Dlsym(objc, "class_addMethod")
	class_addIvar             = purego.Dlsym(objc, "class_addIvar")
	ivar_getOffset            = purego.Dlsym(objc, "ivar_getOffset")
	object_getClass           = purego.Dlsym(objc, "object_getClass")
)

// ID is an opaque pointer to some Objective-C object
type ID uintptr

// Class returns the class of the object.
func (id ID) Class() Class {
	ret, _, _ := purego.SyscallN(object_getClass, uintptr(id))
	return Class(ret)
}

// Send is a convenience method for sending messages to objects.
func (id ID) Send(sel SEL, args ...interface{}) ID {
	tmp := createArgs(id, sel, args...)
	ret, _, _ := purego.SyscallN(objc_msgSend, tmp...)
	return ID(ret)
}

// objc_super data structure is generated by the Objective-C compiler when it encounters the super keyword
// as the receiver of a message. It specifies the class definition of the particular superclass that should
// be messaged.
type objc_super struct {
	receiver   ID
	superClass Class
}

// SendSuper is a convenience method for sending message to object's super
func (id ID) SendSuper(sel SEL, args ...interface{}) ID {
	var super = &objc_super{
		receiver:   id,
		superClass: id.Class(),
	}
	tmp := createArgs(0, sel, args...)
	tmp[0] = uintptr(unsafe.Pointer(super)) // if createArgs splits the stack the pointer would be wrong
	ret, _, _ := purego.SyscallN(objc_msgSendSuper2, tmp...)
	return ID(ret)
}

func createArgs(cls ID, sel SEL, args ...interface{}) (out []uintptr) {
	out = make([]uintptr, 2, len(args)+2)
	out[0] = uintptr(cls)
	out[1] = uintptr(sel)
	for _, a := range args {
		switch v := a.(type) {
		case ID:
			out = append(out, uintptr(v))
		case Class:
			out = append(out, uintptr(v))
		case SEL:
			out = append(out, uintptr(v))
		case _IMP:
			out = append(out, uintptr(v))
		case bool:
			if v {
				out = append(out, uintptr(1))
			} else {
				out = append(out, uintptr(0))
			}
		case unsafe.Pointer:
			out = append(out, uintptr(v))
		case uintptr:
			out = append(out, v)
		case int:
			out = append(out, uintptr(v))
		case uint:
			out = append(out, uintptr(v))
		default:
			panic(fmt.Sprintf("objc: unknown type %T", v))
		}
	}
	return out
}

// SEL is an opaque type that represents a method selector
type SEL uintptr

// RegisterName registers a method with the Objective-C runtime system, maps the method name to a selector,
// and returns the selector value.
func RegisterName(name string) SEL {
	n := strings.CString(name)
	ret, _, _ := purego.SyscallN(sel_registerName, uintptr(unsafe.Pointer(n)))
	runtime.KeepAlive(n)
	return SEL(ret)
}

// Class is an opaque type that represents an Objective-C class.
type Class uintptr

// GetClass returns the Class object for the named class, or nil if the class is not registered with the Objective-C runtime.
func GetClass(name string) Class {
	n := strings.CString(name)
	ret, _, _ := purego.SyscallN(objc_getClass, uintptr(unsafe.Pointer(n)))
	runtime.KeepAlive(n)
	return Class(ret)
}

// AllocateClassPair creates a new class and metaclass. Then returns the new class, or Nil if the class could not be created
func AllocateClassPair(super Class, name string, extraBytes uintptr) Class {
	n := strings.CString(name)
	ret, _, _ := purego.SyscallN(objc_allocateClassPair, uintptr(super), uintptr(unsafe.Pointer(n)), extraBytes)
	runtime.KeepAlive(n)
	return Class(ret)
}

// SuperClass returns the superclass of a class.
// You should usually use NSObject‘s superclass method instead of this function.
func (c Class) SuperClass() Class {
	ret, _, _ := purego.SyscallN(class_getSuperclass, uintptr(c))
	return Class(ret)
}

// AddMethod adds a new method to a class with a given name and implementation.
// The types argument is a string containing the mapping of parameters and return type.
// Since the function must take at least two arguments—self and _cmd, the second and third
// characters must be “@:” (the first character is the return type).
func (c Class) AddMethod(name SEL, imp _IMP, types string) bool {
	t := strings.CString(types)
	ret, _, _ := purego.SyscallN(class_addMethod, uintptr(c), uintptr(name), uintptr(imp), uintptr(unsafe.Pointer(t)))
	runtime.KeepAlive(t)
	return byte(ret) != 0
}

// AddIvar adds a new instance variable to a class.
// It may only be called after AllocateClassPair and before Register.
// Adding an instance variable to an existing class is not supported.
// The class must not be a metaclass. Adding an instance variable to a metaclass is not supported.
// The instance variable's minimum alignment in bytes is 1<<align. The minimum alignment of an
// instance variable depends on the ivar's type and the machine architecture.
// For variables of any pointer type, pass math.Log2(unsafe.Alignof(type)).
func (c Class) AddIvar(name string, size uintptr, alignment uint8, types string) bool {
	n := strings.CString(name)
	t := strings.CString(types)
	ret, _, _ := purego.SyscallN(class_addIvar, uintptr(c), uintptr(unsafe.Pointer(n)), size, uintptr(alignment), uintptr(unsafe.Pointer(t)))
	runtime.KeepAlive(n)
	runtime.KeepAlive(t)
	return byte(ret) != 0
}

// InstanceVariable returns an Ivar data structure containing information about the instance variable specified by name.
func (c Class) InstanceVariable(name string) Ivar {
	n := strings.CString(name)
	ret, _, _ := purego.SyscallN(class_getInstanceVariable, uintptr(c), uintptr(unsafe.Pointer(n)))
	runtime.KeepAlive(n)
	return Ivar(ret)
}

// Register registers a class that was allocated using AllocateClassPair.
// It can now be used to make objects by sending it either alloc and init or new.
func (c Class) Register() {
	purego.SyscallN(objc_registerClassPair, uintptr(c))
}

// Ivar an opaque type that represents an instance variable.
type Ivar uintptr

// Offset returns the offset of an instance variable that can be used to assign and read the Ivar's value.
//
// For instance variables of type id or other object types, call Ivar and SetIvar instead
// of using this offset to access the instance variable data directly.
func (i Ivar) Offset() uintptr {
	ret, _, _ := purego.SyscallN(ivar_getOffset, uintptr(i))
	return ret
}

// _IMP is unexported so that the only way to make this type is by providing a Go function and casting
// it with the IMP function
type _IMP uintptr

// IMP takes a Go function that takes (ID, SEL) as its first two arguments. It returns an _IMP function
// pointer that can be called by Objective-C code. The function pointer is never deallocated.
func IMP(fn interface{}) _IMP {
	// this is only here so that it is easier to port C code to Go.
	// this is not guaranteed to be here forever so make sure to port your callbacks to Go
	// If you have a C function pointer cast it to a uintptr before passing it
	// to this function.
	if x, ok := fn.(uintptr); ok {
		return _IMP(x)
	}
	val := reflect.ValueOf(fn)
	if val.Kind() != reflect.Func {
		panic("objc: not a function")
	}
	// IMP is stricter than a normal callback
	// id (*IMP)(id, SEL, ...)
	switch {
	case val.Type().NumIn() < 2:
		fallthrough
	case val.Type().In(0).Kind() != reflect.Uintptr:
		fallthrough
	case val.Type().In(1).Kind() != reflect.Uintptr:
		panic("objc: IMP must take a (id, SEL) as its first two arguments")
	}
	return _IMP(purego.NewCallback(fn))
}

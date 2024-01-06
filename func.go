// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebitengine Authors

//go:build darwin || freebsd || linux || windows

package purego

import (
	"math"
	"reflect"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego/internal/strings"
)

// RegisterLibFunc is a wrapper around RegisterFunc that uses the C function returned from Dlsym(handle, name).
// It panics if it can't find the name symbol.
func RegisterLibFunc(fptr interface{}, handle uintptr, name string) {
	sym, err := loadSymbol(handle, name)
	if err != nil {
		panic(err)
	}
	RegisterFunc(fptr, sym)
}

// RegisterFunc takes a pointer to a Go function representing the calling convention of the C function.
// fptr will be set to a function that when called will call the C function given by cfn with the
// parameters passed in the correct registers and stack.
//
// A panic is produced if the type is not a function pointer or if the function returns more than 1 value.
//
// These conversions describe how a Go type in the fptr will be used to call
// the C function. It is important to note that there is no way to verify that fptr
// matches the C function. This also holds true for struct types where the padding
// needs to be ensured to match that of C; RegisterFunc does not verify this.
//
// # Type Conversions (Go <=> C)
//
//	string <=> char*
//	bool <=> _Bool
//	uintptr <=> uintptr_t
//	uint <=> uint32_t or uint64_t
//	uint8 <=> uint8_t
//	uint16 <=> uint16_t
//	uint32 <=> uint32_t
//	uint64 <=> uint64_t
//	int <=> int32_t or int64_t
//	int8 <=> int8_t
//	int16 <=> int16_t
//	int32 <=> int32_t
//	int64 <=> int64_t
//	float32 <=> float
//	float64 <=> double
//	struct <=> struct (WIP - macOS only)
//	func <=> C function
//	unsafe.Pointer, *T <=> void*
//	[]T => void*
//
// There is a special case when the last argument of fptr is a variadic interface (or []interface}
// it will be expanded into a call to the C function as if it had the arguments in that slice.
// This means that using arg ...interface{} is like a cast to the function with the arguments inside arg.
// This is not the same as C variadic.
//
// # Memory
//
// In general it is not possible for purego to guarantee the lifetimes of objects returned or received from
// calling functions using RegisterFunc. For arguments to a C function it is important that the C function doesn't
// hold onto a reference to Go memory. This is the same as the [Cgo rules].
//
// However, there are some special cases. When passing a string as an argument if the string does not end in a null
// terminated byte (\x00) then the string will be copied into memory maintained by purego. The memory is only valid for
// that specific call. Therefore, if the C code keeps a reference to that string it may become invalid at some
// undefined time. However, if the string does already contain a null-terminated byte then no copy is done.
// It is then the responsibility of the caller to ensure the string stays alive as long as it's needed in C memory.
// This can be done using runtime.KeepAlive or allocating the string in C memory using malloc. When a C function
// returns a null-terminated pointer to char a Go string can be used. Purego will allocate a new string in Go memory
// and copy the data over. This string will be garbage collected whenever Go decides it's no longer referenced.
// This C created string will not be freed by purego. If the pointer to char is not null-terminated or must continue
// to point to C memory (because it's a buffer for example) then use a pointer to byte and then convert that to a slice
// using unsafe.Slice. Doing this means that it becomes the responsibility of the caller to care about the lifetime
// of the pointer
//
// # Example
//
// All functions below call this C function:
//
//	char *foo(char *str);
//
//	// Let purego convert types
//	var foo func(s string) string
//	goString := foo("copied")
//	// Go will garbage collect this string
//
//	// Manually, handle allocations
//	var foo2 func(b string) *byte
//	mustFree := foo2("not copied\x00")
//	defer free(mustFree)
//
// [Cgo rules]: https://pkg.go.dev/cmd/cgo#hdr-Go_references_to_C
func RegisterFunc(fptr interface{}, cfn uintptr) {
	fn := reflect.ValueOf(fptr).Elem()
	ty := fn.Type()
	if ty.Kind() != reflect.Func {
		panic("purego: fptr must be a function pointer")
	}
	if ty.NumOut() > 1 {
		panic("purego: function can only return zero or one values")
	}
	if cfn == 0 {
		panic("purego: cfn is nil")
	}
	{
		// this code checks how many registers and stack this function will use
		// to avoid crashing with too many arguments
		var ints int
		var floats int
		var stack int
		for i := 0; i < ty.NumIn(); i++ {
			arg := ty.In(i)
			switch arg.Kind() {
			case reflect.String, reflect.Uintptr, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
				reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Ptr, reflect.UnsafePointer, reflect.Slice,
				reflect.Func, reflect.Bool:
				if ints < numOfIntegerRegisters() {
					ints++
				} else {
					stack++
				}
			case reflect.Float32, reflect.Float64:
				if floats < numOfFloats {
					floats++
				} else {
					stack++
				}
			case reflect.Struct:
				if runtime.GOOS != "darwin" || (runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64") {
					panic("purego: struct arguments are only supported on macOS amd64 & arm64")
				}
				addInt := func(u uintptr) {
					ints++
				}
				addFloat := func(u uintptr) {
					floats++
				}
				addStack := func(u uintptr) {
					stack++
				}
				if arg.Size() == 0 {
					continue
				}
				_ = addStruct(reflect.New(arg).Elem(), &ints, &floats, &stack, addInt, addFloat, addStack, nil)
			default:
				panic("purego: unsupported kind " + arg.Kind().String())
			}
		}
		sizeOfStack := maxArgs - numOfIntegerRegisters()
		if stack > sizeOfStack {
			panic("purego: too many arguments")
		}
	}
	v := reflect.MakeFunc(ty, func(args []reflect.Value) (results []reflect.Value) {
		if len(args) > 0 {
			if variadic, ok := args[len(args)-1].Interface().([]interface{}); ok {
				// subtract one from args bc the last argument in args is []interface{}
				// which we are currently expanding
				tmp := make([]reflect.Value, len(args)-1+len(variadic))
				n := copy(tmp, args[:len(args)-1])
				for i, v := range variadic {
					tmp[n+i] = reflect.ValueOf(v)
				}
				args = tmp
			}
		}
		var sysargs [maxArgs]uintptr
		stack := sysargs[numOfIntegerRegisters():]
		var floats [numOfFloats]uintptr
		var numInts int
		var numFloats int
		var numStack int
		var addStack, addInt, addFloat func(x uintptr)
		if runtime.GOARCH == "arm64" || runtime.GOOS != "windows" {
			// Windows arm64 uses the same calling convention as macOS and Linux
			addStack = func(x uintptr) {
				stack[numStack] = x
				numStack++
			}
			addInt = func(x uintptr) {
				if numInts >= numOfIntegerRegisters() {
					addStack(x)
				} else {
					sysargs[numInts] = x
					numInts++
				}
			}
			addFloat = func(x uintptr) {
				if numFloats < len(floats) {
					floats[numFloats] = x
					numFloats++
				} else {
					addStack(x)
				}
			}
		} else {
			// On Windows amd64 the arguments are passed in the numbered registered.
			// So the first int is in the first integer register and the first float
			// is in the second floating register if there is already a first int.
			// This is in contrast to how macOS and Linux pass arguments which
			// tries to use as many registers as possible in the calling convention.
			addStack = func(x uintptr) {
				sysargs[numStack] = x
				numStack++
			}
			addInt = addStack
			addFloat = addStack
		}

		var keepAlive []interface{}
		defer func() {
			runtime.KeepAlive(keepAlive)
			runtime.KeepAlive(args)
		}()
		for _, v := range args {
			switch v.Kind() {
			case reflect.String:
				ptr := strings.CString(v.String())
				keepAlive = append(keepAlive, ptr)
				addInt(uintptr(unsafe.Pointer(ptr)))
			case reflect.Uintptr, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				addInt(uintptr(v.Uint()))
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				addInt(uintptr(v.Int()))
			case reflect.Ptr, reflect.UnsafePointer, reflect.Slice:
				// There is no need to keepAlive this pointer separately because it is kept alive in the args variable
				addInt(v.Pointer())
			case reflect.Func:
				addInt(NewCallback(v.Interface()))
			case reflect.Bool:
				if v.Bool() {
					addInt(1)
				} else {
					addInt(0)
				}
			case reflect.Float32:
				addFloat(uintptr(math.Float32bits(float32(v.Float()))))
			case reflect.Float64:
				addFloat(uintptr(math.Float64bits(v.Float())))
			case reflect.Struct:
				keepAlive = addStruct(v, &numInts, &numFloats, &numStack, addInt, addFloat, addStack, keepAlive)
			default:
				panic("purego: unsupported kind: " + v.Kind().String())
			}
		}
		// TODO: support structs
		var r1, r2 uintptr
		if runtime.GOARCH == "arm64" || runtime.GOOS != "windows" {
			// Use the normal arm64 calling convention even on Windows
			syscall := syscall15Args{
				cfn,
				sysargs[0], sysargs[1], sysargs[2], sysargs[3], sysargs[4], sysargs[5],
				sysargs[6], sysargs[7], sysargs[8], sysargs[9], sysargs[10], sysargs[11],
				sysargs[12], sysargs[13], sysargs[14],
				floats[0], floats[1], floats[2], floats[3], floats[4], floats[5], floats[6], floats[7],
				0, 0, 0,
			}
			runtime_cgocall(syscall15XABI0, unsafe.Pointer(&syscall))
			r1, r2 = syscall.r1, syscall.r2
		} else {
			// This is a fallback for Windows amd64, 386, and arm. Note this may not support floats
			r1, r2, _ = syscall_syscall15X(cfn, sysargs[0], sysargs[1], sysargs[2], sysargs[3], sysargs[4],
				sysargs[5], sysargs[6], sysargs[7], sysargs[8], sysargs[9], sysargs[10], sysargs[11],
				sysargs[12], sysargs[13], sysargs[14])
		}
		if ty.NumOut() == 0 {
			return nil
		}
		outType := ty.Out(0)
		v := reflect.New(outType).Elem()
		switch outType.Kind() {
		case reflect.Uintptr, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			v.SetUint(uint64(r1))
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			v.SetInt(int64(r1))
		case reflect.Bool:
			v.SetBool(byte(r1) != 0)
		case reflect.UnsafePointer:
			// We take the address and then dereference it to trick go vet from creating a possible miss-use of unsafe.Pointer
			v.SetPointer(*(*unsafe.Pointer)(unsafe.Pointer(&r1)))
		case reflect.Ptr:
			// It is safe to have the address of r1 not escape because it is immediately dereferenced with .Elem()
			v = reflect.NewAt(outType, runtime_noescape(unsafe.Pointer(&r1))).Elem()
		case reflect.Func:
			// wrap this C function in a nicely typed Go function
			v = reflect.New(outType)
			RegisterFunc(v.Interface(), r1)
		case reflect.String:
			v.SetString(strings.GoString(r1))
		case reflect.Float32:
			// NOTE: r2 is only the floating return value on 64bit platforms.
			// On 32bit platforms r2 is the upper part of a 64bit return.
			v.SetFloat(float64(math.Float32frombits(uint32(r2))))
		case reflect.Float64:
			// NOTE: r2 is only the floating return value on 64bit platforms.
			// On 32bit platforms r2 is the upper part of a 64bit return.
			v.SetFloat(math.Float64frombits(uint64(r2)))
		default:
			panic("purego: unsupported return kind: " + outType.Kind().String())
		}
		return []reflect.Value{v}
	})
	fn.Set(v)
}

func addStruct(v reflect.Value, numInts, numFloats, numStack *int, addInt, addFloat, addStack func(uintptr), keepAlive []interface{}) []interface{} {
	if runtime.GOARCH == "arm64" {
		// https://student.cs.uwaterloo.ca/~cs452/docs/rpi4b/aapcs64.pdf
		if hva, hfa, size := isHVA(v.Type()), isHFA(v.Type()), v.Type().Size(); hva || hfa || size <= 16 {
			// if this doesn't fit entirely in registers then
			// each element goes onto the stack
			if hfa && *numFloats+v.NumField() > numOfFloats {
				*numFloats = numOfFloats
			} else if hva && *numInts+v.NumField() > numOfIntegerRegisters() {
				*numInts = numOfIntegerRegisters()
			}

			numFields := v.NumField()
			var val uint64
			var shift byte
			flushed := false
			for k := 0; k < numFields; k++ {
				f := v.Field(k)
				if shift >= 64 {
					shift = 0
					flushed = true
					addInt(uintptr(val))
				}
				switch f.Type().Kind() {
				case reflect.Uint8:
					val |= f.Uint() << shift
					shift += 8
				case reflect.Uint16:
					val |= f.Uint() << shift
					shift += 16
				case reflect.Uint32:
					val |= f.Uint() << shift
					shift += 32
				case reflect.Uint64:
					addInt(uintptr(f.Uint()))
					shift = 0
				case reflect.Int8:
					val |= uint64(f.Int()&0xFF) << shift
					shift += 8
				case reflect.Int16:
					val |= uint64(f.Int()&0xFFFF) << shift
					shift += 16
				case reflect.Int32:
					val |= uint64(f.Int()&0xFFFF_FFFF) << shift
					shift += 32
				case reflect.Int64:
					addInt(uintptr(f.Int()))
					shift = 0
				case reflect.Float32:
					addFloat(uintptr(math.Float32bits(float32(f.Float()))))
				case reflect.Float64:
					addFloat(uintptr(math.Float64bits(float64(f.Float()))))
				case reflect.Array:
					arraySize := f.Len()
					var arrayVal uint64
					var arrayShift byte
					arrayFlushed := false
					for i := 0; i < arraySize; i++ {
						elm := f.Index(i)
						if arrayShift >= 64 {
							arrayShift = 0
							arrayFlushed = true
							addInt(uintptr(arrayVal))
						}
						switch elm.Type().Kind() {
						case reflect.Uint8:
							arrayVal |= elm.Uint() << arrayShift
							arrayShift += 8
						case reflect.Uint16:
							arrayVal |= elm.Uint() << arrayShift
							arrayShift += 16
						case reflect.Uint32:
							arrayVal |= elm.Uint() << arrayShift
							arrayShift += 32
						case reflect.Uint64:
							addInt(uintptr(elm.Uint()))
							arrayShift = 0
						case reflect.Int8:
							arrayVal |= uint64(elm.Int()&0xFF) << arrayShift
							arrayShift += 8
						case reflect.Int16:
							arrayVal |= uint64(elm.Int()&0xFFFF) << arrayShift
							arrayShift += 16
						case reflect.Int32:
							arrayVal |= uint64(elm.Int()&0xFFFF_FFFF) << arrayShift
							arrayShift += 32
						case reflect.Int64:
							addInt(uintptr(elm.Int()))
							arrayShift = 0
						case reflect.Float32:
							addFloat(uintptr(math.Float32bits(float32(elm.Float()))))
						case reflect.Float64:
							addFloat(uintptr(math.Float64bits(float64(elm.Float()))))
						default:
							panic("purego: unsupported kind " + elm.Kind().String())
						}
					}
					if !arrayFlushed {
						addInt(uintptr(arrayVal))
					}
				default:
					panic("purego: unsupported kind " + f.Kind().String())
				}
			}
			if !flushed {
				addInt(uintptr(val))
			}
		} else {
			// Struct is too big to be placed in registers.
			// Copy to heap and place the pointer in register
			ptrStruct := reflect.New(v.Type())
			ptrStruct.Elem().Set(v)
			ptr := ptrStruct.Elem().Addr().UnsafePointer()
			keepAlive = append(keepAlive, ptr)
			addInt(uintptr(ptr))
		}
		return keepAlive // the struct was allocated so don't panic
	} else if runtime.GOARCH == "amd64" {
		// https://www.uclibc.org/docs/psABI-x86_64.pdf
		if v.Type().Size() == 0 {
			return keepAlive
		} else if v.Type().Size() > 0 {
			// Class determines where the 8 byte value goes.
			// Higher value classes win over lower value classes
			const (
				NO_CLASS = 0b0000
				SSE      = 0b0001
				X87      = 0b0011 // long double not used in Go
				INTEGER  = 0b0111
				MEMORY   = 0b1111
			)
			var (
				placedOnStack  = v.Type().Size() > 8*8 // if greater than 64 bytes place on stack
				savedNumFloats = *numFloats
				savedNumInts   = *numInts
				savedNumStack  = *numStack
			)
			numFields := v.Type().NumField()
			var val uint64
			var shift byte // # of bits to shift
			flushed := false
			class := NO_CLASS
		loop:
			for i := 0; i < numFields; i++ {
				f := v.Field(i)
				if shift+byte(f.Type().Size())*8 > 64 {
					shift = 0
					flushed = true
					if class == SSE {
						addFloat(uintptr(val))
					} else {
						addInt(uintptr(val))
					}
					class = NO_CLASS
				}
				switch f.Kind() {
				case reflect.Pointer:
					placedOnStack = true
					break loop
				case reflect.Int8:
					val |= uint64(f.Int()&0xFF) << shift
					shift += 8
					class |= INTEGER
				case reflect.Int16:
					val |= uint64(f.Int()&0xFFFF) << shift
					shift += 16
					class |= INTEGER
				case reflect.Int32:
					val |= uint64(f.Int()&0xFFFF_FFFF) << shift
					shift += 32
					class |= INTEGER
				case reflect.Int, reflect.Int64:
					addInt(uintptr(f.Int()))
					shift = 0
					class = NO_CLASS
				case reflect.Uint8:
					val |= f.Uint() << shift
					shift += 8
					class |= INTEGER
				case reflect.Uint16:
					val |= f.Uint() << shift
					shift += 16
					class |= INTEGER
				case reflect.Uint32:
					val |= f.Uint() << shift
					shift += 32
					class |= INTEGER
				case reflect.Uint, reflect.Uint64:
					addInt(uintptr(f.Uint()))
					shift = 0
					class = NO_CLASS
				case reflect.Float32:
					val |= uint64(math.Float32bits(float32(f.Float()))) << shift
					shift += 32
					class |= SSE
				case reflect.Float64:
					if v.Type().Size() > 16 {
						placedOnStack = true
						break loop
					}
					addFloat(uintptr(math.Float64bits(f.Float())))
					class = NO_CLASS
				case reflect.Array:
					arraySize := f.Len()
					arrayFirstType := f.Index(0).Type()
					switch arrayFirstType.Kind() {
					case reflect.Float64:
						for k := 0; k < arraySize; k++ {
							elm := f.Index(k)
							addFloat(uintptr(math.Float64bits(float64(elm.Float()))))
						}
					case reflect.Float32:
						for k := 0; k < arraySize; k++ {
							elm := f.Index(k)
							addFloat(uintptr(math.Float32bits(float32(elm.Float()))))
						}
					case reflect.Uint8, reflect.Uint16, reflect.Uint32:
						sizeInBytes := int(arrayFirstType.Size())
						var val uint64
						var k int
						for k = i; k < arraySize; k++ {
							elm := f.Index(k)
							// Reverse the bytes
							// 0xde_ad_be_ef becomes 0xef_be_ad_de
							val |= elm.Uint() << ((k - i) * (8 * sizeInBytes))
						}
						i = k - 1
						addInt(uintptr(val))
					case reflect.Int8, reflect.Int16, reflect.Int32:
						sizeInBytes := int(arrayFirstType.Size())
						mask := uint64(0xFF)
						for k := 1; k < sizeInBytes; k++ {
							mask = (mask << 8) + 0xFF
						}
						var val uint64
						var k int
						for k = i; k < arraySize; k++ {
							elm := f.Index(k)
							val |= (uint64(elm.Int()) & mask) << ((k - i) * (8 * sizeInBytes))
						}
						i = k - 1
						addInt(uintptr(val))
					default:
						panic("purego: unsupported array kind " + arrayFirstType.Kind().String())
					}
				default:
					panic("purego: unsupported kind " + f.Kind().String())
				}
			}
			if !flushed {
				if class == SSE {
					addFloat(uintptr(val))
				} else {
					addInt(uintptr(val))
				}
			}
			if placedOnStack {
				*numFloats = savedNumFloats
				*numInts = savedNumInts
				*numStack = savedNumStack
				for i := 0; i < v.Type().NumField(); i++ {
					f := v.Field(i)
					switch f.Kind() {
					case reflect.Pointer:
						addStack(f.Pointer())
					case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
						addStack(uintptr(f.Int()))
					case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
						addStack(uintptr(f.Uint()))
					case reflect.Float32:
						addStack(uintptr(math.Float32bits(float32(f.Float()))))
					case reflect.Float64:
						addStack(uintptr(math.Float64bits(f.Float())))
					default:
						panic("purego: unsupported kind " + f.Kind().String())
					}
				}
			}
			return keepAlive
		}
	}
	panic("purego: struct has field that can't be allocated")
}

func roundUpTo8(val uintptr) uintptr {
	return (val + 7) &^ 7
}

func isHFA(t reflect.Type) bool {
	// round up struct size to nearest 8 see section B.4
	structSize := roundUpTo8(t.Size())
	if structSize == 0 || t.NumField() > 4 {
		return false
	}
	first := t.Field(0)
	switch first.Type.Kind() {
	case reflect.Float32, reflect.Float64:
		firstKind := first.Type.Kind()
		for i := 0; i < t.NumField(); i++ {
			if t.Field(i).Type.Kind() != firstKind {
				return false
			}
		}
		return true
	case reflect.Array:
		switch first.Type.Elem().Kind() {
		case reflect.Float32, reflect.Float64:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func isHVA(t reflect.Type) bool {
	// round up struct size to nearest 8 see section B.4
	structSize := roundUpTo8(t.Size())
	if structSize == 0 || (structSize != 8 && structSize != 16) {
		return false
	}
	first := t.Field(0)
	switch first.Type.Kind() {
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Int8, reflect.Int16, reflect.Int32:
		firstKind := first.Type.Kind()
		for i := 0; i < t.NumField(); i++ {
			if t.Field(i).Type.Kind() != firstKind {
				return false
			}
		}
		return true
	case reflect.Array:
		switch first.Type.Elem().Kind() {
		case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Int8, reflect.Int16, reflect.Int32:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func numOfIntegerRegisters() int {
	switch runtime.GOARCH {
	case "arm64":
		return 8
	case "amd64":
		return 6
	// TODO: figure out why 386 tests are not working
	/*case "386":
		return 0
	case "arm":
		return 4*/
	default:
		panic("purego: unknown GOARCH (" + runtime.GOARCH + ")")
	}
}

package purego

import (
	"math"
	"reflect"
)

func addStruct(v reflect.Value, numInts, numFloats, numStack *int, addInt, addFloat, addStack func(uintptr), keepAlive []interface{}) []interface{} {
	if v.Type().Size() == 0 {
		return keepAlive
	}
	// https://www.uclibc.org/docs/psABI-x86_64.pdf
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

	var val uint64
	var shift byte // # of bits to shift
	var flushed bool
	class := NO_CLASS
	var place func(v reflect.Value)
	place = func(v reflect.Value) {
		var numFields int
		if v.Kind() == reflect.Struct {
			numFields = v.Type().NumField()
		} else {
			numFields = v.Type().Len()
		}

	loop:
		for i := 0; i < numFields; i++ {
			var f reflect.Value
			if v.Kind() == reflect.Struct {
				f = v.Field(i)
			} else {
				f = v.Index(i)
			}
			if shift >= 64 {
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
			case reflect.Struct:
				place(f)
			case reflect.Bool:
				if f.Bool() {
					val |= 1
				}
				shift += 8
				class |= INTEGER
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
				place(f)
			default:
				panic("purego: unsupported kind " + f.Kind().String())
			}
		}
	}
	place(v)
	if !flushed {
		if class == SSE {
			addFloat(uintptr(val))
		} else {
			addInt(uintptr(val))
		}
	}
	if placedOnStack {
		// reset any values placed in registers
		*numFloats = savedNumFloats
		*numInts = savedNumInts
		*numStack = savedNumStack
		var placeStack func(v reflect.Value)
		placeStack = func(v reflect.Value) {
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
				case reflect.Struct:
					placeStack(f)
				default:
					panic("purego: unsupported kind " + f.Kind().String())
				}
			}
		}
		placeStack(v)
	}
	return keepAlive
}

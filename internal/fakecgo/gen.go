// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebitengine Authors

//go:build ignore

package main

import (
	"log"
	"os"
	"strings"
	"text/template"
)

const templateSymbols = `// Code generated by 'go generate' with gen.go. DO NOT EDIT.

// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebitengine Authors

//go:build darwin || linux

package fakecgo

import (
	"syscall"
	"unsafe"
)

// setg_trampoline calls setg with the G provided
func setg_trampoline(setg uintptr, G uintptr)

//go:linkname memmove runtime.memmove
func memmove(to, from unsafe.Pointer, n uintptr)

// call5 takes fn the C function and 5 arguments and calls the function with those arguments
func call5(fn, a1, a2, a3, a4, a5 uintptr) uintptr

{{ range . -}}
func {{.Name}}(
{{- range .Args -}}
	{{- if .Name -}}
		{{.Name}} {{.Type}},
	{{- end -}}
{{- end -}}) {{.Return}} {
	{{- if .Return -}}
		{{- if eq .Return "unsafe.Pointer" -}}
			ret :=
		{{- else -}}
			return {{.Return}}(
		{{- end -}}
	{{- end -}}
call5({{.Name}}ABI0,
{{- range .Args}}
	{{- if .Name -}}
		{{- if hasPrefix .Type "*" -}}
			uintptr(unsafe.Pointer({{.Name}})),
		{{- else -}}
			uintptr({{.Name}}),
		{{- end -}}
	{{- else -}}
		0,
	{{- end -}}
{{- end -}}
	) {{/* end of call5 */}}
{{- if .Return -}}
	{{- if eq .Return "unsafe.Pointer"}}
		// this indirection is to avoid go vet complaining about possible misuse of unsafe.Pointer
		return *(*unsafe.Pointer)(unsafe.Pointer(&ret))
	{{- else -}}
		) {{/* end of cast */}}
	{{- end -}}
{{- end}}
}

{{end}} 
{{- range . }}
//go:linkname _{{.Name}} _{{.Name}}
var _{{.Name}} uintptr
var {{.Name}}ABI0 = uintptr(unsafe.Pointer(&_{{.Name}}))
{{ end }}
`

const templateTrampolinesStubs = `// Code generated by 'go generate' with gen.go. DO NOT EDIT.

// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebitengine Authors

//go:build darwin || linux

#include "textflag.h"

// these stubs are here because it is not possible to go:linkname directly the C functions on darwin arm64
{{ range . }}
TEXT _{{.Name}}(SB), NOSPLIT, $0-0
	JMP purego_{{.Name}}(SB)
	RET
{{ end -}}
`

type Arg struct {
	Name string
	Type string
}

type Symbol struct {
	Name   string
	Args   [5]Arg
	Return string
}

var symbols = []Symbol{
	{"malloc", [5]Arg{{"size", "uintptr"}}, "unsafe.Pointer"},
	{"free", [5]Arg{{"ptr", "unsafe.Pointer"}}, ""},
	{"setenv", [5]Arg{{"name", "*byte"}, {"value", "*byte"}, {"overwrite", "int32"}}, "int32"},
	{"unsetenv", [5]Arg{{"name", "*byte"}}, "int32"},
	{"pthread_attr_init", [5]Arg{{"attr", "*pthread_attr_t"}}, "int32"},
	{"pthread_create", [5]Arg{{"thread", "*pthread_t"}, {"attr", "*pthread_attr_t"}, {"start", "unsafe.Pointer"}, {"arg", "unsafe.Pointer"}}, "int32"},
	{"pthread_detach", [5]Arg{{"thread", "pthread_t"}}, "int32"},
	{"pthread_sigmask", [5]Arg{{"how", "sighow"}, {"ign", "*sigset_t"}, {"oset", "*sigset_t"}}, "int32"},
	{"pthread_attr_getstacksize", [5]Arg{{"attr", "*pthread_attr_t"}, {"stacksize", "*size_t"}}, "int32"},
	{"pthread_attr_destroy", [5]Arg{{"attr", "*pthread_attr_t"}}, "int32"},
	{"sigfillset", [5]Arg{{"set", "*sigset_t"}}, "int32"},
	{"nanosleep", [5]Arg{{"ts", "*syscall.Timespec"}, {"rem", "*syscall.Timespec"}}, "int32"},
	{"abort", [5]Arg{}, ""},
}

var funcs = map[string]any{
	"hasPrefix": strings.HasPrefix,
}

func run() error {
	t, err := template.New("symbol.go").Funcs(funcs).Parse(templateSymbols)
	if err != nil {
		return err
	}
	f, err := os.Create("symbols.go")
	if err != nil {
		return err
	}
	if err := t.Execute(f, symbols); err != nil {
		return err
	}
	t, err = template.New("trampolines_stubs.s").Funcs(funcs).Parse(templateTrampolinesStubs)
	if err != nil {
		return err
	}
	f, err = os.Create("trampolines_stubs.s")
	if err != nil {
		return err
	}
	if err := t.Execute(f, symbols); err != nil {
		return err
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

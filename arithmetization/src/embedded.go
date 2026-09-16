// Package src embeds the R5 interpreter source code into the Go binary.
//
// The source code is embedded using the Go 1.16 embed package, which allows us
// to include files and directories in the Go binary at compile time.
//
// This package provides only the embedded source code (due to prohibition of
// embedding to include files outside of the package root). It does not provide
// any functionality to compile or execute the R5 interpreter. For obtaining the
// compiled binary file of the R5 interpreter, use the
// [github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded]
// package, which provides the [CompiledBinaryFile] function to compile the
// embedded source code into a binary file.
package src

import "embed"

//go:embed main
var MainDir embed.FS

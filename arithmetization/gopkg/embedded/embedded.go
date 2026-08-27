// Package embedded stored the embedded R5 interpreter implementation.
package embedded

import (
	"errors"
	"io/fs"
	"path/filepath"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/src"
	"github.com/LFDT-Lineth/zkc/pkg/util/field"
	"github.com/LFDT-Lineth/zkc/pkg/util/field/koalabear"
	"github.com/LFDT-Lineth/zkc/pkg/util/source"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler/ast"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler/codegen"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/constraints"
)

// ArithmetizationFS is the embedded R5 interpreter implementation.
//
// It implements the fs.FS interface, allowing access to the embedded files in the "main" directory.
// See [fs] package for more information how to walk the embedded files.
var ArithmetizationFS = src.MainDir

const mainDir = "main"
const zkcExt = ".zkc"
const predecodingDir = "predecoding"

// ArithmetizationSourceFiles returns the embedded R5 interpreter source files
// as a slice of fs.File.
func ArithmetizationFiles() []fs.File {
	subFs, err := fs.Sub(ArithmetizationFS, mainDir)
	if err != nil {
		panic("failed to get sub FS for embedded R5 interpreter: " + err.Error())
	}
	files := []fs.File{}
	if err := fs.WalkDir(subFs, ".", func(path string, d fs.DirEntry, err error) error {
		// skip predecoding directory, as it contains files that are not part of
		// the main R5 interpreter source code.
		if d.IsDir() && path == predecodingDir {
			return fs.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != zkcExt {
			return nil
		}
		file, err := subFs.Open(path)
		if err != nil {
			return err
		}
		files = append(files, file)
		return nil
	}); err != nil {
		panic("failed to walk embedded R5 interpreter files: " + err.Error())
	}
	return files
}

// ArithmetizationSourceFiles returns the embedded R5 interpreter source files
// as a slice of source.File.
func ArithmetizationSourceFiles() []source.File {
	subFs, err := fs.Sub(ArithmetizationFS, mainDir)
	if err != nil {
		panic("failed to get sub FS for embedded R5 interpreter: " + err.Error())
	}
	srcFiles := []source.File{}
	if err := fs.WalkDir(subFs, ".", func(path string, d fs.DirEntry, err error) error {
		// skip predecoding directory, as it contains files that are not part of
		// the main R5 interpreter source code.
		if d.IsDir() && path == predecodingDir {
			return fs.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != zkcExt {
			return nil
		}
		bytes, err := fs.ReadFile(subFs, path)
		if err != nil {
			return err
		}
		absPath := filepath.Join("/", path)
		srcFile := source.NewSourceFile(absPath, bytes)
		srcFiles = append(srcFiles, *srcFile)
		return nil
	}); err != nil {
		panic("failed to walk embedded R5 interpreter source files: " + err.Error())
	}
	return srcFiles
}

// CompiledBinaryFile compiles the embedded R5 interpreter source files into a
// binary file.
//
// It returns the compiled binary file, or an error if the compilation fails.
// metadata and attributes are optional parameters that can be used to provide
// additional information about the binary file. Provide nil for metadata and
// attributes if not needed.
func CompiledBinaryFile(cfg codegen.Config, metadata []byte, attributes []constraints.Attribute) (binfile *constraints.BinaryFile[koalabear.Element], err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New("failed to compile embedded R5 interpreter: " + r.(error).Error())
		}
	}()
	srcFiles := ArithmetizationSourceFiles()
	macroProgram, _, sErrs := compiler.Compile(field.KOALABEAR_16, cfg.GetMaxStaticHeight(), srcFiles...)
	if len(sErrs) > 0 {
		errs := make([]error, len(sErrs))
		for i := range sErrs {
			errs[i] = &sErrs[i]
		}
		return nil, errors.Join(errs...)
	}
	ir, sErrs := ast.Compile(macroProgram, cfg)
	if len(sErrs) > 0 {
		errs := make([]error, len(sErrs))
		for i := range sErrs {
			errs[i] = &sErrs[i]
		}
		return nil, errors.Join(errs...)
	}
	binfile = constraints.NewBinaryFile[koalabear.Element](metadata, attributes, ir)
	return
}

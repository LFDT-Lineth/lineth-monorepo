// Package embedded stores the embedded R5 interpreter implementation.
package embedded

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"

	arith_src "github.com/LFDT-Lineth/lineth-monorepo/arithmetization/src"
	"github.com/LFDT-Lineth/zkc/pkg/util/field"
	"github.com/LFDT-Lineth/zkc/pkg/util/field/koalabear"
	"github.com/LFDT-Lineth/zkc/pkg/util/source"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler/ast"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler/ast/decl"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler/ast/symbol"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler/codegen"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler/parser"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/constraints"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/vm"
)

const mainDir = "main"

// entryPoint is the R5 interpreter entry point, relative to [mainDir]. Only
// the files it reaches through include declarations are compiled.
const entryPoint = "riscv/main.zkc"

// BinaryFile provides an instantiation of constraints.BinaryFile suitable for
// running Koalabear.
type BinaryFile = constraints.BinaryFile[koalabear.Element, vm.Uint32]

// arithmetizationSourceFiles returns the embedded R5 interpreter source files
// as a slice of source.File: the entry point [entryPoint] and the transitive
// closure of its include declarations, resolved inside rootFs.
//
// Following the includes, rather than walking the whole directory, matches
// what `zkc compile riscv/main.zkc` sees. A library that nothing includes (for
// instance an unplugged accelerator under lib/) is not compiled, so it adds no
// modules, and no all-zero columns, to the system. The files have to be listed
// up front because the compiler resolves any include it does not already know
// against the real filesystem, where the embedded paths do not exist.
func arithmetizationSourceFiles(rootFs fs.FS) []source.File {
	subFs, err := fs.Sub(rootFs, mainDir)
	if err != nil {
		panic("failed to get sub FS for embedded R5 interpreter: " + err.Error())
	}
	var (
		srcFiles []source.File
		seen     = map[string]bool{entryPoint: true}
		queue    = []string{entryPoint}
	)
	for len(queue) > 0 {
		filename := queue[0]
		queue = queue[1:]
		bytes, err := fs.ReadFile(subFs, filename)
		if err != nil {
			panic("failed to read embedded R5 interpreter source file: " + err.Error())
		}
		srcFile := source.NewSourceFile(filepath.Join("/", filename), bytes)
		srcFiles = append(srcFiles, *srcFile)
		// Syntax errors are left for compiler.Compile to report; only the
		// include declarations matter here.
		parsed, _ := parser.Parse(srcFile)
		for _, d := range parsed.Declarations {
			include, ok := d.(*decl.Include[symbol.Unresolved])
			if !ok {
				continue
			}
			// Include patterns are relative to the including file and may be
			// globs, as in the compiler.
			matches, err := fs.Glob(subFs, path.Join(path.Dir(filename), include.Pattern()))
			if err != nil {
				panic("invalid include pattern in embedded R5 interpreter: " + err.Error())
			}
			for _, match := range matches {
				if !seen[match] {
					seen[match] = true
					queue = append(queue, match)
				}
			}
		}
	}
	return srcFiles
}

type compileCfg struct {
	config      *codegen.Config
	metadata    []byte
	attributes  []constraints.Attribute
	rootfs      fs.FS
	validateAir bool
}

func setDefaults(cfg *compileCfg) {
	if cfg.config == nil {
		cfg.config = &codegen.DEFAULT_CONFIG
	}
	if cfg.rootfs == nil {
		cfg.rootfs = arith_src.MainDir
	}
}

// CompileOption defines a functional option for modifying the [CompiledBinaryFile] configuration.
type CompileOption func(*compileCfg) error

// WithConfig sets the [codegen.Config] for the compilation.
// If the config is already set, it returns an error.
//
// If not set explicitly, the default config [codegen.DEFAULT_CONFIG] will be used.
func WithConfig(cfg codegen.Config) CompileOption {
	return func(c *compileCfg) error {
		if c.config != nil {
			return errors.New("config already set")
		}
		c.config = &cfg
		return nil
	}
}

// WithMetadata sets the metadata for the compiled binary file.
// If the metadata is already set, it returns an error.
//
// If not set explicitly, the metadata will be nil.
func WithMetadata(metadata []byte) CompileOption {
	return func(c *compileCfg) error {
		if c.metadata != nil {
			return errors.New("metadata already set")
		}
		c.metadata = metadata
		return nil
	}
}

// WithAttributes sets the attributes for the compiled binary file.
// If the attributes are already set, it returns an error.
//
// If not set explicitly, the attributes will be nil.
func WithAttributes(attributes []constraints.Attribute) CompileOption {
	return func(c *compileCfg) error {
		if c.attributes != nil {
			return errors.New("attributes already set")
		}
		c.attributes = attributes
		return nil
	}
}

// WithRootFS sets the root filesystem for the compilation.
// If the root filesystem is already set, it returns an error.
//
// If not set explicitly, the default root filesystem will be used, which is
// the embedded R5 interpreter source files.
func WithRootFS(rootfs fs.FS) CompileOption {
	return func(c *compileCfg) error {
		if c.rootfs != nil {
			return errors.New("rootfs already set")
		}
		c.rootfs = rootfs
		return nil
	}
}

// WithAirValidation enables AIR validation for the compiled binary file.
// If AIR validation is already enabled, it returns an error.
//
// If not set explicitly, AIR validation will be disabled.
func WithAirValidation() CompileOption {
	return func(c *compileCfg) error {
		if c.validateAir {
			return errors.New("air validation already set")
		}
		c.validateAir = true
		return nil
	}
}

// CompiledBinaryFile compiles the embedded R5 interpreter source files into a
// binary file.
//
// It returns the compiled binary file, or an error if the compilation fails.
//
// The compilation can be customized using the provided [CompileOption]. When no
// options are provided, the default configuration is used:
//   - [codegen.DEFAULT_CONFIG] for the code generator configuration.
//   - The embedded R5 interpreter source files as the root filesystem.
//   - No metadata or attributes for the compiled binary file.
//   - No AIR validation.
func CompiledBinaryFile(opts ...CompileOption) (binfile *BinaryFile, err error) {
	cfg := new(compileCfg)
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}
	setDefaults(cfg)

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("failed to compile embedded R5 interpreter: %v", r)
		}
	}()
	srcFiles := arithmetizationSourceFiles(cfg.rootfs)
	macroProgram, _, sErrs := compiler.Compile(field.KOALABEAR_16, cfg.config.GetMaxStaticHeight(), srcFiles...)
	if len(sErrs) > 0 {
		errs := make([]error, len(sErrs))
		for i := range sErrs {
			errs[i] = &sErrs[i]
		}
		return nil, errors.Join(errs...)
	}
	ir, sErrs := ast.Compile(macroProgram, *cfg.config)
	if len(sErrs) > 0 {
		errs := make([]error, len(sErrs))
		for i := range sErrs {
			errs[i] = &sErrs[i]
		}
		return nil, errors.Join(errs...)
	}
	binfile = constraints.NewBinaryFile[koalabear.Element, vm.Uint32](cfg.metadata, cfg.attributes, ir)
	if cfg.validateAir {
		air := binfile.AirConstraints()
		errs := constraints.Validate(air)
		err = errors.Join(errs...)
	}
	return
}

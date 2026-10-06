// Package embedded stores the embedded R5 interpreter implementation.
package embedded

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	arith_src "github.com/LFDT-Lineth/lineth-monorepo/arithmetization/src"
	"github.com/LFDT-Lineth/zkc/pkg/util/field"
	"github.com/LFDT-Lineth/zkc/pkg/util/field/koalabear"
	"github.com/LFDT-Lineth/zkc/pkg/util/source"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler/ast"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler/codegen"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/constraints"
)

const mainDir = "main"
const zkcExt = ".zkc"
const predecodingDir = "predecoding"

// arithmetizationSourceFiles returns the embedded R5 interpreter source files
// as a slice of source.File.
func arithmetizationSourceFiles(rootFs fs.FS) []source.File {
	subFs, err := fs.Sub(rootFs, mainDir)
	if err != nil {
		panic("failed to get sub FS for embedded R5 interpreter: " + err.Error())
	}
	srcFiles := []source.File{}
	if err := fs.WalkDir(subFs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
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
func CompiledBinaryFile(opts ...CompileOption) (binfile *constraints.BinaryFile[koalabear.Element], err error) {
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
	binfile = constraints.NewBinaryFile[koalabear.Element](cfg.metadata, cfg.attributes, ir)
	if cfg.validateAir {
		air := binfile.AirConstraints()
		errs := constraints.Validate(air)
		err = errors.Join(errs...)
	}
	return
}

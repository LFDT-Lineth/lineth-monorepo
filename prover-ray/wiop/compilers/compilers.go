// Package compilers provides the canonical wiop compilation pipeline. Its
// passes run in dependency order and may be configured for each System.
package compilers

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/localvanishing"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/logderivativesum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/lookuptologderivsum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/nonnative"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/rangecheck"
)

// Pass identifies a compilation pass in dependency order.
type Pass uint8

const (
	NonNative Pass = iota
	RangeCheck
	LookupToLogDerivativeSum
	MessageBus
	GrandProduct
	LogDerivativeSum
	LocalVanishing
	Global
	PCS
)

// String names a pass for diagnostics, including unrecognized numeric IDs.
func (p Pass) String() string {
	switch p {
	case NonNative:
		return "NonNative"
	case RangeCheck:
		return "RangeCheck"
	case LookupToLogDerivativeSum:
		return "LookupToLogDerivativeSum"
	case MessageBus:
		return "MessageBus"
	case GrandProduct:
		return "GrandProduct"
	case LogDerivativeSum:
		return "LogDerivativeSum"
	case LocalVanishing:
		return "LocalVanishing"
	case Global:
		return "Global"
	case PCS:
		return "PCS"
	default:
		return fmt.Sprintf("Pass(%d)", p)
	}
}

type hook func(*wiop.System) error

type passHooks struct {
	pre  []hook
	post []hook
}

type hookRegistration struct {
	pass  Pass
	phase string
	hook  hook
}

type options struct {
	pcsOptions        []pcs.Option
	messageBusOptions []messagebus.Option
	withoutPCS        bool
	hooks             []hookRegistration
}

// Option configures a single CompileFull invocation.
type Option func(*options)

// WithPCSOption configures the PCS pass for this System.
func WithPCSOption(option pcs.Option) Option {
	return func(o *options) { o.pcsOptions = append(o.pcsOptions, option) }
}

// WithMessageBusOption configures the message-bus pass for this System.
func WithMessageBusOption(option messagebus.Option) Option {
	return func(o *options) { o.messageBusOptions = append(o.messageBusOptions, option) }
}

// WithoutPCS leaves compiled columns uncommitted and skips both PCS hooks.
func WithoutPCS() Option { return func(o *options) { o.withoutPCS = true } }

// WithPreHook runs hook immediately before pass, even if the pass is a no-op.
// PCS hooks are skipped when WithoutPCS is set.
func WithPreHook(pass Pass, h func(*wiop.System) error) Option {
	return func(o *options) {
		o.hooks = append(o.hooks, hookRegistration{pass: pass, phase: "pre", hook: h})
	}
}

// WithPostHook runs hook immediately after pass, even if the pass is a no-op.
// PCS hooks are skipped when WithoutPCS is set.
func WithPostHook(pass Pass, h func(*wiop.System) error) Option {
	return func(o *options) {
		o.hooks = append(o.hooks, hookRegistration{pass: pass, phase: "post", hook: h})
	}
}

// CompileFull runs all wiop passes in dependency order. Invalid options are
// rejected before any pass runs. A failing hook stops the pipeline, leaving
// mutations from preceding passes intact.
func CompileFull(sys *wiop.System, opts ...Option) error {
	var cfg options
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.withoutPCS && len(cfg.pcsOptions) != 0 {
		return fmt.Errorf("compilers: WithoutPCS conflicts with WithPCSOption")
	}

	var hooks [PCS + 1]passHooks
	for _, registration := range cfg.hooks {
		if registration.pass > PCS {
			return fmt.Errorf("compilers: invalid pass %s", registration.pass)
		}
		if registration.hook == nil {
			return fmt.Errorf("compilers: nil %s hook for %s", registration.phase, registration.pass)
		}
		entry := &hooks[registration.pass]
		if registration.phase == "pre" {
			entry.pre = append(entry.pre, registration.hook)
		} else {
			entry.post = append(entry.post, registration.hook)
		}
	}

	passes := [...]func(*wiop.System){
		nonnative.Compile,
		rangecheck.Compile,
		lookuptologderivsum.Compile,
		func(s *wiop.System) { messagebus.Compile(s, cfg.messageBusOptions...) },
		grandproduct.Compile,
		logderivativesum.Compile,
		localvanishing.Compile,
		global.Compile,
		func(s *wiop.System) { pcs.Compile(s, cfg.pcsOptions...) },
	}
	for index, compile := range passes {
		pass := Pass(index)
		if pass == PCS && cfg.withoutPCS {
			break
		}
		for _, h := range hooks[pass].pre {
			if err := h(sys); err != nil {
				return fmt.Errorf("compilers: %s pre-hook: %w", pass, err)
			}
		}
		compile(sys)
		for _, h := range hooks[pass].post {
			if err := h(sys); err != nil {
				return fmt.Errorf("compilers: %s post-hook: %w", pass, err)
			}
		}
	}
	return nil
}

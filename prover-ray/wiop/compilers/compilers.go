// Package compilers provides the canonical wiop compilation pipeline. Its
// passes run in dependency order and may be configured for each System.
package compilers

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
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

// Option configures a [Configure] or [CompileFull] invocation. Both entry
// points share this type: the configuration options ([ConfigurePCS],
// [ConfigureMessageBus], [WithoutPCS]) select a System's pass settings, while
// [WithPreHook] and [WithPostHook] are pipeline-only and rejected by
// [Configure].
type Option func(*options)

// ConfigurePCS configures the PCS pass for this System. The options are
// merged with any previously configured selection; conflicting or repeated
// choices are rejected before the System is mutated. At least one option is
// required, and [WithoutPCS] conflicts with it.
func ConfigurePCS(opts ...pcs.Option) Option {
	if len(opts) == 0 {
		utils.Panic("compilers: ConfigurePCS requires at least one pcs option")
	}
	return func(o *options) { o.pcsOptions = append(o.pcsOptions, opts...) }
}

// ConfigureMessageBus configures the message-bus pass for this System. The
// options are merged with any previously configured selection; conflicting
// choices are rejected before the System is mutated. At least one option is
// required.
func ConfigureMessageBus(opts ...messagebus.Option) Option {
	if len(opts) == 0 {
		utils.Panic("compilers: ConfigureMessageBus requires at least one messagebus option")
	}
	return func(o *options) { o.messageBusOptions = append(o.messageBusOptions, opts...) }
}

// WithoutPCS declares the System PCS-less ([wiop.System.DeclareNoPCS]). In
// [CompileFull] it leaves compiled columns uncommitted and skips both the PCS
// pass and its hooks. It conflicts with [ConfigurePCS] and with a previously
// configured PCS selection.
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

// resolve applies opts and validates them as a whole: cross-option
// contradictions, invalid passes and nil hooks are rejected. It returns the
// resolved configuration and the per-pass hook table.
func resolve(opts []Option, allowHooks bool) (*options, *[PCS + 1]passHooks, error) {
	var cfg options
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.withoutPCS && len(cfg.pcsOptions) != 0 {
		return nil, nil, fmt.Errorf("compilers: WithoutPCS conflicts with ConfigurePCS")
	}
	if len(cfg.hooks) != 0 && !allowHooks {
		return nil, nil, fmt.Errorf("compilers: hooks are only accepted by CompileFull")
	}
	var hooks [PCS + 1]passHooks
	for _, registration := range cfg.hooks {
		if registration.pass > PCS {
			return nil, nil, fmt.Errorf("compilers: invalid pass %s", registration.pass)
		}
		if registration.hook == nil {
			return nil, nil, fmt.Errorf("compilers: nil %s hook for %s", registration.phase, registration.pass)
		}
		entry := &hooks[registration.pass]
		if registration.phase == "pre" {
			entry.pre = append(entry.pre, registration.hook)
		} else {
			entry.post = append(entry.post, registration.hook)
		}
	}
	return &cfg, &hooks, nil
}

// apply records the resolved pass selections on sys. Every selection is
// validated against the System's existing ones before any is written, so a
// conflicting option set leaves the System untouched.
func (cfg *options) apply(sys *wiop.System) error {
	if len(cfg.pcsOptions) != 0 {
		if err := pcs.Validate(sys, cfg.pcsOptions...); err != nil {
			return fmt.Errorf("compilers: %w", err)
		}
	}
	if len(cfg.messageBusOptions) != 0 {
		if err := messagebus.Validate(sys, cfg.messageBusOptions...); err != nil {
			return fmt.Errorf("compilers: %w", err)
		}
	}
	if cfg.withoutPCS && sys.PCSMode() == 1 {
		return fmt.Errorf("compilers: WithoutPCS conflicts with a previously configured PCS selection")
	}

	// Every remaining selection was validated: none of these can fail against
	// the same System state.
	if len(cfg.pcsOptions) != 0 {
		if err := pcs.Apply(sys, cfg.pcsOptions...); err != nil {
			return fmt.Errorf("compilers: %w", err)
		}
	}
	if len(cfg.messageBusOptions) != 0 {
		if err := messagebus.Apply(sys, cfg.messageBusOptions...); err != nil {
			return fmt.Errorf("compilers: %w", err)
		}
	}
	// resolve rejects WithoutPCS together with ConfigurePCS, so no PCS
	// selection was applied above.
	if cfg.withoutPCS {
		sys.DeclareNoPCS()
	}
	return nil
}

// Configure records the configuration options on sys without running any
// pass. It accepts the same configuration options as [CompileFull] but
// rejects hooks. The selections are stored on the System so that standalone
// passes, and a later [CompileFull], consume them: defaults stay unset until
// the first consuming pass freezes them, and a pass receiving previously
// configured values merges them instead of overriding. Conflicting or invalid
// options are returned as errors before sys is mutated.
//
// Typical standalone use:
//
//	compilers.Configure(sys,
//		compilers.ConfigurePCS(pcs.WithFRINumQueries(115)),
//		compilers.ConfigureMessageBus(messagebus.WithoutSharedRandomness()),
//	)
//	messagebus.Compile(sys)
//	// ... remaining passes ...
//	pcs.Compile(sys)
func Configure(sys *wiop.System, opts ...Option) error {
	cfg, _, err := resolve(opts, false)
	if err != nil {
		return err
	}
	return cfg.apply(sys)
}

// CompileFull runs all wiop passes in dependency order. The configuration
// options are recorded on sys as by [Configure] before the first pass runs;
// invalid options, including contradictions with an earlier [Configure], are
// rejected before any pass runs. A failing hook stops the pipeline, leaving
// mutations from preceding passes intact.
func CompileFull(sys *wiop.System, opts ...Option) error {
	cfg, hooks, err := resolve(opts, true)
	if err != nil {
		return err
	}
	if err := cfg.apply(sys); err != nil {
		return err
	}

	// The message-bus and PCS passes consume the selections apply recorded.
	passes := [...]func(*wiop.System){
		nonnative.Compile,
		rangecheck.Compile,
		lookuptologderivsum.Compile,
		func(s *wiop.System) { messagebus.Compile(s) },
		grandproduct.Compile,
		logderivativesum.Compile,
		localvanishing.Compile,
		global.Compile,
		func(s *wiop.System) { pcs.Compile(s) },
	}
	for index, compile := range passes {
		pass := Pass(index)
		if pass == PCS && sys.PCSMode() == 0 {
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

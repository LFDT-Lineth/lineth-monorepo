package fft

import (
	"runtime"
)

// Option defines option for altering the behavior of FFT methods.
// See the descriptions of functions returning instances of this type for
// particular options.
type Option func(fftConfig) fftConfig

type fftConfig struct {
	coset   bool
	nbTasks int
}

// OnCoset if provided, FFT(a) returns the evaluation of a on a coset.
func OnCoset() Option {
	return func(opt fftConfig) fftConfig {
		opt.coset = true
		return opt
	}
}

// WithNbTasks sets the max number of task (go routine) to spawn. Must be between 1 and 512.
func WithNbTasks(nbTasks int) Option {
	if nbTasks < 1 {
		nbTasks = 1
	} else if nbTasks > 512 {
		nbTasks = 512
	}
	return func(opt fftConfig) fftConfig {
		opt.nbTasks = nbTasks
		return opt
	}
}

// default options
func fftOptions(opts ...Option) fftConfig {
	// apply options
	opt := fftConfig{
		coset:   false,
		nbTasks: runtime.NumCPU(),
	}
	for _, option := range opts {
		opt = option(opt)
	}
	return opt
}

func EmptyOption() Option {
	return func(config fftConfig) fftConfig {
		return config
	}
}

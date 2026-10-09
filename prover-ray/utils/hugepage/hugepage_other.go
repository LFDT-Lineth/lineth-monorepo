//go:build !linux

package hugepage

func advise(_, _ uintptr) {}

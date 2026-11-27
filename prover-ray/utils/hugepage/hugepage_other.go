//go:build !linux

package hugepage

import "unsafe"

func advise(_ unsafe.Pointer, _ uintptr) {}

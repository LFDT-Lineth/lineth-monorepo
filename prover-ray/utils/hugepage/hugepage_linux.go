//go:build linux

package hugepage

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

func advise(p unsafe.Pointer, length uintptr) {
	_ = unix.Madvise(unsafe.Slice((*byte)(p), length), unix.MADV_HUGEPAGE)
}

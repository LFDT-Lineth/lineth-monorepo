//go:build linux

package hugepage

import "syscall"

func advise(addr, length uintptr) {
	_, _, _ = syscall.Syscall(syscall.SYS_MADVISE, addr, length, syscall.MADV_HUGEPAGE)
}

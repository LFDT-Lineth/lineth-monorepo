//go:build linux

package hugepage

import "golang.org/x/sys/unix"

// advise asks for transparent huge pages over b, whose bounds are huge-page
// aligned. The advice is a hint: its error (e.g. EINVAL on a kernel built
// without THP) is ignored.
func advise(b []byte) {
	_ = unix.Madvise(b, unix.MADV_HUGEPAGE)
}

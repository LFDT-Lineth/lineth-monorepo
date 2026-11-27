// Package hugepage advises the kernel to back large buffers with transparent
// huge pages.
//
// The prover allocates many large short-lived vectors, and every CPU touches
// fresh ones at the same time; with 4 KiB pages the kernel then spends more
// time servicing the page faults, under contention, than the prover spends
// computing. Where transparent huge pages are enabled on request (the
// "madvise" mode), advising a buffer before its first touch makes each fault
// map 2 MiB instead of 4 KiB.
package hugepage

import "unsafe"

// hugePageSize is the transparent huge page size with 4 KiB base pages:
// amd64, and arm64 kernels built with 4 KiB pages. With larger base pages
// the huge pages are larger, and the advice covers fewer of them.
const hugePageSize = 2 << 20

// Advise advises the kernel to back the huge pages entirely inside v with
// transparent huge pages. It is a hint: it never fails, and has no effect on
// buffers smaller than a huge page, on platforms without the advice, or on
// pages already faulted in.
//
// The gain depends on the host's transparent huge page mode
// (/sys/kernel/mm/transparent_hugepage/enabled): with "madvise" only advised
// ranges get huge pages, with "always" every large enough range already does
// and the advice changes nothing, and with "never" it is ignored. Each advised
// range also becomes its own kernel memory area (VMA), splitting the area
// around it; the number of areas this adds is bounded by the heap size over
// 2 MiB.
func Advise[T any](v []T) {
	if len(v) == 0 {
		return
	}
	base := unsafe.Pointer(unsafe.SliceData(v))
	start := uintptr(base)
	end := start + uintptr(len(v))*unsafe.Sizeof(v[0])
	lo := (start + hugePageSize - 1) &^ (hugePageSize - 1)
	hi := end &^ (hugePageSize - 1)
	if hi <= lo {
		return
	}
	advise(unsafe.Add(base, lo-start), hi-lo)
}

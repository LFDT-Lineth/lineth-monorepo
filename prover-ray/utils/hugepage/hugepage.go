// Package hugepage advises the kernel to back large buffers with transparent
// huge pages.
//
// The prover allocates many large short-lived vectors, and every CPU touches
// fresh ones at the same time; with 4 KiB pages the kernel then spends more
// time servicing the page faults, under contention, than the prover spends
// computing. Where transparent huge pages are enabled on request (the
// "madvise" mode), advising a buffer before its first touch makes each fault
// map 2 MiB instead of 4 KiB.
//
// The gain depends on the host's THP mode
// (/sys/kernel/mm/transparent_hugepage/enabled): under "madvise" it comes
// from this advice; under "always" the kernel uses huge pages anyway and the
// advice changes nothing; under "never" the advice is ignored. Each advice
// also splits the VMA holding the range (adjacent advised ranges merge back),
// so the number of VMAs it creates is bounded by the heap size over 2 MiB:
// about 11k for a 22 GiB heap, under the kernel's default vm.max_map_count
// of 65530, but a heap above 128 GiB could approach that limit.
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
	advise(unsafe.Slice((*byte)(unsafe.Add(base, lo-start)), hi-lo))
}

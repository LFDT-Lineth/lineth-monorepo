/* Minimal <string.h> for the freestanding rv64 guest.
   zstd's zstd_deps.h includes <string.h> unconditionally; the guest target has
   no libc, so this declares only the three functions zstd actually uses. They
   are defined in main.zig (word-at-a-time, overriding compiler-rt's byte
   loops). With a static DCtx zstd needs no malloc, so no <stdlib.h> shim is
   required. */
#ifndef ZSTD_GUEST_SHIM_STRING_H
#define ZSTD_GUEST_SHIM_STRING_H
#include <stddef.h>
void *memcpy(void *dst, const void *src, size_t n);
void *memmove(void *dst, const void *src, size_t n);
void *memset(void *s, int c, size_t n);
#endif

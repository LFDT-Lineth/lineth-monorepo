/* Minimal <stdlib.h> for the freestanding rv64 guest.
   zstd includes this via allocations.h even when a static DCtx is used, so the
   declarations must exist for the translation units to compile. The
   definitions in main.zig always fail to allocate: with ZSTD_initStaticDCtx
   nothing should call them, and if anything does, zstd reports a memory error
   rather than proceeding on a bad pointer. */
#ifndef ZSTD_GUEST_SHIM_STDLIB_H
#define ZSTD_GUEST_SHIM_STDLIB_H
#include <stddef.h>
void *malloc(size_t size);
void *calloc(size_t n, size_t size);
void free(void *ptr);
#endif

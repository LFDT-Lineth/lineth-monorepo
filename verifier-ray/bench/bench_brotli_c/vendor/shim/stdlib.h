/* Minimal <stdlib.h>. brotli's malloc/free calls all go through the
   alloc_func/free_func passed to BrotliDecoderCreateInstance -- the guest
   supplies a bump allocator (main.zig) rather than these -- but platform.h's
   IWYU pragma still expects the declarations to exist. exit() is declared but
   never reachable in decode-only builds; guest.exit_stub in main.zig traps if
   it somehow were. */
#ifndef BROTLI_GUEST_SHIM_STDLIB_H
#define BROTLI_GUEST_SHIM_STDLIB_H
#include <stddef.h>
void *malloc(size_t size);
void free(void *ptr);
_Noreturn void exit(int status);
#endif

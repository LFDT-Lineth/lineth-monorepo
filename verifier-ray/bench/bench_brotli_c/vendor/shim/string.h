/* Minimal <string.h> for the freestanding rv64 guest. brotli's platform.h
   includes this unconditionally; only memcpy/memcmp/memset are used. */
#ifndef BROTLI_GUEST_SHIM_STRING_H
#define BROTLI_GUEST_SHIM_STRING_H
#include <stddef.h>
void *memcpy(void *dst, const void *src, size_t n);
int memcmp(const void *a, const void *b, size_t n);
void *memset(void *s, int c, size_t n);
#endif

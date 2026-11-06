/* <string.h> for the freestanding guest: the mem* family comes from the compiler runtime. */
#ifndef LINETH_FREESTANDING_STRING_H
#define LINETH_FREESTANDING_STRING_H

#include <stddef.h>

void *memcpy(void *dest, const void *src, size_t size);
void *memmove(void *dest, const void *src, size_t size);
void *memset(void *dest, int value, size_t size);
int memcmp(const void *a, const void *b, size_t size);

#endif

/* <stdlib.h> for the freestanding guest: libzstd allocates through the ZSTD_customMem it is given,
 * so its default allocation path reports exhaustion. */
#ifndef LINETH_FREESTANDING_STDLIB_H
#define LINETH_FREESTANDING_STDLIB_H

#include <stddef.h>

#define malloc(size) ((void)(size), (void *)0)
#define calloc(count, size) ((void)(count), (void)(size), (void *)0)
#define free(ptr) ((void)(ptr))

#endif

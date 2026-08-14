/* Assertions compile out: NDEBUG is set and a freestanding target has no
   abort() or stderr to report through. */
#ifndef BROTLI_GUEST_SHIM_ASSERT_H
#define BROTLI_GUEST_SHIM_ASSERT_H
#define assert(x) ((void)0)
#endif

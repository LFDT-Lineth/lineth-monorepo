/* Assertions compile out in the guest: DEBUGLEVEL=0 and NDEBUG are set, and a
   freestanding target has no abort() or stderr to report through. */
#ifndef ZSTD_GUEST_SHIM_ASSERT_H
#define ZSTD_GUEST_SHIM_ASSERT_H
#define assert(x) ((void)0)
#endif

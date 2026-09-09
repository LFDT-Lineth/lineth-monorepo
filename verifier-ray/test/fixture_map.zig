//! Shared mmap helper for tests that load a Go-encoded proof/pair image from
//! disk and cast it directly to a verifier-ray input type, at a fixed guest
//! address. Used by riscv_proof_image_test.zig (VerifyInput) and
//! aggregator_image_test.zig (AggregatorInput) — the two share the exact same
//! open/lseek/mmap/skip-on-error contract, only `T`, the fixture path, and the
//! base address differ.

const o_rdonly: c_int = 0;
const prot_read: c_int = 1;
const map_private: c_int = 2;
// MAP_FIXED_NOREPLACE (not MAP_FIXED): these tests share a process with every
// other test in test/all.zig, and Zig randomizes test order per run, so
// whatever else has already been placed in the address space by the time a
// given test runs varies run to run. Plain MAP_FIXED would silently overlap
// (and corrupt) anything already mapped at the fixture's base, producing a
// non-reproducible verify() outcome keyed to test order/ASLR rather than to
// the input actually under test. _NOREPLACE fails the syscall instead —
// surfaced below as MapFixedUnavailable, the same skip path already used for
// the "environment refuses low-address mappings" case.
const map_fixed_noreplace: c_int = 0x10 | 0x100000;
const seek_end: c_int = 2;
const map_failed = ~@as(usize, 0);

extern fn open(path: [*:0]const u8, flags: c_int) c_int;
extern fn mmap(address: ?*anyopaque, length: usize, prot: c_int, flags: c_int, fd: c_int, offset: i64) *anyopaque;
extern fn close(fd: c_int) c_int;
extern fn lseek(fd: c_int, offset: i64, whence: c_int) i64;

pub const MapFixtureError = error{ ImageMissing, MapFixedUnavailable };

/// Opens `path`, mmaps its full contents MAP_FIXED_NOREPLACE at `base`, and
/// casts the mapping to `*const T`. Callers should treat both error cases as
/// `error.SkipZigTest`: `ImageMissing` means the fixture hasn't been
/// generated yet (`make generate-testdata`), and `MapFixedUnavailable` means
/// either the environment refuses low-address fixed mappings, or `base` was
/// already occupied by another test in this process — neither is a failure
/// of the image format itself.
pub fn mapFixtureImage(comptime T: type, path: [*:0]const u8, base: usize) MapFixtureError!*const T {
    const fd = open(path, o_rdonly);
    if (fd < 0) return error.ImageMissing;
    defer _ = close(fd);

    const image_len = lseek(fd, 0, seek_end);
    if (image_len <= 0) return error.ImageMissing;

    const p = mmap(@ptrFromInt(base), @intCast(image_len), prot_read, map_private | map_fixed_noreplace, fd, 0);
    if (@intFromPtr(p) == map_failed) return error.MapFixedUnavailable;

    return @ptrCast(@alignCast(p));
}

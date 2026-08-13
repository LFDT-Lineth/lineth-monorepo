// Micro-benchmark: RISC-V cycle cost of reading the read-only IN region versus
// reading and writing RAM.
//
// Why this exists. A decompressor can either PRODUCE its output into RAM, or be
// handed the claimed output in the read-only IN region and merely VERIFY it.
// Verify-mode is attractive because the decompressed payload is already a
// committed input -- it is what gets hashed for the public input and checked
// against the execution proof -- so supplying it costs nothing extra in trust,
// and it would move ~780 kB of output buffer out of writable memory entirely.
// It also worked well in the Plonk circuit this stack replaces.
//
// The whole idea rests on one assumption: that reads from IN are cheaper than
// read-write traffic to RAM. That is plausible (IN is `(r)` in the linker
// script, so it can in principle be committed once as preprocessed data rather
// than carrying a full read-write consistency argument) but it is an assumption
// about zkC's memory cost model, not a fact we have measured. If IN reads and
// RAM writes cost the same, verify-mode buys nothing and is not worth the
// decoder surgery.
//
// The three loops below mirror the three access patterns an LZ decoder actually
// performs, so the numbers translate directly:
//
//   rom_read     - literal verification: read IN, compare
//   ram_write    - literal production:   write RAM
//   ram_copy     - match production:     read RAM, write RAM (the backref loop)
//
// Marker IDs:
//    0 = start baseline,   1 = end baseline
//   10 = start rom_read,  11 = end rom_read
//   20 = start ram_write, 21 = end ram_write
//   30 = start ram_copy,  31 = end ram_copy
//
// The baseline loop has the same shape with an empty body so the runner can
// subtract loop-counter and branch overhead.

const verifier_ray = @import("verifier_ray");
const accel = @import("lineth_accelerators");
const profiling = verifier_ray.profiling;

const N: u64 = 4096;

// Base of the read-only IN region, per riscv-guests/build_common/linker_script.ld.
const IN_BASE: usize = 0x08800000;

// Destination buffer in .bss, i.e. the writable PROGRAM region. This is what a
// produce-mode decoder would write its output into.
var ram_buf: [N]u8 = @splat(0);

pub export fn main() noreturn {
    const rom: [*]const volatile u8 = @ptrFromInt(IN_BASE);
    var checksum: u64 = 0;
    var i: u64 = 0;

    // Baseline: same loop shape, empty body.
    profiling.markR5Value(0, 0);
    while (i < N) : (i += 1) {
        asm volatile ("" ::: .{ .memory = true });
    }
    profiling.markR5Value(1, 0);

    // rom_read: one read from IN per iteration, accumulated so it cannot be
    // optimized away. This is what verify-mode does per literal byte.
    profiling.markR5Value(10, 0);
    i = 0;
    while (i < N) : (i += 1) {
        checksum +%= rom[i];
    }
    profiling.markR5Value(11, checksum);

    // ram_write: one write to RAM per iteration. Produce-mode, per literal byte.
    profiling.markR5Value(20, 0);
    i = 0;
    while (i < N) : (i += 1) {
        ram_buf[i] = @truncate(i);
    }
    profiling.markR5Value(21, 0);

    // ram_copy: read RAM and write RAM, the shape of a backref copy. Offset by
    // one so it mirrors the overlapping-match case, which is the common one.
    profiling.markR5Value(30, 0);
    i = 1;
    while (i < N) : (i += 1) {
        ram_buf[i] = ram_buf[i - 1];
    }
    profiling.markR5Value(31, ram_buf[N - 1]);

    accel.zkvm_exit(0);
}

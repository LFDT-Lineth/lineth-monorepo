// zkVM standard io-interface: read_input / write_output
// https://github.com/eth-act/zkvm-standards/tree/main/standards/io-interface

// The linker-defined start of the IN memory region the proving system maps the private input
// into. The region begins with an 8-byte little-endian u64 payload length, followed by that many
// payload bytes.
extern var _input_start: u64;

/// Size of the linker-defined IN memory region (1 GiB).
const IN_REGION_SIZE: usize = 0x40000000;

/// File descriptor for the logging output channel.  Data written to
/// this channel does not form part of the guest's public output, and
/// may not be traced at all.  However, it is made available after
/// program execution.
const GUEST_LOG: u64 = 3;
/// File descriptor for the public output channel.  Data written to
/// this channel forms the guest's public output and, as such, forms
/// part of the final proof.
const GUEST_OUTPUT: u64 = 4;

/// Read the private input from the memory-mapped IN region: hands back a zero-copy slice into
/// that region via `buf_ptr`/`buf_size`. The input slot's address is this module's own detail —
/// a guest entry point only ever sees the slice.
pub fn read_input(buf_ptr: *[*]const u8, buf_size: *usize) void {
    const payload_len = _input_start;
    // A length word claiming more than the IN region can hold means the input slot itself is
    // corrupt — an ABI-level integrity violation, not a malformed message — so this fails the
    // execution outright rather than handing the guest a slice it could misclassify as a
    // decodable-but-invalid input.
    if (payload_len > IN_REGION_SIZE - @sizeOf(u64)) @panic("input payload_len exceeds IN region");

    const input_bytes: [*]const u8 = @ptrCast(&_input_start);
    buf_ptr.* = input_bytes + @sizeOf(u64);
    buf_size.* = @intCast(payload_len);
}

// Write `size` bytes read from the buffer at `output` to the guest's
// output and log.  More specifically the first 32 bytes are written
// to the guest's public output and the remainder are logged.
//
// Signature matches the extern zesu declares for the standard `write_output` name:
//   @extern(*const fn ([*]const u8, usize) callconv(.c) void, .{ .name = "write_output" })
pub fn write_output(output: [*]const u8, size: usize) callconv(.c) void {
    // sanity check at least 32 bytes given
    if (size < 32) @panic("write_output: payload shorter than 32-byte commitment");
    // Write output then log
    write_guest_output(GUEST_OUTPUT, output[0..32], 32);
    write_guest_output(GUEST_LOG, output[32..], size - 32);
}

// Write `size` bytes read from the buffer at `output` to the given
// file descriptor.
pub fn write_guest_output(fd: u64, output: [*]const u8, size: usize) callconv(.c) void {
    asm volatile (
        \\mv a0, %[fd]
        \\mv a1, %[output]
        \\mv a2, %[size]
        \\li a7, 64
        \\ecall
        :
        : [fd] "r" (fd),
          [output] "r" (@intFromPtr(output)),
          [size] "r" (size),
        : .{ .a0 = true, .a1 = true, .a2 = true, .a7 = true, .memory = true });
}

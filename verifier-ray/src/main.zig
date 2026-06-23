const builtin = @import("builtin");
const verifier_ray = @import("verifier_ray");

const field = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;
const polynomial = verifier_ray.polynomial.canonical;
const poseidon2 = verifier_ray.crypto.poseidon2;
const Transcript = verifier_ray.crypto.fiat_shamir.Transcript;

const is_r5_zkvm = builtin.target.cpu.arch == .riscv64 and builtin.target.os.tag == .freestanding;
const is_native_os = builtin.target.os.tag == .linux or builtin.target.os.tag == .macos;
const is_native_arch = builtin.target.cpu.arch == .x86_64 or builtin.target.cpu.arch == .aarch64;
const is_supported_native = is_native_os and is_native_arch;

const Commitment = verifier_ray.crypto.commitment.Commitment;
const Digest = poseidon2.Digest;

// Temporary smoke-test fixture shape. These values are currently hand-picked
// so the native and R5 paths exercise non-trivial verifier code; they should
// come from prover-ray metadata once we wire in realistic proofs.
const commitment_count = 1;
const commitment_limb_count = 8;
const public_input_count = 2;
const proof_byte_count = 3;
const coefficient_count = 4;
const digest_limb_count = 8;

const native_input_path: [:0]const u8 = "zig-out/input.bin";

extern const _in_start: u8;

// Input is cast directly from raw bytes in both native mmap and R5 linked-memory paths.
// Keep declaration-order layout stable for the binary fixtures.
const Input = extern struct {
    commitments: [commitment_count]Commitment,
    public_inputs: [public_input_count]field.Element,
    proof_bytes: [proof_byte_count]u8,
    coefficients: [coefficient_count]field.Element,
    point: ext.Ext,
    expected_challenge: ext.Ext,
};

const raw_input_len = @sizeOf(Input);

// The main entry point for the verifier ray smoke test. This is separate from
// the main verifier entry point in `verifier.zig` because we want to be able to
// run this smoke test in both native and R5 zkVM environments, and the way we load
// input and exit differs between those environments. The actual verifier logic
// being tested is still in `verifier.zig`, and this main function just serves as a
// thin wrapper around it to handle environment-specific details.
pub fn main() noreturn {
    if (comptime is_r5_zkvm) {
        // this entry point should only be called from native build (`make build` or `make build-release`)
        unreachable;
    }
    if (comptime !is_supported_native) {
        @compileError("native verifier libc path currently supports x86_64/aarch64 Linux and macOS only");
    }

    const input = loadNativeInput();
    exitNative(runVerifierSmoke(input));
}

// The main entry point for the R5 zkVM smoke test. This is separate from the
// native main function because we need to use a different method for loading input
// and exiting in the R5 zkVM environment. The actual verifier logic being tested
// is still in `verifier.zig`, and this main function just serves as a thin wrapper
// around it to handle R5-specific details.
pub export fn r5_main() noreturn {
    if (comptime !is_r5_zkvm) {
        // this entry point should only be called from R5 zkVM build (`make build-r5` or `make build-r5-release`)
        unreachable;
    }

    // the input is linked into the binary at compile time using the
    // `_in_start` symbol defined in the linker script, so we can just take its
    // address and cast it to our structured input type
    const input: *const Input = @ptrCast(@alignCast(&_in_start));

    // run the verifier smoke test with the loaded input
    const res = runVerifierSmoke(input);
    exitR5(res);
}

fn runVerifierSmoke(input: *const Input) u8 {
    // some temporary work to exercise the zkVM trace with a realistic
    // polynomial evaluation and Fiat-Shamir transcript interaction, before we have
    // a real proof to verify. This will be removed once we have a real proof and
    // can test the full verifier end-to-end in the smoke test.
    if (!exerciseTemporaryTraceWork(input)) {
        return 1;
    }

    return 0;
}

fn exerciseTemporaryTraceWork(input: *const Input) bool {
    // Temporary zkVM trace exercise. Remove this once main verifies a realistic proof.
    const evaluation = polynomial.evaluateBaseAtExt(input.coefficients[0..], input.point);

    var transcript = Transcript.init();
    transcript.updateElements(input.coefficients[0..]);
    transcript.updateExt(&.{evaluation});
    const challenge = transcript.randomExt();

    return challenge.eql(input.expected_challenge);
}

// Native smoke tests use the same fixed binary input image as the R5 linked-memory path.
// The Makefile places that image at `native_input_path`, so native execution only needs a
// small libc surface: open the file, mmap exactly `@sizeOf(Input)`, and cast the bytes to
// `Input`. Avoiding std file/argument handling keeps ReleaseSmall native binaries compact.
const o_rdonly: c_int = 0;
const prot_read: c_int = 1;
const map_private: c_int = 2;
const map_failed = ~@as(usize, 0);

extern fn open(path: [*:0]const u8, flags: c_int) c_int;
extern fn mmap(address: ?*anyopaque, length: usize, protection: c_int, flags: c_int, fd: c_int, offset: i64) *anyopaque;
extern fn _exit(status: c_int) noreturn;

fn loadNativeInput() *const Input {
    if (comptime !is_supported_native) {
        @compileError("native verifier libc path currently supports x86_64/aarch64 Linux and macOS only");
    }

    const fd = open(native_input_path.ptr, o_rdonly);
    if (fd < 0) exitNative(1);

    const mapped_addr = mmap(null, raw_input_len, prot_read, map_private, fd, 0);
    if (@intFromPtr(mapped_addr) == map_failed) exitNative(1);

    const mapped_bytes: [*]const u8 = @ptrCast(mapped_addr);
    return @ptrCast(@alignCast(mapped_bytes));
}

fn exitNative(code: u8) noreturn {
    if (comptime !is_supported_native) {
        @compileError("native verifier libc exit currently supports x86_64/aarch64 Linux and macOS only");
    }

    _exit(@intCast(code));
}

fn exitR5(code: u8) noreturn {
    if (comptime !is_r5_zkvm) {
        @compileError("R5 exit currently supports only R5 zkVM target");
    }
    switch (code) {
        0 => exitR5Success(),
        else => exitR5Failure(),
    }
}

fn exitR5Success() noreturn {
    asm volatile (
        \\li a0, 0
        \\li a7, 93
        \\ecall
    );
    unreachable;
}

fn exitR5Failure() noreturn {
    asm volatile (
        \\li a0, 1
        \\li a7, 93
        \\ecall
    );
    unreachable;
}

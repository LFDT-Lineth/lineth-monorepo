const builtin = @import("builtin");
const verifier_ray = @import("verifier_ray");
const embedded_data = @import("embedded_data");
const embedded_data_conf = @import("embedded_data_config");
const main_config = @import("main_config");
const riscv_system = @import("riscv_system");
const lineth_accel = @import("lineth_accelerators");

const verifier = verifier_ray.verifier;

const is_r5_zkvm = verifier_ray.r5_config.is_r5_zkvm;
const is_native_os = builtin.target.os.tag == .linux or builtin.target.os.tag == .macos;
const is_native_arch = builtin.target.cpu.arch == .x86_64 or builtin.target.cpu.arch == .aarch64;
const is_supported_native = is_native_os and is_native_arch;

// Aggregator mode (`-Daggregator=true`): the input region carries an
// aggregator pair image (verifier.AggregatorInput: two proofs of the real
// riscv system), and the binary verifies BOTH proofs plus their public-input
// consistency instead of one proof. Proving THIS guest's execution under the
// R5 zkVM is what turns the pair into one proof attesting to both.
const aggregator_mode = main_config.aggregator;

comptime {
    // The embedded fixtures are single synthetic proofs against their own
    // embedded systems; the aggregator verifies a pair against the real riscv
    // system. Combining the two flags has no meaningful semantics.
    if (aggregator_mode and embedded_data_conf.embed_input)
        @compileError("aggregator mode loads a pair image; -Dembedded-input does not apply");
}

const native_input_path: [:0]const u8 = if (aggregator_mode)
    "testdata/riscv_proof_pair_image.bin"
else
    "testdata/riscv_proof_image.bin";
const input_guest_base: usize = 0x08800000;

extern const _in_start: u8;

// When the input is embedded at build time, the fixture proof is materialized
// into static (.rodata) memory here so the loaders can hand out a runtime
// pointer to it, exactly like the mmap/linker paths do. This keeps the bundled
// verifier input as a plain runtime value — only `spec`/`systems` are comptime
// in `verify`. The const is lazily analyzed, so it costs nothing when
// `embed_input` is false.
const embedded_input: verifier.VerifyInput = if (embedded_data_conf.invalid_input)
    embedded_data.getInputFailing(embedded_data_conf.spec_index)
else
    embedded_data.getInput(embedded_data_conf.spec_index);

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

    if (comptime aggregator_mode) {
        const image = mapNativeImage() orelse exitNative(1);
        const input: *const verifier.AggregatorInput = @ptrCast(@alignCast(image));
        exitNative(runAggregator(input));
    }

    const input = loadNativeInput();
    exitNative(runVerifier(input));
}

// The main entry point for the R5 zkVM smoke test. This is separate from the
// native main function because we need to use a different method for loading input
// and exiting in the R5 zkVM environment. The actual verifier logic being tested
// is still in `verifier.zig`, and this main function just serves as a thin wrapper
// around it to handle R5-specific details.
fn r5_main() callconv(.c) noreturn {
    if (comptime !is_r5_zkvm) {
        // this entry point should only be called from R5 zkVM build (`make build-r5` or `make build-r5-release`)
        unreachable;
    }

    if (comptime aggregator_mode) {
        // The zkc JSON input writer places the pair image bytes directly at
        // `_in_start`, already relocated for GuestBase — same contract as the
        // single-proof image, with AggregatorInput as the root instead.
        const input: *const verifier.AggregatorInput = @ptrCast(@alignCast(&_in_start));
        exitR5(runAggregator(input));
    }

    // load the input depending on the running mode (embedded by the zkVM or at compile time)
    const input = loadR5Input();

    // run the verifier smoke test with the loaded input
    const res = runVerifier(input);
    exitR5(res);
}

// We have standard entry point convention for R5 zkvm. Export the symbol so that the linker can find it.
comptime {
    if (is_r5_zkvm) {
        @export(&r5_main, .{ .name = "main" });
    }
}

// The aggregator: verify BOTH proofs of the pair against the real compiled
// riscv system, and reject unless their public-input statements agree.
// Everything protocol-level lives in `verifier.verifyPair`; this wrapper only
// dereferences the image's two absolute pointers. There is no embedded-fixture
// variant (see the comptime exclusion above), so the spec/systems are always
// the generated riscv system's.
fn runAggregator(input: *const verifier.AggregatorInput) u8 {
    verifier.verifyPair(
        riscv_system.system_0_spec,
        riscv_system.system_0_systems,
        input.a.*,
        input.b.*,
    ) catch return 1;
    return 0;
}

fn runVerifier(input: *const verifier.VerifyInput) u8 {
    const spec = if (comptime embedded_data_conf.embed_input)
        comptime embedded_data.get(embedded_data_conf.spec_index).spec
    else
        riscv_system.system_0_spec;
    const systems = if (comptime embedded_data_conf.embed_input)
        comptime embedded_data.get(embedded_data_conf.spec_index).systems
    else
        riscv_system.system_0_systems;
    // `spec`/`systems` are comptime, but the verifier input is a runtime value
    // read from `input` (mmap/linker/embedded memory), so dereference it here.
    verifier.verify(spec, systems, input.proof, input.public_inputs) catch {
        // if the verifier fails, return a non-zero exit code
        return 1;
    };
    return 0; // success
}

// Native smoke tests use the same fixed binary input image as the R5 linked-memory path.
// The Makefile places that image at `native_input_path`, so native execution only needs a
// small libc surface: open the file, mmap exactly `@sizeOf(Input)`, and cast the bytes to
// `Input`. Avoiding std file/argument handling keeps ReleaseSmall native binaries compact.
const o_rdonly: c_int = 0;
const prot_read: c_int = 1;
const map_private: c_int = 2;
const map_fixed: c_int = 0x10;
const seek_end: c_int = 2;
const map_failed = ~@as(usize, 0);

extern fn open(path: [*:0]const u8, flags: c_int) c_int;
extern fn close(fd: c_int) c_int;
extern fn lseek(fd: c_int, offset: i64, whence: c_int) i64;
extern fn mmap(address: ?*anyopaque, length: usize, protection: c_int, flags: c_int, fd: c_int, offset: i64) *anyopaque;
extern fn _exit(status: c_int) noreturn;

// Maps the fixed binary input image (single-proof or aggregator-pair; the
// build mode picks `native_input_path`) at the guest base address, returning
// null on any failure so each caller can exit through its own path.
fn mapNativeImage() ?[*]const u8 {
    if (comptime !is_supported_native) {
        @compileError("native verifier libc path currently supports x86_64/aarch64 Linux and macOS only");
    }

    const fd = open(native_input_path.ptr, o_rdonly);
    if (fd < 0) return null;
    defer _ = close(fd);

    const image_len = lseek(fd, 0, seek_end);
    if (image_len <= 0) return null;

    const mapped_addr = mmap(
        @ptrFromInt(input_guest_base),
        @intCast(image_len),
        prot_read,
        map_private | map_fixed,
        fd,
        0,
    );
    if (@intFromPtr(mapped_addr) == map_failed) return null;

    return @ptrCast(mapped_addr);
}

fn loadNativeInput() *const verifier.VerifyInput {
    if (comptime embedded_data_conf.embed_input) {
        return &embedded_input;
    }

    const image = mapNativeImage() orelse exitNative(1);
    return @ptrCast(@alignCast(image));
}

fn loadR5Input() *const verifier.VerifyInput {
    if (comptime !is_r5_zkvm) {
        @compileError("R5 verifier path currently supports only R5 zkVM target");
    }
    if (comptime embedded_data_conf.embed_input) {
        return &embedded_input;
    }

    // The zkc JSON input writer places the proof image bytes directly at
    // `_in_start`, already relocated for GuestBase.
    return @ptrCast(@alignCast(&_in_start));
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
    // Delegate to the Lineth accelerator package's standard zkVM exit (zkvm_std.h).
    lineth_accel.zkvm_exit(@intCast(code));
}

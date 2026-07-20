const builtin = @import("builtin");
const verifier_ray = @import("verifier_ray");
const embedded_data = @import("embedded_data");
const embedded_data_conf = @import("embedded_data_config");
const lineth_accel = @import("lineth_accelerators");

const verifier = verifier_ray.verifier;

const is_r5_zkvm = verifier_ray.r5_config.is_r5_zkvm;
const is_native_os = builtin.target.os.tag == .linux or builtin.target.os.tag == .macos;
const is_native_arch = builtin.target.cpu.arch == .x86_64 or builtin.target.cpu.arch == .aarch64;
const is_supported_native = is_native_os and is_native_arch;

const native_input_path: [:0]const u8 = "zig-out/input.bin";

extern const _in_start: u8;

// When the input is embedded at build time, the fixture proof is materialized
// into static (.rodata) memory here so the loaders can hand out a runtime
// pointer to it, exactly like the mmap/linker paths do. This keeps `Proof` as
// a plain runtime value — only `spec`/`systems` are comptime in `verify`. The
// const is lazily analyzed, so it costs nothing when `embed_input` is false.
const embedded_input: verifier.Proof = if (embedded_data_conf.invalid_input)
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

fn runVerifier(input: *const verifier.Proof) u8 {
    const verifier_case = comptime embedded_data.get(embedded_data_conf.spec_index);
    const spec = verifier_case.spec;
    const systems = verifier_case.systems;
    // `spec`/`systems` are comptime, but the proof is a runtime value read from
    // `input` (mmap/linker/embedded memory), so dereference it here.
    verifier.verify(spec, systems, input.*) catch {
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
const map_failed = ~@as(usize, 0);

extern fn open(path: [*:0]const u8, flags: c_int) c_int;
extern fn mmap(address: ?*anyopaque, length: usize, protection: c_int, flags: c_int, fd: c_int, offset: i64) *anyopaque;
extern fn _exit(status: c_int) noreturn;

fn loadNativeInput() *const verifier.Proof {
    if (comptime !is_supported_native) {
        @compileError("native verifier libc path currently supports x86_64/aarch64 Linux and macOS only");
    }
    if (comptime embedded_data_conf.embed_input) {
        return &embedded_input;
    }
    // TODO: we have kept the compatibility with the old way of loading input, but we don't have serialization
    // so it will fail if the input is not embedded.

    const fd = open(native_input_path.ptr, o_rdonly);
    if (fd < 0) exitNative(1);

    const mapped_addr = mmap(null, @sizeOf(verifier.Proof), prot_read, map_private, fd, 0);
    if (@intFromPtr(mapped_addr) == map_failed) exitNative(1);

    const mapped_bytes: [*]const u8 = @ptrCast(mapped_addr);
    return @ptrCast(@alignCast(mapped_bytes));
}

fn loadR5Input() *const verifier.Proof {
    if (comptime !is_r5_zkvm) {
        @compileError("R5 verifier path currently supports only R5 zkVM target");
    }
    if (comptime embedded_data_conf.embed_input) {
        return &embedded_input;
    }
    // TODO: we have kept the compatibility with the old way of loading input, but we don't have serialization
    // so it will fail if the input is not embedded.

    // the input is linked into the binary at compile time using the
    // `_in_start` symbol defined in the linker script, so we can just take its
    // address and cast it to our structured input type
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
    // Delegate to the Linea accelerator package's standard zkVM exit (zkvm_std.h).
    lineth_accel.zkvm_exit(@intCast(code));
}

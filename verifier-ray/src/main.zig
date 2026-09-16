const std = @import("std");
const builtin = @import("builtin");
const verifier_ray = @import("verifier_ray");
const embedded_data = @import("embedded_data");
const embedded_data_conf = @import("embedded_data_config");
const riscv_system = @import("riscv_system");
const lineth_accel = @import("lineth_accelerators");

const verifier = verifier_ray.verifier;
const image_relocation = verifier_ray.image_relocation;
const system_runtime = verifier_ray.system_runtime;
const profiling = verifier_ray.profiling;

const is_r5_zkvm = verifier_ray.r5_config.is_r5_zkvm;
const is_native_os = builtin.target.os.tag == .linux or builtin.target.os.tag == .macos;
const is_native_arch = builtin.target.cpu.arch == .x86_64 or builtin.target.cpu.arch == .aarch64;
const is_supported_native = is_native_os and is_native_arch;

const native_input_path: [:0]const u8 = "testdata/riscv_proof_image.bin";
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

// The compact system is decoded once per process into zero-initialized guest
// memory. Its generated capacity includes headroom over the measured decoded
// graph and contributes to .bss rather than the ELF file's .rodata.
var decoded_system_storage: [riscv_system.system_0_decoded_arena_size]u8 = undefined;
var verifier_workspace: verifier.RuntimeWorkspace(riscv_system.system_0_limits) = undefined;

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

fn runVerifier(input: *const verifier.VerifyInput) u8 {
    if (comptime embedded_data_conf.embed_input) {
        const case = comptime embedded_data.get(embedded_data_conf.spec_index);
        verifier.verify(case.spec, case.systems, input.proof, input.public_inputs) catch return 1;
        return 0;
    }

    if (comptime profiling.r5_marks)
        profiling.markR5Value(profiling.Mark.system_decode_start, riscv_system.system_0_encoded.len);
    var fba = std.heap.FixedBufferAllocator.init(&decoded_system_storage);
    const bundle = system_runtime.decodeBundle(&riscv_system.system_0_encoded, fba.allocator()) catch return 1;
    if (comptime profiling.r5_marks)
        profiling.markR5Value(profiling.Mark.system_decode_done, fba.end_index);
    if (comptime embedded_data_conf.decode_only) return 0;

    verifier.verifyRuntimeWithWorkspace(
        riscv_system.system_0_limits,
        bundle.spec,
        bundle.systems,
        input.proof,
        input.public_inputs,
        &verifier_workspace,
    ) catch {
        // if the verifier fails, return a non-zero exit code
        return 1;
    };
    return 0; // success
}

// Native smoke tests use the same binary input image as the R5 linked-memory path.
// The file is mapped privately at an address chosen by the OS, then its absolute
// guest pointers are rebased in the copy-on-write mapping. Avoiding std
// file/argument handling keeps ReleaseSmall native binaries compact.
const o_rdonly: c_int = 0;
const prot_read: c_int = 1;
const prot_write: c_int = 2;
const map_private: c_int = 2;
const seek_end: c_int = 2;
const map_failed = ~@as(usize, 0);

extern fn open(path: [*:0]const u8, flags: c_int) c_int;
extern fn close(fd: c_int) c_int;
extern fn lseek(fd: c_int, offset: i64, whence: c_int) i64;
extern fn mmap(address: ?*anyopaque, length: usize, protection: c_int, flags: c_int, fd: c_int, offset: i64) *anyopaque;
extern fn _exit(status: c_int) noreturn;

fn loadNativeInput() *const verifier.VerifyInput {
    if (comptime !is_supported_native) {
        @compileError("native verifier libc path currently supports x86_64/aarch64 Linux and macOS only");
    }
    if (comptime embedded_data_conf.embed_input) {
        return &embedded_input;
    }

    const fd = open(native_input_path.ptr, o_rdonly);
    if (fd < 0) exitNative(1);
    defer _ = close(fd);

    const image_len = lseek(fd, 0, seek_end);
    if (image_len <= 0) exitNative(1);
    const img_len: usize = @intCast(image_len);

    // MAP_PRIVATE makes pointer rewrites copy-on-write: the mapped bytes change,
    // but the stored proof image does not. A read-only file descriptor is enough
    // because no write is ever propagated back to the file.
    const buf_addr = mmap(null, img_len, prot_read | prot_write, map_private, fd, 0);
    if (@intFromPtr(buf_addr) == map_failed) exitNative(1);

    const buf: [*]u8 = @ptrCast(buf_addr);
    image_relocation.rebase(buf, img_len, input_guest_base, @intFromPtr(buf_addr));
    return @ptrCast(@alignCast(buf_addr));
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

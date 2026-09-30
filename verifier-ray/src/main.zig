const std = @import("std");
const builtin = @import("builtin");
const verifier_ray = @import("verifier_ray");
const embedded_data = @import("embedded_data");
const embedded_data_conf = @import("embedded_data_config");
const riscv_system = @import("riscv_system");
const lineth_accel = @import("lineth_accelerators");

const verifier = verifier_ray.verifier;
const proof_guest = verifier_ray.proof_guest;

const is_r5_zkvm = verifier_ray.r5_config.is_r5_zkvm;
const is_native_os = builtin.target.os.tag == .linux or builtin.target.os.tag == .macos;
const is_native_arch = builtin.target.cpu.arch == .x86_64 or builtin.target.cpu.arch == .aarch64;
const is_supported_native = is_native_os and is_native_arch;

const native_input_path: [:0]const u8 = "testdata/riscv_proof_image.bin";

extern const _in_start: u8;
extern const _in_end: u8;

// Expanded proof. The packed image is smaller; decode materializes the zero
// limbs and the slice headers the Merkle hasher reads. Sized for the RISC-V
// opening, which lands well under this.
var guest_arena_buf: [proof_guest.decode_buffer_len]u8 align(16) = undefined;
var decoded_guest_input: verifier.VerifyInput = undefined;

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
// The bound-round-message workspace lives here, in .bss, rather than in
// `verify`'s stack frame: it holds every round cell (17,842 on the real RISC-V
// system) and the guest's linker stack is a fixed 8 MiB.
var verifier_workspace: verifier.Workspace(riscv_system.system_0_public_input) = undefined;

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
    verifier.verifyWithWorkspace(spec, systems, input.proof, input.public_inputs, &verifier_workspace) catch {
        // if the verifier fails, return a non-zero exit code
        return 1;
    };
    return 0; // success
}

// Native smoke tests read the same guest image the R5 path receives at
// `_in_start`. The image is pointer-free; decode expands it into
// `guest_arena_buf`. Avoiding std file/argument handling keeps ReleaseSmall
// native binaries compact.
const o_rdonly: c_int = 0;
const prot_read: c_int = 1;
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

    const buf_addr = mmap(null, img_len, prot_read, map_private, fd, 0);
    if (@intFromPtr(buf_addr) == map_failed) exitNative(1);

    const buf: [*]const u8 = @ptrCast(buf_addr);
    return decodeGuestImage(buf[0..img_len]);
}

fn loadR5Input() *const verifier.VerifyInput {
    if (comptime !is_r5_zkvm) {
        @compileError("R5 verifier path currently supports only R5 zkVM target");
    }
    if (comptime embedded_data_conf.embed_input) {
        return &embedded_input;
    }

    // The zkc JSON input writer places the guest image at `_in_start`. The
    // region runs to `_in_end`; the image's own length prefix says where the
    // proof stops.
    const start = @intFromPtr(&_in_start);
    const end = @intFromPtr(&_in_end);
    if (end < start) exitR5(1);
    const bytes: [*]const u8 = @ptrCast(&_in_start);
    return decodeGuestImage(bytes[0 .. end - start]);
}

fn decodeGuestImage(bytes: []const u8) *const verifier.VerifyInput {
    var fba = std.heap.FixedBufferAllocator.init(&guest_arena_buf);
    decoded_guest_input = proof_guest.decode(fba.allocator(), bytes) catch {
        if (comptime is_r5_zkvm) exitR5(1) else exitNative(1);
    };
    return &decoded_guest_input;
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

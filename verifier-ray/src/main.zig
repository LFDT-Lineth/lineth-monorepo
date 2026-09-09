const builtin = @import("builtin");
const verifier_ray = @import("verifier_ray");
const embedded_data = @import("embedded_data");
const embedded_data_conf = @import("embedded_data_config");
const riscv_system = @import("riscv_system");
const lineth_accel = @import("lineth_accelerators");
const main_config = @import("main_config");
const image_relocation = @import("image_relocation");

const verifier = verifier_ray.verifier;

const is_r5_zkvm = verifier_ray.r5_config.is_r5_zkvm;
const is_aggregator = main_config.aggregator;
const is_native_os = builtin.target.os.tag == .linux or builtin.target.os.tag == .macos;
const is_native_arch = builtin.target.cpu.arch == .x86_64 or builtin.target.cpu.arch == .aarch64;
const is_supported_native = is_native_os and is_native_arch;

const native_input_path: [:0]const u8 = "testdata/riscv_proof_image.bin";
const native_pair_input_path: [:0]const u8 = "testdata/riscv_proof_pair_image.bin";
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

    if (comptime is_aggregator) {
        const pair = loadNativePairInput();
        exitNative(runVerifierPair(pair));
    } else {
        const input = loadNativeInput();
        exitNative(runVerifier(input));
    }
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

    if (comptime is_aggregator) {
        // the aggregator pair image is linked at `_in_start` by the zkc JSON input writer
        const pair = loadR5PairInput();
        exitR5(runVerifierPair(pair));
    } else {
        // load the input depending on the running mode (embedded by the zkVM or at compile time)
        const input = loadR5Input();

        // run the verifier smoke test with the loaded input
        const res = runVerifier(input);
        exitR5(res);
    }
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
    verifier.verify(spec, systems, input.proof, input.public_inputs) catch {
        // if the verifier fails, return a non-zero exit code
        return 1;
    };
    return 0; // success
}

fn runVerifierPair(pair: verifier.AggregatorInput) u8 {
    // the aggregator entry point never embeds its input at build time; it always
    // verifies against the real compiled riscv system.
    verifier.verifyPair(riscv_system.system_0_spec, riscv_system.system_0_systems, pair.a.*, pair.b.*) catch {
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
const o_rdwr: c_int = 2;
const prot_read: c_int = 1;
const prot_write: c_int = 2;
const map_private: c_int = 2;
// MAP_FIXED: map at exactly the requested address. Safe here because
// loadNativeInput runs in a standalone process that does not share its address
// space with anything else mapped at input_guest_base.
const map_fixed: c_int = 0x10;
// MAP_ANONYMOUS: allocate anonymous (not file-backed) memory.
// Linux uses 0x20; macOS uses 0x1000.
const map_anon: c_int = if (builtin.target.os.tag == .macos) 0x1000 else 0x20;
const seek_set: c_int = 0;
const seek_end: c_int = 2;
const map_failed = ~@as(usize, 0);

extern fn open(path: [*:0]const u8, flags: c_int) c_int;
extern fn close(fd: c_int) c_int;
extern fn lseek(fd: c_int, offset: i64, whence: c_int) i64;
extern fn read(fd: c_int, buf: [*]u8, count: usize) isize;
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

    // Try to map at GuestBase so the baked-in absolute pointers are valid with no
    // fixup. On Linux this always works. On macOS, MAP_FIXED at low addresses is
    // refused by the kernel; in that case fall back to an anonymous mapping at any
    // address and rebase the embedded pointers.
    const try_fixed = mmap(
        @ptrFromInt(input_guest_base),
        @intCast(image_len),
        prot_read,
        map_private | map_fixed,
        fd,
        0,
    );
    if (@intFromPtr(try_fixed) != map_failed) {
        return @ptrCast(@alignCast(try_fixed));
    }

    // MAP_FIXED failed (macOS). Allocate a writable anonymous buffer, read the
    // file into it, then patch every slice pointer from its guest address to the
    // equivalent host address.
    const buf_addr = mmap(null, @intCast(image_len), prot_read | prot_write, map_private | map_anon, -1, 0);
    if (@intFromPtr(buf_addr) == map_failed) exitNative(1);

    // Seek back to start and read the full image.
    if (lseek(fd, 0, seek_set) < 0) exitNative(1);
    const buf: [*]u8 = @ptrCast(buf_addr);
    var remaining: usize = @intCast(image_len);
    var off: usize = 0;
    while (remaining > 0) {
        const n = read(fd, buf + off, remaining);
        if (n <= 0) exitNative(1);
        off += @intCast(n);
        remaining -= @intCast(n);
    }

    rebaseImagePointers(buf, @intCast(image_len), input_guest_base, @intFromPtr(buf_addr));
    return @ptrCast(@alignCast(buf_addr));
}

// The aggregator pair image is the two-pointer `AggregatorInput` header (see
// `verifier.AggregatorInput` / `proof_abi.zig`) followed by its two relocated
// `VerifyInput` sub-images, all encoded absolute for `input_guest_base`. It is
// loaded the same way as the single-proof image (fixed mmap on Linux, patched
// anonymous mapping on macOS); the sub-image pointers inside the header are
// rebased alongside the two `VerifyInput`s they point to.
fn loadNativePairInput() verifier.AggregatorInput {
    if (comptime !is_supported_native) {
        @compileError("native verifier libc path currently supports x86_64/aarch64 Linux and macOS only");
    }

    const fd = open(native_pair_input_path.ptr, o_rdonly);
    if (fd < 0) exitNative(1);
    defer _ = close(fd);

    const image_len = lseek(fd, 0, seek_end);
    if (image_len <= 0) exitNative(1);

    const try_fixed = mmap(
        @ptrFromInt(input_guest_base),
        @intCast(image_len),
        prot_read,
        map_private | map_fixed,
        fd,
        0,
    );
    if (@intFromPtr(try_fixed) != map_failed) {
        return readAggregatorInput(@ptrCast(try_fixed));
    }

    // MAP_FIXED failed (macOS). Allocate a writable anonymous buffer, read the
    // file into it, then patch every slice/sub-image pointer from its guest
    // address to the equivalent host address.
    const buf_addr = mmap(null, @intCast(image_len), prot_read | prot_write, map_private | map_anon, -1, 0);
    if (@intFromPtr(buf_addr) == map_failed) exitNative(1);

    if (lseek(fd, 0, seek_set) < 0) exitNative(1);
    const buf: [*]u8 = @ptrCast(buf_addr);
    var remaining: usize = @intCast(image_len);
    var off: usize = 0;
    while (remaining > 0) {
        const n = read(fd, buf + off, remaining);
        if (n <= 0) exitNative(1);
        off += @intCast(n);
        remaining -= @intCast(n);
    }

    const len: usize = @intCast(image_len);
    // The pair header itself is two absolute pointers (offsets 0 and 8); rebase
    // those, then rebase each sub-image they now point into.
    const mapped_base = @intFromPtr(buf_addr);
    const a_off = patchHeaderPtr(buf, len, 0, input_guest_base, mapped_base);
    const b_off = patchHeaderPtr(buf, len, 8, input_guest_base, mapped_base);
    rebaseImagePointers(buf + a_off, len - a_off, input_guest_base + a_off, mapped_base + a_off);
    rebaseImagePointers(buf + b_off, len - b_off, input_guest_base + b_off, mapped_base + b_off);

    return readAggregatorInput(@ptrCast(buf_addr));
}

// Reinterprets a mapped/patched pair image as `AggregatorInput`. The header's
// two fields are themselves absolute pointers written by the encoder
// (`EncodeAggregatorPair`), so no further arithmetic is needed once the
// mapping is rebased to the address the image was encoded for.
fn readAggregatorInput(base: *anyopaque) verifier.AggregatorInput {
    const header: *const AggregatorHeader = @ptrCast(@alignCast(base));
    return .{
        .a = @ptrFromInt(header.a),
        .b = @ptrFromInt(header.b),
    };
}

// Byte-for-byte layout of the aggregator pair image header: two absolute
// little-endian u64 pointers, matching `EncodeAggregatorPair`'s header write
// and `verifier.AggregatorInput`.
const AggregatorHeader = extern struct {
    a: u64,
    b: u64,
};

fn patchHeaderPtr(
    img: [*]u8,
    len: usize,
    off: usize,
    encoded_base: usize,
    mapped_base: usize,
) usize {
    if (off + 8 > len) exitNative(1);
    const old_ptr = readU64(img, off);
    const image_offset = image_relocation.imageOffset(old_ptr, encoded_base) catch exitNative(1);
    const new_ptr = image_relocation.relocatePointer(old_ptr, encoded_base, mapped_base) catch exitNative(1);
    writeU64(img, off, @intCast(new_ptr));
    return image_offset;
}

fn readU64(img: [*]const u8, off: usize) u64 {
    return @as(u64, img[off]) |
        (@as(u64, img[off + 1]) << 8) |
        (@as(u64, img[off + 2]) << 16) |
        (@as(u64, img[off + 3]) << 24) |
        (@as(u64, img[off + 4]) << 32) |
        (@as(u64, img[off + 5]) << 40) |
        (@as(u64, img[off + 6]) << 48) |
        (@as(u64, img[off + 7]) << 56);
}

fn writeU64(img: [*]u8, off: usize, v: u64) void {
    img[off + 0] = @truncate(v);
    img[off + 1] = @truncate(v >> 8);
    img[off + 2] = @truncate(v >> 16);
    img[off + 3] = @truncate(v >> 24);
    img[off + 4] = @truncate(v >> 32);
    img[off + 5] = @truncate(v >> 40);
    img[off + 6] = @truncate(v >> 48);
    img[off + 7] = @truncate(v >> 56);
}

// rebaseImagePointers patches every slice-pointer in the image from its
// encoded address (encoded_addr + offset) to the equivalent host address
// (mapped_addr + offset). The image layout is fully determined by the
// proofserialization layout constants mirrored in verifier-ray/src/proof_abi.zig;
// every []const T header is a {ptr: u64le, len: u64le} pair and only the ptr
// field needs adjustment. We walk the structure typed, never scanning raw bytes,
// so non-pointer u64s (lengths, field values) are never touched.
fn rebaseImagePointers(
    img: [*]u8,
    img_len: usize,
    encoded_addr: usize,
    mapped_addr: usize,
) void {
    // Patch a single slice-pointer at byte offset `off` in the image.
    const patchPtr = struct {
        fn f(
            image: [*]u8,
            len: usize,
            off: usize,
            encoded_base: usize,
            mapped_base: usize,
        ) usize {
            if (off + 16 > len) return 0; // bounds check
            const old_ptr = readU64(image, off);
            if (old_ptr == 0) return 0; // malformed null slice
            const payload_offset = image_relocation.imageOffset(old_ptr, encoded_base) catch exitNative(1);
            const new_ptr = image_relocation.relocatePointer(old_ptr, encoded_base, mapped_base) catch exitNative(1);
            writeU64(image, off, @intCast(new_ptr));
            // Return the payload offset (for callers that need to walk into it).
            return payload_offset;
        }
    }.f;

    // Returns the slice count stored at `off + 8`.
    const sliceLen = struct {
        fn f(image: [*]u8, off: usize) usize {
            return @intCast(readU64(image, off + 8));
        }
    }.f;

    // VerifyInput offsets (from proof_abi.zig / layout.go):
    //   proof @ 0 (96 bytes), public_inputs @ 96 (slice of Scalar — no nested ptrs)
    _ = patchPtr(img, img_len, 96, encoded_addr, mapped_addr); // public_inputs.ptr

    // Proof offsets: rounds @ 0, module_sizes @ 16, pcs_opening @ 32
    const rounds_ptr = patchPtr(img, img_len, 0, encoded_addr, mapped_addr);
    const n_rounds = sliceLen(img, 0);
    _ = patchPtr(img, img_len, 16, encoded_addr, mapped_addr); // module_sizes.ptr (scalar elements, no nesting)

    // Each RoundMessage is 56 bytes: cells @ 0, commitment @ 16.
    // cells is []Scalar (no nested ptrs), commitment is inline optional.
    for (0..n_rounds) |i| {
        const rm_off = rounds_ptr + i * 56;
        _ = patchPtr(img, img_len, rm_off + 0, encoded_addr, mapped_addr); // cells.ptr
    }

    // PcsOpening @ 32: one field `proof` (OpeningProof) @ 32+0=32.
    // OpeningProof: input_queries @ 32, fri_proof @ 48.

    // input_queries: [][]InputTreeOpening
    const iq_outer_ptr = patchPtr(img, img_len, 32 + 0, encoded_addr, mapped_addr);
    const n_iq = sliceLen(img, 32 + 0);
    for (0..n_iq) |i| {
        const inner_hdr = iq_outer_ptr + i * 16;
        const iq_inner_ptr = patchPtr(img, img_len, inner_hdr, encoded_addr, mapped_addr);
        const n_ito = sliceLen(img, inner_hdr);
        // InputTreeOpening: siblings @ 0, leaves @ 16
        for (0..n_ito) |j| {
            const ito_off = iq_inner_ptr + j * 32;
            _ = patchPtr(img, img_len, ito_off + 0, encoded_addr, mapped_addr); // siblings.ptr
            // leaves: []*?RowPair — no nested ptrs (?RowPair is inline)
            _ = patchPtr(img, img_len, ito_off + 16, encoded_addr, mapped_addr); // leaves.ptr
        }
    }

    // FriProof @ 48: round_roots @ 48, final_poly @ 64, running_queries @ 80.
    _ = patchPtr(img, img_len, 48 + 0, encoded_addr, mapped_addr); // round_roots.ptr (scalar)
    _ = patchPtr(img, img_len, 48 + 16, encoded_addr, mapped_addr); // final_poly.ptr (scalar)

    // running_queries: [][]Branch
    const rq_outer_ptr = patchPtr(img, img_len, 48 + 32, encoded_addr, mapped_addr);
    const n_rq = sliceLen(img, 48 + 32);
    for (0..n_rq) |i| {
        const inner_hdr = rq_outer_ptr + i * 16;
        const rq_inner_ptr = patchPtr(img, img_len, inner_hdr, encoded_addr, mapped_addr);
        const n_br = sliceLen(img, inner_hdr);
        // Branch: siblings @ 0, leaf @ 16 (inline Digest — no ptr)
        for (0..n_br) |j| {
            const br_off = rq_inner_ptr + j * 48;
            _ = patchPtr(img, img_len, br_off + 0, encoded_addr, mapped_addr); // siblings.ptr
        }
    }
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

fn loadR5PairInput() verifier.AggregatorInput {
    if (comptime !is_r5_zkvm) {
        @compileError("R5 verifier path currently supports only R5 zkVM target");
    }

    // The zkc JSON input writer places the pair image bytes directly at
    // `_in_start`, already relocated for GuestBase: a two-pointer
    // `AggregatorInput` header followed by its two `VerifyInput` sub-images.
    return readAggregatorInput(@ptrCast(@constCast(&_in_start)));
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

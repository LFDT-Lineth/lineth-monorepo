//! Lineth zkVM accelerator wrappers, packaged as a reusable Zig module.
//!
//! Exposes the `lineth_accelerators` module: thin wrappers that issue the custom RISC-V
//! instructions the Lineth prover accelerates plus the standard runtime helpers (zkvm_exit).
//! The matching C interface headers live under include/.
//!
//! Consumers add a path dependency in their build.zig.zon and import the module, passing the
//! target/optimize they build for (every guest builds the freestanding rv64im ZkC profile), e.g.
//!   const dep = b.dependency("lineth_accelerators", .{ .target = target, .optimize = optimize });
//!   some_module.addImport("lineth_zkvm_accel", dep.module("lineth_accelerators"));
//!
//! `-Dzkvm-provide=true` additionally exposes the crypto surface that zesu's extern backend needs:
//! `zkvm_provide` (defines every `zkvm_*` symbol, linking the Constantine archive for the target)
//! and `guest_crypto` (the Zig bindings to that archive). `-Dkeccak-accel=true` makes
//! `zkvm_provide` export the prover-accelerated keccak instead of the standard Zig one, and
//! `-Dsecp256k1-backend` selects its secp256k1 provider: `secp256k1_std` (default, also exposed as a
//! module) or Constantine. zesu and
//! Constantine are lazy dependencies behind `-Dzkvm-provide`, fetched and built only for consumers
//! that pass it. A consumer that opts in takes `lineth_accelerators` itself from the same
//! dependency instance, e.g.
//!   const dep = b.dependency("lineth_accelerators", .{ .target = target, .optimize = optimize, .@"zkvm-provide" = true });

const std = @import("std");
const common = @import("build_common");

const Secp256k1Backend = enum { std, constantine };

pub fn build(b: *std.Build) void {
    common.requireZigVersion();

    // Same freestanding rv64im ZkC profile as every guest (shared helper); a consumer that passes
    // its own target via b.dependency overrides this default.
    const target = common.standardGuestTarget(b);
    // Use b.option directly (not standardOptimizeOption) so the `-Doptimize` enum option stays
    // exposed — consumers set it through `b.dependency(..., .{ .optimize = ... })` — while still
    // defaulting to ReleaseSmall. (standardOptimizeOption with preferred_optimize_mode would swap
    // `-Doptimize` for `-Drelease`, breaking the dependency pass-through.)
    const optimize = b.option(std.builtin.OptimizeMode, "optimize", "Optimization mode (default: ReleaseSmall)") orelse .ReleaseSmall;
    const zkvm_provide = b.option(bool, "zkvm-provide", "Expose the zkvm_provide and guest_crypto modules (fetches zesu, builds Constantine)") orelse false;
    const keccak_accel = b.option(bool, "keccak-accel", "zkvm_provide exports the arithmetization keccak wrapper instead of standard zig keccak (default: unaccelerated)") orelse false;
    const secp256k1_backend = b.option(Secp256k1Backend, "secp256k1-backend", "zkvm_provide's secp256k1 ecrecover/verify provider: std (default) or constantine") orelse .std;

    const accelerators = b.addModule("lineth_accelerators", .{
        .root_source_file = b.path("src/root.zig"),
        .target = target,
        .optimize = optimize,
    });

    if (zkvm_provide) addCryptoProviders(b, accelerators, target, optimize, keccak_accel, secp256k1_backend);
}

/// Builds `zkvm_provide` and `guest_crypto` for `target`. The freestanding guest target links the
/// rv64 Constantine archive and exports the guest-only `zkvm_*` symbols; every other target links
/// the host archive and libc. The modules exist from the first configure pass on; the lazy
/// dependencies are wired in once fetched, after which the build reruns.
fn addCryptoProviders(
    b: *std.Build,
    accelerators: *std.Build.Module,
    target: std.Build.ResolvedTarget,
    optimize: std.builtin.OptimizeMode,
    keccak_accel: bool,
    secp256k1_backend: Secp256k1Backend,
) void {
    const is_guest = target.result.os.tag == .freestanding;

    const zesu_crypto_backend = b.createModule(.{
        .root_source_file = b.path("src/zesu_crypto_backend.zig"),
        .target = target,
        .optimize = optimize,
    });
    const guest_crypto = b.addModule("guest_crypto", .{
        .root_source_file = b.path("src/guest_crypto.zig"),
        .target = target,
        .optimize = optimize,
    });

    const secp256k1_std = b.addModule("secp256k1_std", .{
        .root_source_file = b.path("src/secp256k1_std.zig"),
        .target = target,
        .optimize = optimize,
    });

    const provide_options = b.addOptions();
    provide_options.addOption(bool, "keccak_accel", keccak_accel);
    provide_options.addOption(bool, "is_guest", is_guest);
    provide_options.addOption(Secp256k1Backend, "secp256k1_backend", secp256k1_backend);

    const provide = b.addModule("zkvm_provide", .{
        .root_source_file = b.path("src/zkvm_provide.zig"),
        .target = target,
        .optimize = optimize,
        .link_libc = if (is_guest) null else true,
    });
    provide.addImport("lineth_zkvm_accel", accelerators);
    provide.addImport("zesu_crypto_backend", zesu_crypto_backend);
    provide.addImport("guest_crypto", guest_crypto);
    provide.addImport("secp256k1_std", secp256k1_std);
    provide.addOptions("build_options", provide_options);

    const zesu = b.lazyDependency("zesu", .{
        .target = target,
        .optimize = optimize,
        .@"crypto-backend" = .@"extern",
    }) orelse return;
    const modexp_impl = b.createModule(.{
        .root_source_file = zesu.path("src/crypto/backends/modexp_impl.zig"),
        .target = target,
        .optimize = optimize,
    });
    modexp_impl.addImport("zesu_allocator", zesu.module("zesu_allocator"));
    const ripemd160_impl = b.createModule(.{
        .root_source_file = zesu.path("src/crypto/backends/ripemd160_impl.zig"),
        .target = target,
        .optimize = optimize,
    });
    const blake2f_impl = b.createModule(.{
        .root_source_file = zesu.path("src/crypto/backends/blake2f_impl.zig"),
        .target = target,
        .optimize = optimize,
    });
    zesu_crypto_backend.addImport("zesu_modexp_impl", modexp_impl);
    zesu_crypto_backend.addImport("zesu_ripemd160_impl", ripemd160_impl);
    zesu_crypto_backend.addImport("zesu_blake2f_impl", blake2f_impl);

    const constantine = b.lazyDependency("guest_crypto_ctt", .{}) orelse return;
    const archive_name = if (is_guest) "riscv_staticlib" else "host_staticlib";
    // The archives exist once Constantine is fetched.
    provide.addObjectFile(constantine.builder.named_lazy_paths.get(archive_name) orelse return);
}

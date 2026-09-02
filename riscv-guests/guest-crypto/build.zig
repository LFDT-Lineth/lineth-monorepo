//! Zig-package face of the guest-crypto Rust staticlib. The crate itself is built by cargo
//! (toolchain pinned in rust-toolchain.toml); this build script drives the two cargo profiles and
//! exposes the resulting archives as named lazy paths, so consumers add them with
//! `dep.namedLazyPath("riscv_staticlib" | "host_staticlib")` and get the cargo step ordered
//! before their own link automatically.

const std = @import("std");

pub fn build(b: *std.Build) void {
    b.addNamedLazyPath("host_staticlib", cargoStaticlib(
        b,
        "guest-crypto host staticlib (cargo)",
        "cargo build --release --quiet && cp target/release/libguest_crypto.a \"$1\"",
    ));
    b.addNamedLazyPath("riscv_staticlib", cargoStaticlib(
        b,
        "guest-crypto rv64im staticlib (cargo)",
        "cargo build --release --quiet -Zbuild-std=core,alloc --target ./riscv64im-zkc.json && " ++
            "cp target/riscv64im-zkc/release/libguest_crypto.a \"$1\"",
    ));
}

fn cargoStaticlib(b: *std.Build, name: []const u8, script: []const u8) std.Build.LazyPath {
    const run = b.addSystemCommand(&.{ "sh", "-c", script, "cargo-staticlib" });
    run.setCwd(b.path("."));
    run.setName(name);
    // Cargo owns incrementality over the Rust sources; always let it decide.
    run.has_side_effects = true;
    return run.addOutputFileArg("libguest_crypto.a");
}

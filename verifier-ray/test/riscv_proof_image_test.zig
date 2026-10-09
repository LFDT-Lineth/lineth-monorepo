//! Reads the committed honest-proof image as a real `verifier.VerifyInput`.
//!
//! The fixture is `testdata/riscv_proof_image.bin`, generated
//! from the same real `arithmetization/src/main/riscv/main.zkc` proof path
//! (proving `zkc_r5.AllInOneGuestELF`, which exercises the full RV64I +
//! M-extension + custom-precompile surface in a single witness) that emits
//! `testdata/generated/riscv_system.zig`. This is the
//! cross-language end-to-end check: Go writes the native layout bytes, Zig
//! mmaps and casts them directly, then the real verifier accepts the proof
//! against the real compiled system.
//!
//! Deliberately a distinct file from `testdata/proof_image.bin`, which is
//! prover-ray's `TestVerifierRayImageIsUpToDate` fixture: a small synthetic
//! `VerifyInput` at a different base address, for a cross-language ABI-
//! agreement check unrelated to this real end-to-end proof. The two must not
//! share a path — each writer would silently clobber the other's fixture with
//! content the other's reader can't decode.

const std = @import("std");
const verifier_ray = @import("verifier_ray");
const riscv_system = @import("riscv_system");

const verifier = verifier_ray.verifier;
const pcs = verifier_ray.query.pcs;
const field = verifier_ray.field.koalabear;

// The base address at which riscv_proof_image.bin was encoded.
// Must match GuestBase in prover-ray/wiop/proofserialization/layout.go.
const encoded_base: usize = 0x08800000;

const image_path = "testdata/riscv_proof_image.bin";

const o_rdonly: c_int = 0;
const prot_read: c_int = 1;
const prot_write: c_int = 2;
const map_private: c_int = 2;
const seek_end: c_int = 2;
const map_failed = ~@as(usize, 0);

extern fn open(path: [*:0]const u8, flags: c_int) c_int;
extern fn mmap(address: ?*anyopaque, length: usize, prot: c_int, flags: c_int, fd: c_int, offset: i64) *anyopaque;
extern fn munmap(address: *anyopaque, length: usize) c_int;
extern fn close(fd: c_int) c_int;
extern fn lseek(fd: c_int, offset: i64, whence: c_int) i64;

fn readU64(buf: [*]const u8, offset: usize) u64 {
    return std.mem.readInt(u64, buf[offset..][0..8], .little);
}

fn loadFixtureImage() !*verifier.VerifyInput {
    const fd = open(image_path, o_rdonly);
    if (fd < 0) return error.ImageMissing;
    defer _ = close(fd);

    const image_len = lseek(fd, 0, seek_end);
    if (image_len <= 0) return error.ImageMissing;
    const img_len: usize = @intCast(image_len);

    // Map the file privately at any address the OS picks. Pointer rewrites are
    // copy-on-write and cannot modify the stored proof image.
    const p = mmap(null, img_len, prot_read | prot_write, map_private, fd, 0);
    if (@intFromPtr(p) == map_failed) return error.MmapFailed;

    const buf: [*]u8 = @ptrCast(p);
    const rounds_header = @offsetOf(verifier.VerifyInput, "proof") +
        @offsetOf(verifier.Proof, "rounds");
    const stored_rounds_pointer = readU64(buf, rounds_header);

    verifier_ray.image_relocation.rebase(buf, img_len, encoded_base, @intFromPtr(p));

    const rounds_offset = stored_rounds_pointer - encoded_base;
    try std.testing.expectEqual(
        @as(u64, @intCast(@intFromPtr(p))) + rounds_offset,
        readU64(buf, rounds_header),
    );

    // A fresh mapping must still see the original pointer bytes, proving the
    // relocation dirtied only the private mapping rather than the file.
    const stored = mmap(null, img_len, prot_read, map_private, fd, 0);
    if (@intFromPtr(stored) == map_failed) return error.MmapFailed;
    defer _ = munmap(stored, img_len);
    try std.testing.expectEqual(stored_rounds_pointer, readU64(@ptrCast(stored), rounds_header));

    return @ptrCast(@alignCast(p));
}

test "a Go-encoded honest proof image verifies against the real riscv system" {
    const input = loadFixtureImage() catch |err| switch (err) {
        error.ImageMissing => return error.SkipZigTest,
        else => return err,
    };

    try std.testing.expect(input.proof.rounds.len > 0);
    try std.testing.expect(input.proof.pcs_opening.proof.input_queries.len > 0);

    try verifier.verify(
        riscv_system.system_0_spec,
        riscv_system.system_0_systems,
        input.proof,
        input.public_inputs,
    );
}

// ── Failing fixtures: tampering the real proof's column manifest ─────────────
//
// The manifest rejection paths (InvalidAliasTarget, BatchFullyElided,
// ElidedZeroClaimNonZero, AliasClaimMismatch, AliasShiftNotInTarget) are
// exercised as unit tests on the hand-built `manifest_system` in
// pcs_test.zig. Those drive `reconstructWithManifest` / `buildEntryClaims`
// against a stubbed cell source, so they never touch the real RISC-V system's
// transcript replay, the codegen-emitted `BatchManifest.cell_start/.col_start`
// offsets, or a real proof. The honest fixture above covers the accepting path
// with real elision; the two tests below close the failing side: each loads a
// FRESH private mapping of the honest image (fixture bytes on disk stay
// pristine), tampers one transcript cell of the real proof, and requires
// verification to fail.

test "riscv image: a Present->Zero manifest flip is rejected on the real proof" {
    const input = loadFixtureImage() catch |err| switch (err) {
        error.ImageMissing => return error.SkipZigTest,
        else => return err,
    };
    const cells = input.proof.rounds[0].cells;

    // Find the first column the honest proof actually committed (manifest
    // Present) in batch 0 — its manifest cells are round 0's cells
    // [cell_start, cell_start + <batch-0 column count>) per the generated
    // `batch_manifests` table — and flip its code to Zero.
    const bm = riscv_system.pcs_system_0.batch_manifests[0].?;
    var batch0_cols: usize = 0;
    for (riscv_system.pcs_system_0.columns) |col| {
        if (col.batch_idx == 0) batch0_cols += 1;
    }
    var flipped = false;
    for (0..batch0_cols) |i| {
        const cell: *verifier_ray.protocol.Scalar = @constCast(&cells[bm.cell_start + i]);
        if (cell.* == .base and cell.base.value == pcs.manifest_present) {
            cell.* = .{ .base = field.Element.init(pcs.manifest_zero) };
            flipped = true;
            break;
        }
    }
    // The honest fixture must have at least one Present batch-0 column, or
    // this test would silently check nothing.
    try std.testing.expect(flipped);

    // The verifier now believes a committed column was elided: the
    // reconstructed canonical layout disagrees with the FRI commitment, so the
    // proof must be rejected (the exact stage — layout vs. FRI — is an
    // implementation detail, so this asserts rejection, not a targeted error).
    if (verifier.verify(
        riscv_system.system_0_spec,
        riscv_system.system_0_systems,
        input.proof,
        input.public_inputs,
    )) |_| {
        return error.TamperedManifestAccepted;
    } else |err| {
        std.debug.print("Present->Zero flip rejected with {s}\n", .{@errorName(err)});
    }
}

test "riscv image: a zero-elided column's claim cell is pinned (ElidedZeroClaimNonZero)" {
    const input = loadFixtureImage() catch |err| switch (err) {
        error.ImageMissing => return error.SkipZigTest,
        else => return err,
    };

    // The proof's `rounds` omit the public-input cells, while the compiled
    // manifest/claim `CellRef`s index the BOUND rounds (public inputs merged
    // back). Reproduce that binding exactly as `verifyWithWorkspace` does so
    // the coordinates line up.
    const view: *const verifier.VerifyInput = input;
    const public_input = verifier_ray.protocol.public_input;
    var workspace: verifier.Workspace(riscv_system.system_0_systems.public_input) = .{};
    try public_input.bindRoundMessagesInto(
        riscv_system.system_0_systems.public_input,
        &workspace.bound_rounds,
        view.proof.rounds,
        view.public_inputs,
    );
    const bound_rounds = workspace.bound_rounds.rounds();

    // Read the honest manifest the same way the verifier does, to locate the
    // first zero-elided column's claim cell — the same transcript cell
    // `buildEntryClaims` pins.
    var manifest: pcs.Manifest(riscv_system.pcs_system_0) = .{};
    try pcs.readManifest(riscv_system.pcs_system_0, ManifestProbeCtx{ .rounds = bound_rounds }, &manifest);

    const system = riscv_system.pcs_system_0;
    var target: ?pcs.CellRef = null;
    for (system.columns, 0..) |col, c| {
        if (manifest.codes[c] != pcs.manifest_zero) continue;
        target = system.all_claim_cells[col.claim_start];
        break;
    }
    const ref = target orelse return error.TestExpectedZeroElidedColumn;

    // The claim cell is transcript-bound, so tampering it in the mapped image
    // would shift every downstream coin and trip the vanishing stage BEFORE
    // the PCS pinning runs. `ElidedZeroClaimNonZero` is the guard against a
    // claim cell that reaches the PCS layer dishonestly — i.e. one that is
    // NOT the value Fiat-Shamir absorbed. Build exactly that: clone the one
    // round carrying the cell, leaving the transcript-replayed original
    // (rounds passed to `verify` below are the honest image's) untouched. This
    // is the aliasing gap pcs_test.zig's "equality binding" comment documents:
    // the deduped/skipped claim slots must be equality-bound to the
    // authenticated one, or a prover smuggles a different value through them.
    // Clone the one bound round carrying the cell and tamper the clone,
    // leaving the transcript-replayed honest value untouched: this models a
    // claim cell that reaches the PCS layer dishonestly (the precondition
    // under which the pinning, not a Fiat-Shamir coin shift, is what fires).
    const round_cells = bound_rounds[ref.round].cells;
    const cloned = try std.heap.page_allocator.dupe(verifier_ray.protocol.Scalar, round_cells);
    defer std.heap.page_allocator.free(cloned);
    // Force the zero-elided column's claim cell to a non-zero value.
    cloned[ref.index] = .{ .base = field.Element.init(1) };

    // Verify the pinning directly against the reconstructed layout and the
    // tampered claim cell, with the honest manifest: `buildEntryClaims` must
    // reject the non-zero claim of a Zero-elided column.
    const recon = try pcs.reconstructWithManifest(system, view.proof.module_sizes, manifest.slice());
    var claims: pcs.EntryClaims(system) = .{};
    const tampered_ctx = CloneProbeCtx{ .rounds = bound_rounds, .override_round = ref.round, .override_cells = cloned };
    try std.testing.expectError(
        error.ElidedZeroClaimNonZero,
        pcs.buildEntryClaims(system, &recon, tampered_ctx, &claims),
    );
}

/// A minimal `cell`-shaped probe used to replay the manifest out of the proof
/// rounds exactly as `verifier.verify` does, without running the transcript.
const ManifestProbeCtx = struct {
    rounds: []const verifier_ray.protocol.RoundMessage,

    pub fn cell(self: @This(), round: usize, index: usize) error{CellRefOutOfRange}!verifier_ray.protocol.Scalar {
        if (round >= self.rounds.len) return error.CellRefOutOfRange;
        const cells = self.rounds[round].cells;
        if (index >= cells.len) return error.CellRefOutOfRange;
        return cells[index];
    }
};

/// A `cell`-shaped probe serving the honest proof's rounds, except for ONE
/// round whose cells are replaced by a caller-tampered clone. It models a
/// claim cell that reaches the PCS layer dishonestly while the
/// transcript-replayed bytes stay honest — the precondition under which
/// `buildEntryClaims`'s pinning (not a Fiat-Shamir coin shift) is what fires.
const CloneProbeCtx = struct {
    rounds: []const verifier_ray.protocol.RoundMessage,
    override_round: usize,
    override_cells: []const verifier_ray.protocol.Scalar,

    pub fn cell(self: @This(), round: usize, index: usize) error{CellRefOutOfRange}!verifier_ray.protocol.Scalar {
        if (round >= self.rounds.len) return error.CellRefOutOfRange;
        const cells = if (round == self.override_round) self.override_cells else self.rounds[round].cells;
        if (index >= cells.len) return error.CellRefOutOfRange;
        return cells[index];
    }
};

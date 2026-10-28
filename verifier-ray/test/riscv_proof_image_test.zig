//! Reads the generated honest-proof image as a real `verifier.VerifyInput`.
//!
//! The fixture is `testdata/riscv_proof_image.bin`, generated
//! from the same real `arithmetization/src/main/riscv/main.zkc` proof path
//! (proving `zkc_r5.AllInOneGuestELF`, which exercises the full RV64I +
//! M-extension + custom-precompile surface in a single witness) that emits
//! `testdata/generated/riscv_system.zig`. This is the
//! cross-language end-to-end check: Go writes the native layout bytes, Zig
//! mmaps, relocates and validates them, then the real verifier accepts the
//! proof against the real compiled system. The image is not committed (see
//! .gitignore); `make generate-testdata` produces it together with the
//! riscv_system module this file imports.
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
const fri = verifier_ray.query.fri;
const merkle = verifier_ray.crypto.merkle;
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
    const img = try mapRawImage();
    errdefer _ = munmap(@ptrCast(img.ptr), img.len);
    const rounds_header = @offsetOf(verifier.VerifyInput, "proof") +
        @offsetOf(verifier.Proof, "rounds");
    const stored_rounds_pointer = readU64(img.ptr, rounds_header);

    // The native loader's sequence: relocate, then validate at the mapped layout.
    verifier_ray.image_relocation.rebase(img.ptr, img.len, encoded_base, @intFromPtr(img.ptr));
    try verifier_ray.image_relocation.validateImage(img.ptr, img.len, @intFromPtr(img.ptr));

    const rounds_offset = stored_rounds_pointer - encoded_base;
    try std.testing.expectEqual(
        @as(u64, @intCast(@intFromPtr(img.ptr))) + rounds_offset,
        readU64(img.ptr, rounds_header),
    );

    // A fresh mapping must still see the original pointer bytes, proving the
    // relocation dirtied only the private mapping rather than the file.
    const stored = try mapRawImage();
    defer _ = munmap(@ptrCast(stored.ptr), stored.len);
    try std.testing.expectEqual(stored_rounds_pointer, readU64(stored.ptr, rounds_header));

    return @ptrCast(@alignCast(img.ptr));
}

/// The payload of the slice header at `header` in an image at its encoded
/// layout, as an offset into the image.
fn payload(img: []const u8, header: usize) struct { offset: usize, len: usize } {
    return .{
        .offset = @intCast(readU64(img.ptr, header) - encoded_base),
        .len = @intCast(readU64(img.ptr, header + 8)),
    };
}

/// Maps the fixture privately at any address the OS picks, at its encoded
/// layout. Writes are copy-on-write and cannot modify the stored proof image.
fn mapRawImage() ![]u8 {
    const fd = open(image_path, o_rdonly);
    if (fd < 0) return error.ImageMissing;
    defer _ = close(fd);

    const image_len = lseek(fd, 0, seek_end);
    if (image_len <= 0) return error.ImageMissing;
    const img_len: usize = @intCast(image_len);

    const p = mmap(null, img_len, prot_read | prot_write, map_private, fd, 0);
    if (@intFromPtr(p) == map_failed) return error.MmapFailed;
    const buf: [*]u8 = @ptrCast(p);
    return buf[0..img_len];
}

test "validateImage accepts the honest image and rejects each malformed field" {
    const relocation = verifier_ray.image_relocation;
    const ext = verifier_ray.field.koalabear_ext;
    const protocol = verifier_ray.protocol;
    const poseidon2 = verifier_ray.crypto.poseidon2;

    // A fresh private mapping at its encoded layout (not rebased): the bytes
    // the R5 guest would see at `_in_start`.
    const img = mapRawImage() catch |err| switch (err) {
        error.ImageMissing => return error.SkipZigTest,
        else => return err,
    };
    defer _ = munmap(@ptrCast(img.ptr), img.len);
    try relocation.validateImage(img.ptr, img.len, encoded_base);

    const public_inputs_header = @offsetOf(verifier.VerifyInput, "public_inputs");
    const public_inputs_pointer = readU64(img.ptr, public_inputs_header);
    try std.testing.expect(readU64(img.ptr, public_inputs_header + 8) > 0);
    const public_inputs = public_inputs_pointer - encoded_base;
    const rounds_header = @offsetOf(verifier.VerifyInput, "proof") + @offsetOf(verifier.Proof, "rounds");
    try std.testing.expect(readU64(img.ptr, rounds_header + 8) > 0);
    const rounds = readU64(img.ptr, rounds_header) - encoded_base;

    const Mutation = struct { offset: usize, width: u8, value: u64, err: relocation.Error };
    const fixed = [_]Mutation{
        // Scalar tag outside {base, ext}.
        .{ .offset = public_inputs + @sizeOf(ext.Ext), .width = 1, .value = 2, .err = error.PointerPastImage },
        // Non-canonical limb (>= p).
        .{ .offset = public_inputs, .width = 4, .value = field.modulus, .err = error.PointerPastImage },
        // Optional commitment flag outside {0, 1}.
        .{
            .offset = rounds + @offsetOf(protocol.RoundMessage, "commitment") + @sizeOf(poseidon2.Digest),
            .width = 1,
            .value = 2,
            .err = error.PointerPastImage,
        },
        // Slice pointer misaligned, before the image, or with a payload past it.
        .{ .offset = public_inputs_header, .width = 8, .value = public_inputs_pointer + 1, .err = error.PointerPastImage },
        .{ .offset = public_inputs_header, .width = 8, .value = encoded_base - 8, .err = error.PointerBeforeImage },
        .{ .offset = public_inputs_header + 8, .width = 8, .value = img.len, .err = error.PointerPastImage },
        .{ .offset = public_inputs_header + 8, .width = 8, .value = std.math.maxInt(u64), .err = error.AddressOverflow },
    };
    var mutations: [fixed.len + 5]Mutation = undefined;
    @memcpy(mutations[0..fixed.len], &fixed);
    var count: usize = fixed.len;

    // Nested paths, located by walking the honest image's own headers: a
    // non-canonical limb or an out-of-range optional flag in each.
    const non_canonical: u64 = field.modulus;
    const opening = @offsetOf(verifier.VerifyInput, "proof") + @offsetOf(verifier.Proof, "pcs_opening") +
        @offsetOf(verifier.PcsOpening, "proof");
    const fri_proof = opening + @offsetOf(pcs.OpeningProof, "fri_proof");
    const final_poly = payload(img, fri_proof + @offsetOf(fri.Proof, "final_poly"));
    try std.testing.expect(final_poly.len > 0);
    mutations[count] = .{ .offset = final_poly.offset, .width = 4, .value = non_canonical, .err = error.PointerPastImage };
    count += 1;

    const running = payload(img, fri_proof + @offsetOf(fri.Proof, "running_queries"));
    try std.testing.expect(running.len > 0);
    const branches = payload(img, running.offset);
    try std.testing.expect(branches.len > 0);
    mutations[count] = .{
        .offset = branches.offset + @offsetOf(merkle.Branch, "leaf"),
        .width = 4,
        .value = non_canonical,
        .err = error.PointerPastImage,
    };
    count += 1;

    const caps = payload(img, opening + @offsetOf(pcs.OpeningProof, "input_caps"));
    if (caps.len > 0) {
        const nodes = payload(img, caps.offset + @offsetOf(pcs.InputCap, "nodes"));
        if (nodes.len > 0) {
            mutations[count] = .{ .offset = nodes.offset, .width = 4, .value = non_canonical, .err = error.PointerPastImage };
            count += 1;
        }
    }

    const round_caps = payload(img, fri_proof + @offsetOf(fri.Proof, "round_caps"));
    if (round_caps.len > 0) {
        const aux = payload(img, round_caps.offset + @offsetOf(merkle.MerkleCap, "aux"));
        if (aux.len > 0) {
            mutations[count] = .{ .offset = aux.offset + @sizeOf(poseidon2.Digest), .width = 1, .value = 2, .err = error.PointerPastImage };
            count += 1;
        }
    }

    const queries = payload(img, opening + @offsetOf(pcs.OpeningProof, "input_queries"));
    try std.testing.expect(queries.len > 0);
    const openings = payload(img, queries.offset);
    if (openings.len > 0) {
        const leaves = payload(img, openings.offset + @offsetOf(merkle.InputTreeOpening, "leaves"));
        if (leaves.len > 0) {
            mutations[count] = .{ .offset = leaves.offset + @sizeOf(merkle.RowPair), .width = 1, .value = 2, .err = error.PointerPastImage };
            count += 1;
        }
    }

    // The honest image populates every nested path above; if a fixture change
    // empties one, this fails rather than silently dropping its coverage.
    try std.testing.expectEqual(mutations.len, count);
    for (mutations[0..count]) |m| {
        var saved: [8]u8 = undefined;
        @memcpy(saved[0..m.width], img[m.offset..][0..m.width]);
        switch (m.width) {
            1 => img[m.offset] = @intCast(m.value),
            4 => std.mem.writeInt(u32, img[m.offset..][0..4], @intCast(m.value), .little),
            8 => std.mem.writeInt(u64, img[m.offset..][0..8], m.value, .little),
            else => unreachable,
        }
        try std.testing.expectError(m.err, relocation.validateImage(img.ptr, img.len, encoded_base));
        @memcpy(img[m.offset..][0..m.width], saved[0..m.width]);
    }
    // Restored: the honest image validates again.
    try relocation.validateImage(img.ptr, img.len, encoded_base);

    // Each alignment requirement on its own: the bytes being read, and the
    // base the verifier's pointers are relative to.
    try std.testing.expectError(
        error.MisalignedImage,
        relocation.validateImage(img.ptr + 4, img.len - 4, encoded_base),
    );
    try std.testing.expectError(
        error.MisalignedImage,
        relocation.validateImage(img.ptr, img.len, encoded_base + 4),
    );
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
    } else |_| {}
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

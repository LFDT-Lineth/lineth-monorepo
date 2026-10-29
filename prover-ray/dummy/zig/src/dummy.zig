//! The dummy verifier: a standalone verifier for the dummy circuit of
//! prover-ray/dummy, with the signatures of verifier-ray. A guest builds
//! against it in place of verifier-ray by importing this file as both the
//! `verifier_ray` and the `riscv_system` modules; its code does not change.
//!
//! The circuit is fixed and embedded below (`system_0_*`). Its public inputs X
//! are those of the conglomeration root: the guest-output hash (32 cells), then
//! the program verification key (8 cells, a placeholder until its encoding is
//! fixed). A dummy proof has the shape of a ray proof: one round message whose
//! cells are the witness Y, no commitment, no module sizes, an empty PCS
//! opening. It is accepted iff, limb by limb over KoalaBear,
//!
//!     Y_i = X_i^5
//!
//! x ↦ x^5 is one-to-one on KoalaBear, so a proof is accepted for exactly the
//! public inputs it was made for. Anyone who knows X can produce a dummy proof:
//! test builds only. Depends on Zig std only.

const std = @import("std");
const builtin = @import("builtin");

// ── The dummy circuit ────────────────────────────────────────────────────────

/// Number of public inputs: 32 guest-output cells + 8 program-VK cells.
/// prover-ray/dummy's NumPublicInputs is the same number.
pub const num_public_inputs = 32 + 8;

pub const system_0_public_input = protocol.public_input.Spec{
    .round_cell_counts = &.{2 * num_public_inputs},
    .refs = &refs,
};

const refs = blk: {
    var r: [num_public_inputs]protocol.public_input.CellRef = undefined;
    for (&r, 0..) |*ref, i| ref.* = .{ .statement_index = i, .round = 0, .index = i };
    break :blk r;
};

pub const system_0_spec = protocol.Spec{};

pub const system_0_systems = verifier.Systems{ .public_input = system_0_public_input };

// ── Field ───────────────────────────────────────────────────────────────────

pub const field = struct {
    pub const modulus: u32 = 2_130_706_433;

    /// A KoalaBear element, stored canonically (in [0, p)) as in a proof image.
    pub const Element = extern struct {
        value: u32,

        pub fn mul(self: Element, rhs: Element) Element {
            return .{ .value = @intCast(@as(u64, self.value) * rhs.value % modulus) };
        }

        pub fn pow5(self: Element) Element {
            const sq = self.mul(self);
            return sq.mul(sq).mul(self);
        }
    };

    pub const E2 = extern struct { a0: Element, a1: Element };

    /// Degree-6 extension element; limbs in memory order B0.a0, B0.a1, B1.a0,
    /// B1.a1, B2.a0, B2.a1.
    pub const Ext = extern struct {
        B0: E2,
        B1: E2,
        B2: E2,

        pub const bytes = 6 * 4;

        pub fn lift(value: Element) Ext {
            const z = Element{ .value = 0 };
            return .{ .B0 = .{ .a0 = value, .a1 = z }, .B1 = .{ .a0 = z, .a1 = z }, .B2 = .{ .a0 = z, .a1 = z } };
        }

        pub fn limbs(self: Ext) [6]u32 {
            return .{ self.B0.a0.value, self.B0.a1.value, self.B1.a0.value, self.B1.a1.value, self.B2.a0.value, self.B2.a1.value };
        }

        pub fn eql(self: Ext, rhs: Ext) bool {
            return std.mem.eql(u32, &self.limbs(), &rhs.limbs());
        }

        /// x^5, limb by limb: the dummy relation.
        pub fn pow5Limbs(self: Ext) Ext {
            const p = struct {
                fn f(e: E2) E2 {
                    return .{ .a0 = e.a0.pow5(), .a1 = e.a1.pow5() };
                }
            };
            return .{ .B0 = p.f(self.B0), .B1 = p.f(self.B1), .B2 = p.f(self.B2) };
        }

        /// Canonical encoding: the six limbs as big-endian u32, in memory order.
        pub fn toBytes(self: Ext) [bytes]u8 {
            var out: [bytes]u8 = undefined;
            for (self.limbs(), 0..) |limb, i| std.mem.writeInt(u32, out[i * 4 ..][0..4], limb, .big);
            return out;
        }
    };

    /// A base or an extension value; same declaration, hence same layout, as
    /// verifier-ray's: a 24-byte Ext payload, then the tag.
    pub const Scalar = union(enum) {
        base: Element,
        ext: Ext,

        pub fn toExt(self: Scalar) Ext {
            return switch (self) {
                .base => |b| Ext.lift(b),
                .ext => |e| e,
            };
        }
    };
};

// ── Protocol types (verifier-ray's names and fields) ──────────────────────────

pub const protocol = struct {
    pub const Scalar = field.Scalar;
    pub const Commitment = [8]field.Element;

    pub const RoundMessage = struct {
        commitment: ?Commitment = null,
        cells: []const Scalar = &.{},
    };

    /// The dummy protocol derives no coins, so its spec carries nothing.
    pub const Spec = struct {};

    pub const public_input = struct {
        pub const CellRef = struct {
            statement_index: usize,
            round: usize,
            index: usize,
        };

        pub const Spec = struct {
            round_cell_counts: []const usize = &.{},
            refs: []const CellRef = &.{},
        };
    };
};

// ── Verifier (verifier-ray's signatures) ──────────────────────────────────────

pub const verifier = struct {
    /// Tag of the pair-statement encoding: not verifier-ray's
    /// "lineth-verifier-pair-v1", so a dummy statement is never taken for a
    /// real one.
    pub const pair_statement_domain = "lineth-verifier-dummy-pair-v1";

    pub const Systems = struct {
        public_input: protocol.public_input.Spec = .{},
    };

    pub const PublicInput = []const protocol.Scalar;

    /// A ray proof's PCS opening. A dummy proof's is empty and never read; it
    /// is kept as opaque bytes so that `Proof` has verifier-ray's layout.
    pub const PcsOpening = struct {
        proof: [96]u8 align(8),
    };

    pub const Proof = struct {
        rounds: []const protocol.RoundMessage,
        module_sizes: []const usize = &.{},
        pcs_opening: PcsOpening,
    };

    pub const VerifyInput = struct {
        proof: Proof,
        public_inputs: PublicInput = &.{},
    };

    pub const AggregatorInput = struct {
        a: *const VerifyInput,
        b: *const VerifyInput,
    };

    /// Caller-owned scratch. The dummy needs none.
    pub fn Workspace(comptime spec: protocol.public_input.Spec) type {
        _ = spec;
        return struct {};
    }

    pub fn verify(
        comptime spec: protocol.Spec,
        comptime systems: Systems,
        proof: Proof,
        public_inputs: PublicInput,
    ) !void {
        var workspace: Workspace(systems.public_input) = .{};
        return verifyWithWorkspace(spec, systems, proof, public_inputs, &workspace);
    }

    pub fn verifyWithWorkspace(
        comptime spec: protocol.Spec,
        comptime systems: Systems,
        proof: Proof,
        public_inputs: PublicInput,
        workspace: *Workspace(systems.public_input),
    ) !void {
        _ = spec;
        _ = workspace;
        const n = systems.public_input.refs.len;

        // A real ray proof commits to its rounds, so it fails here.
        if (proof.rounds.len != 1 or proof.rounds[0].commitment != null or proof.module_sizes.len != 0)
            return error.NotADummyProof;
        if (public_inputs.len != n) return error.InvalidPublicInputCount;
        const y = proof.rounds[0].cells;
        if (y.len != n) return error.InvalidProofLength;

        for (public_inputs, y) |xi, yi| {
            if (!yi.toExt().eql(xi.toExt().pow5Limbs())) return error.DummyRelationViolated;
        }
    }

    pub fn verifyPair(
        comptime spec: protocol.Spec,
        comptime systems: Systems,
        a: VerifyInput,
        b: VerifyInput,
    ) !void {
        var workspace: Workspace(systems.public_input) = .{};
        return verifyPairWithWorkspace(spec, systems, a, b, &workspace);
    }

    /// Two proofs of the same statement: equal public inputs first, then each
    /// proof, as verifier-ray's `verifyPair`.
    pub fn verifyPairWithWorkspace(
        comptime spec: protocol.Spec,
        comptime systems: Systems,
        a: VerifyInput,
        b: VerifyInput,
        workspace: *Workspace(systems.public_input),
    ) !void {
        if (a.public_inputs.len != systems.public_input.refs.len or
            b.public_inputs.len != systems.public_input.refs.len)
        {
            return error.InvalidPublicInputCount;
        }
        try checkPublicInputConsistency(a.public_inputs, b.public_inputs);
        try verifyWithWorkspace(spec, systems, a.proof, a.public_inputs, workspace);
        try verifyWithWorkspace(spec, systems, b.proof, b.public_inputs, workspace);
    }

    pub fn checkPublicInputConsistency(
        a: PublicInput,
        b: PublicInput,
    ) error{InconsistentPublicInputs}!void {
        if (a.len != b.len) return error.InconsistentPublicInputs;
        for (a, b) |sa, sb| {
            if (!sa.toExt().eql(sb.toExt())) return error.InconsistentPublicInputs;
        }
    }

    pub fn pairStatementEncodedLen(comptime count: usize) usize {
        return pairStatementLen(count);
    }

    /// out = pair_statement_domain || BE64(n) || enc(X[0]) || … || enc(X[n-1]).
    pub fn encodePairStatement(statement: PublicInput, out: []u8) !void {
        if (out.len != pairStatementLen(statement.len)) return error.InvalidPairStatementBuffer;
        @memcpy(out[0..pair_statement_domain.len], pair_statement_domain);
        std.mem.writeInt(u64, out[pair_statement_domain.len..][0..8], statement.len, .big);
        for (statement, 0..) |scalar, i| {
            @memcpy(out[pair_statement_domain.len + 8 + i * field.Ext.bytes ..][0..field.Ext.bytes], &scalar.toExt().toBytes());
        }
    }

    fn pairStatementLen(count: usize) usize {
        return pair_statement_domain.len + @sizeOf(u64) + count * field.Ext.bytes;
    }
};

// ── Proof images (verifier-ray's image_relocation entry points) ───────────────

/// A dummy proof image has the format of any ray proof image (prover-ray's
/// proofserialization): little-endian, slices as {ptr, len} with absolute
/// pointers for an encoded base, limbs as canonical u32. Only the parts a dummy
/// proof uses are walked; the PCS opening is left untouched.
pub const image_relocation = struct {
    pub const Error = error{
        PointerBeforeImage,
        PointerPastImage,
        AddressOverflow,
        MisalignedImage,
    };

    pub const pair_header_len = 16;

    pub const PairOffsets = struct { a: usize, b: usize };

    const VerifyInput = verifier.VerifyInput;
    const slice = 16;
    const public_inputs_at = @offsetOf(VerifyInput, "public_inputs");
    const rounds_at = @offsetOf(verifier.Proof, "rounds");
    const module_sizes_at = @offsetOf(verifier.Proof, "module_sizes");
    const round_size = @sizeOf(protocol.RoundMessage);
    const cells_at = @offsetOf(protocol.RoundMessage, "cells");
    const commitment_flag_at = @offsetOf(protocol.RoundMessage, "commitment") + @sizeOf(protocol.Commitment);
    const scalar_size = @sizeOf(protocol.Scalar);

    comptime {
        if (@bitSizeOf(usize) != 64 or builtin.cpu.arch.endian() != .little)
            @compileError("image_relocation assumes a 64-bit little-endian target");
        // The layout proofserialization writes (prover-ray/wiop/proofserialization/layout.go).
        const pins = .{
            .{ @sizeOf(VerifyInput), 144 },    .{ public_inputs_at, 128 },
            .{ @sizeOf(verifier.Proof), 128 }, .{ rounds_at, 0 },
            .{ module_sizes_at, 16 },          .{ @offsetOf(verifier.Proof, "pcs_opening"), 32 },
            .{ round_size, 56 },               .{ cells_at, 0 },
            .{ commitment_flag_at, 48 },       .{ scalar_size, 28 },
        };
        for (pins) |p| if (p[0] != p[1]) @compileError("proof image layout differs from proofserialization's");
    }

    pub fn imageOffset(encoded_pointer: u64, encoded_base: usize) Error!usize {
        if (encoded_pointer < encoded_base) return error.PointerBeforeImage;
        return encoded_pointer - encoded_base;
    }

    /// Patches the slice pointers of an image from `encoded_base + offset` to
    /// `mapped_base + offset`, in place. Memory-safe on any bytes; validate the
    /// result with `validateImage(img, img_len, mapped_base)`.
    pub fn rebase(img: [*]u8, img_len: usize, encoded_base: usize, mapped_base: usize) void {
        const w = Walk{ .img = img, .len = img_len, .encoded_base = encoded_base, .mapped_base = mapped_base };
        _ = w.patch(public_inputs_at);
        _ = w.patch(module_sizes_at);
        const rounds = w.patch(rounds_at) orelse return;
        for (0..@min(rounds.len, (img_len - rounds.offset) / round_size)) |i| {
            _ = w.patch(rounds.offset + i * round_size + cells_at);
        }
    }

    /// Validates an untrusted image before it is cast to a `VerifyInput`.
    pub fn validateImage(img: [*]const u8, img_len: usize, encoded_base: usize) Error!void {
        if (@intFromPtr(img) % 8 != 0 or encoded_base % 8 != 0) return error.MisalignedImage;
        try requireRange(0, @sizeOf(VerifyInput), img_len);
        try validateScalars(img, try resolve(img, img_len, public_inputs_at, scalar_size, 4, encoded_base));
        _ = try resolve(img, img_len, module_sizes_at, 8, 8, encoded_base);
        const rounds = try resolve(img, img_len, rounds_at, round_size, 8, encoded_base);
        for (0..rounds.len) |i| {
            const round = rounds.offset + i * round_size;
            const flag = img[round + commitment_flag_at];
            if (flag > 1) return error.PointerPastImage;
            if (flag == 1) try validateLimbs(img, round + commitment_flag_at - @sizeOf(protocol.Commitment), 8);
            try validateScalars(img, try resolve(img, img_len, round + cells_at, scalar_size, 4, encoded_base));
        }
    }

    pub fn parsePairHeader(img: [*]const u8, img_len: usize, encoded_base: usize) Error!PairOffsets {
        try requireRange(0, pair_header_len, img_len);
        const a = try imageOffset(readU64(img, 0), encoded_base);
        const b = try imageOffset(readU64(img, 8), encoded_base);
        if (a != pair_header_len) return error.PointerPastImage;
        if (b < a + @sizeOf(VerifyInput) or b % 8 != 0) return error.PointerPastImage;
        try requireRange(b, @sizeOf(VerifyInput), img_len);
        return .{ .a = a, .b = b };
    }

    pub fn validatePair(img: [*]const u8, img_len: usize, encoded_base: usize) Error!PairOffsets {
        const pair = try parsePairHeader(img, img_len, encoded_base);
        try validateImage(img + pair.a, pair.b - pair.a, encoded_base + pair.a);
        try validateImage(img + pair.b, img_len - pair.b, encoded_base + pair.b);
        return pair;
    }

    pub fn rebasePair(img: [*]u8, img_len: usize, encoded_base: usize, mapped_base: usize) Error!void {
        const pair = try parsePairHeader(img, img_len, encoded_base);
        rebase(img + pair.a, pair.b - pair.a, encoded_base + pair.a, mapped_base + pair.a);
        rebase(img + pair.b, img_len - pair.b, encoded_base + pair.b, mapped_base + pair.b);
        writeU64(img, 0, mapped_base + pair.a);
        writeU64(img, 8, mapped_base + pair.b);
    }

    const Region = struct { offset: usize, len: usize };

    const Walk = struct {
        img: [*]u8,
        len: usize,
        encoded_base: usize,
        mapped_base: usize,

        fn patch(w: Walk, header: usize) ?Region {
            if (header > w.len or w.len - header < slice) return null;
            const pointer = readU64(w.img, header);
            if (pointer < w.encoded_base or pointer - w.encoded_base > w.len) return null;
            const offset: usize = pointer - w.encoded_base;
            writeU64(w.img, header, w.mapped_base + offset);
            return .{ .offset = offset, .len = readU64(w.img, header + 8) };
        }
    };

    fn resolve(img: [*]const u8, img_len: usize, header: usize, elem_size: usize, alignment: usize, encoded_base: usize) Error!Region {
        try requireRange(header, slice, img_len);
        const offset = try imageOffset(readU64(img, header), encoded_base);
        const len: usize = readU64(img, header + 8);
        if (offset % alignment != 0) return error.PointerPastImage;
        const extent = std.math.mul(usize, len, elem_size) catch return error.AddressOverflow;
        try requireRange(offset, extent, img_len);
        return .{ .offset = offset, .len = len };
    }

    fn validateScalars(img: [*]const u8, scalars: Region) Error!void {
        for (0..scalars.len) |i| {
            const scalar = scalars.offset + i * scalar_size;
            if (img[scalar + @sizeOf(field.Ext)] > 1) return error.PointerPastImage;
            try validateLimbs(img, scalar, 6);
        }
    }

    fn validateLimbs(img: [*]const u8, at: usize, count: usize) Error!void {
        for (0..count) |k| {
            if (std.mem.readInt(u32, img[at + 4 * k ..][0..4], .little) >= field.modulus) return error.PointerPastImage;
        }
    }

    fn requireRange(offset: usize, len: usize, image_len: usize) Error!void {
        const end = std.math.add(usize, offset, len) catch return error.AddressOverflow;
        if (end > image_len) return error.PointerPastImage;
    }

    fn readU64(img: [*]const u8, at: usize) u64 {
        return std.mem.readInt(u64, img[at..][0..8], .little);
    }

    fn writeU64(img: [*]u8, at: usize, v: u64) void {
        std.mem.writeInt(u64, img[at..][0..8], v, .little);
    }
};

//! The dummy verifier: a verifier for the dummy circuit of prover-ray/dummy,
//! with the signatures of verifier-ray. A guest builds against it in place of
//! verifier-ray by importing this file as both the `verifier_ray` and the
//! `riscv_system` modules; its code does not change.
//!
//! The circuit is fixed and embedded below (`system_0_*`). Its public inputs X
//! are those of the conglomeration root: the guest-output hash (32 cells), then
//! the program verification key (8 cells, one per limb of the program-round
//! commitment). A dummy proof is a ray proof image: one round message whose
//! cells are the witness Y, no commitment, no module sizes, an empty PCS
//! opening. It is accepted iff, limb by limb over KoalaBear,
//!
//!     Y_i = X_i^5
//!
//! x ↦ x^5 is one-to-one on KoalaBear, so a proof is accepted for exactly the
//! public inputs it was made for. Anyone who knows X can produce a dummy proof:
//! test builds only.
//!
//! Everything but the check comes from verifier-ray: the field, the proof types
//! and `image_relocation`.

const std = @import("std");
const vr = @import("verifier_ray_lib");

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

// ── From verifier-ray ────────────────────────────────────────────────────────

pub const field = vr.field;
pub const image_relocation = vr.image_relocation;

pub const protocol = struct {
    pub const Scalar = vr.protocol.Scalar;
    pub const Commitment = vr.protocol.Commitment;
    pub const RoundMessage = vr.protocol.RoundMessage;
    pub const public_input = vr.protocol.public_input;

    /// The dummy protocol derives no coins, so its spec carries nothing.
    pub const Spec = struct {};
};

const Element = field.koalabear.Element;
const Ext = field.koalabear_ext.Ext;

// ── Verifier (verifier-ray's signatures; only the check is the dummy's) ──────

pub const verifier = struct {
    pub const PublicInput = vr.verifier.PublicInput;
    pub const PcsOpening = vr.verifier.PcsOpening;
    pub const Proof = vr.verifier.Proof;
    pub const VerifyInput = vr.verifier.VerifyInput;
    pub const AggregatorInput = vr.verifier.AggregatorInput;
    pub const checkPublicInputConsistency = vr.verifier.checkPublicInputConsistency;

    /// Tag of the pair-statement encoding: not verifier-ray's
    /// "lineth-verifier-pair-v1", so a dummy statement is never taken for a
    /// real one.
    pub const pair_statement_domain = "lineth-verifier-dummy-pair-v1";

    pub const Systems = struct {
        public_input: protocol.public_input.Spec = .{},
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
        const opening = proof.pcs_opening.proof;
        if (opening.input_queries.len != 0 or opening.input_caps.len != 0 or
            opening.fri_proof.round_roots.len != 0 or opening.fri_proof.round_caps.len != 0 or
            opening.fri_proof.final_poly.len != 0 or opening.fri_proof.running_queries.len != 0)
            return error.NotADummyProof;
        if (public_inputs.len != n) return error.InvalidPublicInputCount;
        const y = proof.rounds[0].cells;
        if (y.len != n) return error.InvalidProofLength;

        for (public_inputs, y) |xi, yi| {
            if (!yi.toExt().eql(pow5Limbs(xi.toExt()))) return error.DummyRelationViolated;
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

    pub fn pairStatementEncodedLen(comptime count: usize) usize {
        return pairStatementLen(count);
    }

    /// out = pair_statement_domain || BE64(n) || enc(X[0]) || … || enc(X[n-1]),
    /// as verifier-ray's, under the dummy domain tag.
    pub fn encodePairStatement(statement: PublicInput, out: []u8) !void {
        if (out.len != pairStatementLen(statement.len)) return error.InvalidPairStatementBuffer;
        @memcpy(out[0..pair_statement_domain.len], pair_statement_domain);
        std.mem.writeInt(u64, out[pair_statement_domain.len..][0..8], statement.len, .big);
        for (statement, 0..) |scalar, i| {
            @memcpy(out[pair_statement_domain.len + 8 + i * field.koalabear_ext.bytes ..][0..field.koalabear_ext.bytes], &scalar.toExt().toBytes());
        }
    }

    fn pairStatementLen(count: usize) usize {
        return pair_statement_domain.len + @sizeOf(u64) + count * field.koalabear_ext.bytes;
    }
};

/// The dummy relation on one value: x^5, limb by limb.
fn pow5Limbs(x: Ext) Ext {
    var limbs = x.toUints();
    for (&limbs) |*limb| limb.* = Element.init(limb.*).powComptime(5).value;
    return Ext.fromUints(limbs);
}

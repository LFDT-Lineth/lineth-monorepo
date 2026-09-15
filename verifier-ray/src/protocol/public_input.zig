const types = @import("types.zig");

pub const Scalar = types.Scalar;
pub const Commitment = types.Commitment;
pub const RoundMessage = types.RoundMessage;

pub const Error = error{
    InvalidRoundCount,
    InvalidPublicInputCount,
    InvalidRoundCellCount,
};

/// One registered public input, referenced by both its position in the flat
/// statement vector (`statement_index`) and the transcript cell slot it occupies
/// after binding (`round`, `index`). Codegen emits these in transcript order so
/// the verifier can merge `public_inputs` into the proof's round cells in one
/// pass while still reading the statement in prover-ray registration order.
pub const CellRef = struct {
    statement_index: usize,
    round: usize,
    index: usize,
};

/// Compile-time layout for rebuilding the verifier-visible transcript cell
/// stream from prover-ray's split `(Proof, PublicInput)` wire format.
pub const Spec = struct {
    /// Total verifier-visible cell count of each replayed round, INCLUDING the
    /// cells that live in the separate public-input statement.
    round_cell_counts: []const usize = &.{},
    /// Registered public inputs, emitted in transcript order (round/index) so
    /// `bindRoundMessages` can merge them into the proof's round cells in one
    /// pass. `statement_index` still points into the prover-ray registration
    /// order of the flat public-input vector.
    refs: []const CellRef = &.{},
};

/// Returns a stack-only container holding the replayed rounds after the flat
/// public-input statement has been merged back into their transcript cell slots.
/// Each `rounds[i].cells` slice points into `cell_storage[i]`.
pub fn BoundRoundMessages(comptime spec: Spec) type {
    const round_count = spec.round_cell_counts.len;
    const max_cells = comptime maxRoundCellCount(spec);
    return struct {
        const Self = @This();
        const cell_cap = @max(max_cells, 1);

        round_commitments: [round_count]?Commitment = undefined,
        cell_counts: [round_count]usize = undefined,
        rounds_buf: [round_count]RoundMessage = undefined,
        cell_storage: [round_count][cell_cap]Scalar = undefined,

        pub fn rounds(self: *Self) []const RoundMessage {
            inline for (0..round_count) |round_index| {
                self.rounds_buf[round_index] = .{
                    .commitment = self.round_commitments[round_index],
                    .cells = self.cell_storage[round_index][0..self.cell_counts[round_index]],
                };
            }
            return self.rounds_buf[0..];
        }
    };
}

/// Merges prover-ray-style public inputs into the proof's round messages so the
/// transcript replay and sub-verifiers see the same `(round, index)` cell
/// coordinates the compiled metadata was generated from. The proof round cells
/// omit any registered public-input cells; `public_inputs` supplies them in
/// registration order, matching prover-ray's `PublicInput`.
pub fn bindRoundMessages(
    comptime spec: Spec,
    rounds: []const RoundMessage,
    public_inputs: []const Scalar,
) Error!BoundRoundMessages(spec) {
    comptime validateSpec(spec);

    if (rounds.len != spec.round_cell_counts.len) return error.InvalidRoundCount;
    if (public_inputs.len != spec.refs.len) return error.InvalidPublicInputCount;

    var bound: BoundRoundMessages(spec) = undefined;
    var public_input_cursor: usize = 0;

    inline for (0..spec.round_cell_counts.len) |round_index| {
        const proof_round = rounds[round_index];
        const total_cells = spec.round_cell_counts[round_index];

        bound.round_commitments[round_index] = proof_round.commitment;
        bound.cell_counts[round_index] = total_cells;

        var proof_cell_index: usize = 0;
        for (0..total_cells) |cell_index| {
            if (public_input_cursor < spec.refs.len and
                spec.refs[public_input_cursor].round == round_index and
                spec.refs[public_input_cursor].index == cell_index)
            {
                const ref = spec.refs[public_input_cursor];
                bound.cell_storage[round_index][cell_index] = public_inputs[ref.statement_index];
                public_input_cursor += 1;
                continue;
            }

            if (proof_cell_index >= proof_round.cells.len) return error.InvalidRoundCellCount;
            bound.cell_storage[round_index][cell_index] = proof_round.cells[proof_cell_index];
            proof_cell_index += 1;
        }

        if (proof_cell_index != proof_round.cells.len) return error.InvalidRoundCellCount;
    }

    return bound;
}

fn maxRoundCellCount(comptime spec: Spec) usize {
    var max_cells: usize = 0;
    for (spec.round_cell_counts) |count| {
        if (count > max_cells) max_cells = count;
    }
    return max_cells;
}

fn validateSpec(comptime spec: Spec) void {
    // The statement_index uniqueness scan below is O(refs.len^2); the
    // shared-randomness contribution alone contributes 328 refs, which pushes
    // some cases past the compiler's default 1000-backwards-branch quota.
    @setEvalBranchQuota(1_000_000);
    for (spec.refs, 0..) |ref, i| {
        if (ref.statement_index >= spec.refs.len)
            @compileError("public_input spec: refs statement_index out of range");
        if (ref.round >= spec.round_cell_counts.len)
            @compileError("public_input spec: refs round out of range");
        if (ref.index >= spec.round_cell_counts[ref.round])
            @compileError("public_input spec: refs index out of range for round");

        if (i > 0) {
            const prev = spec.refs[i - 1];
            if (ref.round < prev.round or (ref.round == prev.round and ref.index <= prev.index))
                @compileError("public_input spec: refs must be strictly sorted by (round, index)");
        }

        for (spec.refs[i + 1 ..]) |other| {
            if (other.statement_index == ref.statement_index)
                @compileError("public_input spec: refs statement_index values must be unique");
        }
    }
}

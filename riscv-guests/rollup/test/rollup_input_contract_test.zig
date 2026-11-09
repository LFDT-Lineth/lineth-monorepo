//! The rollup guest input matches the spec's bytes: the committed `.ssz` decodes to the values of
//! the request JSON, and the codec encodes them back to the same bytes.

const std = @import("std");

const contract = @import("ssz_contract");
const l2_execution_proof = @import("core_serde").l2_execution_proof;
const rollup_ssz = @import("rollup").rollup_ssz;
const testdata = @import("spec_testdata").dir;

const fixture = "10-14-getZkRollupProofV1.request";

/// The request JSON holds the boundary hash as `null` or the hash itself, and an l2-execution
/// proof flat with its program VK beside it.
const Rules = struct {
    pub fn rule(comptime T: type, comptime field: []const u8) contract.Rule {
        if (contract.isField(T, field, rollup_ssz.RollupProofPrivateInput, "boundary_prev_data_rolling_hash")) return .nullable;
        if (contract.isField(T, field, l2_execution_proof.VerifiableL2ExecutionProof, "proof")) return .flatten;
        return .default;
    }
};

test "input: the spec's bytes decode to the request JSON and encode back" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const bytes = try contract.readFixture(alloc, std.testing.io, testdata, fixture ++ ".ssz");
    const json = try contract.readJson(alloc, std.testing.io, testdata, fixture ++ ".json");

    const input = try rollup_ssz.decodeInput(alloc, bytes);
    try contract.expectMatches(rollup_ssz.RollupProofPrivateInput, Rules, alloc, input, json.object.get("proofRequest").?);

    const encoded = try rollup_ssz.encodeInput(alloc, input);
    try std.testing.expectEqualSlices(u8, bytes, encoded);
}

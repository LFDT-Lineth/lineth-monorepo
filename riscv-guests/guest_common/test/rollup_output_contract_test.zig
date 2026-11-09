//! The rollup guest output matches the spec's bytes: the committed `.ssz` decodes to the values of
//! the response JSON, and the codec encodes them back to the same bytes.

const std = @import("std");

const contract = @import("ssz_contract");
const rollup_proof = @import("core_serde").rollup_proof;
const testdata = @import("spec_testdata").dir;

const fixture = "10-14-getZkRollupProofV1.response";

test "output: the spec's bytes decode to the response JSON and encode back" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const output = try contract.readFixture(alloc, std.testing.io, testdata, fixture ++ ".ssz");
    const json = try contract.readJson(alloc, std.testing.io, testdata, fixture ++ ".json");

    const public_inputs = try rollup_proof.decodeOutput(alloc, output);
    try contract.expectMatches(rollup_proof.RollupPublicInput, contract.NoRules, alloc, public_inputs, json.object.get("publicInputs").?);

    const encoded = try rollup_proof.encodeOutput(alloc, public_inputs);
    try std.testing.expectEqualSlices(u8, output, encoded);
}

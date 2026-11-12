//! The l2-execution guest output matches the spec's bytes: the committed `.ssz` decodes to the
//! values of the response JSON, and the codec encodes them back to the same bytes.

const std = @import("std");

const contract = @import("ssz_contract");
const l2_execution_proof = @import("core_serde").l2_execution_proof;
const testdata = @import("spec_testdata").dir;

const fixture = "10-11-getZkL2ExecutionProofV1.response";

test "output: the spec's bytes decode to the response JSON and encode back" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const output = try contract.readFixture(alloc, std.testing.io, testdata, fixture ++ ".ssz");
    const json = try contract.readJson(alloc, std.testing.io, testdata, fixture ++ ".json");

    const body = try contract.framedBody(output, l2_execution_proof.OUTPUT_SCHEMA_ID);
    const public_inputs = try l2_execution_proof.decodePublicInputs(alloc, body);
    try contract.expectMatches(l2_execution_proof.L2ExecutionProofPublicInput, contract.NoRules, alloc, public_inputs, json.object.get("publicInputs").?);

    const encoded = try l2_execution_proof.encodeOutput(alloc, public_inputs);
    try std.testing.expectEqualSlices(u8, output, encoded);
}

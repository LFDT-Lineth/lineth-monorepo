//! The l2-execution guest input matches the spec's bytes: the committed `.ssz` decodes to the
//! values of the request JSON, and the codec encodes them back to the same bytes.

const std = @import("std");

const contract = @import("ssz_contract");
const l2_execution_ssz = @import("l2_execution").l2_execution_ssz;
const testdata = @import("spec_testdata").dir;

const fixture = "10-11-getZkL2ExecutionProofV1.request";

/// The names of `ForcedTransactionAcceptance`, by wire value.
const acceptance_names = [_][]const u8{ "INCLUDED", "BAD_NONCE", "BAD_BALANCE", "FILTERED_ADDRESS_FROM", "FILTERED_ADDRESS_TO" };

/// The request JSON names the parent forced-transaction number `parentFtxNumber`, carries each
/// payload's forced transactions under `rollupExtension` and names acceptances by variant. Its
/// `statelessInput` is a structured object where the wire holds SSZ bytes, so `stateless_input_ssz`
/// is covered by the byte-for-byte round trip and by the spec's own decode of the same file.
const Rules = struct {
    pub fn rule(comptime T: type, comptime field: []const u8) contract.Rule {
        if (contract.isField(T, field, l2_execution_ssz.L2ExecutionProofPrivateInput, "parent_last_processed_ftx_number")) return .{ .path = "parentFtxNumber" };
        if (contract.isField(T, field, l2_execution_ssz.LineaPayloadInput, "stateless_input_ssz")) return .skip;
        if (contract.isField(T, field, l2_execution_ssz.LineaPayloadInput, "forced_transactions")) return .{ .path = "rollupExtension.forcedTransactions" };
        if (contract.isField(T, field, l2_execution_ssz.ForcedTransactionWitness, "acceptance")) return .{ .enum_names = &acceptance_names };
        return .default;
    }
};

test "input: the spec's bytes decode to the request JSON and encode back" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const bytes = try contract.readFixture(alloc, std.testing.io, testdata, fixture ++ ".ssz");
    const json = try contract.readJson(alloc, std.testing.io, testdata, fixture ++ ".json");

    const input = try l2_execution_ssz.decodeInput(alloc, bytes);
    try contract.expectMatches(l2_execution_ssz.L2ExecutionProofPrivateInput, Rules, alloc, input, json.object.get("proofRequest").?);

    const encoded = try l2_execution_ssz.encodeInput(alloc, input);
    try std.testing.expectEqualSlices(u8, bytes, encoded);
}

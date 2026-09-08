const std = @import("std");

const l2_execution_ssz = @import("l2_execution_ssz");

fn repeat32(byte: u8) [32]u8 {
    return @splat(byte);
}

fn repeat20(byte: u8) [20]u8 {
    return @splat(byte);
}

// A hand-built input value — readable, diffable Zig source, not an opaque SSZ blob a human can't
// review or hand-edit when the schema changes. Two payloads, with forced transactions covering a
// non-empty signed_tx_rlp and a different acceptance outcome, exercise every variable-length branch
// this codec has.
fn sampleInput() l2_execution_ssz.L2ExecutionProofPrivateInput {
    const payload0_ftx = [_]l2_execution_ssz.ForcedTransactionWitness{
        .{ .number = 16, .deadline = 1000599, .acceptance = 0, .signed_tx_rlp = &[_]u8{ 0x02, 0xf8, 0x6b } },
    };
    const payload1_ftx = [_]l2_execution_ssz.ForcedTransactionWitness{
        .{ .number = 17, .deadline = 1000600, .acceptance = 0, .signed_tx_rlp = &[_]u8{} },
        .{ .number = 18, .deadline = 1000601, .acceptance = 4, .signed_tx_rlp = &[_]u8{} },
    };
    const payloads = [_]l2_execution_ssz.LineaPayloadInput{
        // The vanilla stateless-input bytes are carried opaquely (zero-copy) by this codec — never
        // decoded further here — so a minimal 0x0001-framed placeholder stands in for a real vector.
        .{ .stateless_input_ssz = &[_]u8{ 0x00, 0x01, 0xaa, 0xbb, 0xcc }, .forced_transactions = &payload0_ftx },
        .{ .stateless_input_ssz = &[_]u8{ 0x00, 0x01, 0xdd, 0xee }, .forced_transactions = &payload1_ftx },
    };
    return .{
        .parent_ftx_rolling_hash = repeat32(0x04),
        .parent_last_processed_ftx_number = 100,
        .chain_config = .{
            .l2_message_service_address = repeat20(0x11),
            .coinbase = repeat20(0x00),
            .chain_id = 59144,
        },
        .payloads = &payloads,
    };
}

test "input: encode then decode round-trips every field" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const value = sampleInput();
    const encoded = try l2_execution_ssz.encodeInput(alloc, value);
    const decoded = try l2_execution_ssz.decodeInput(alloc, encoded);

    try std.testing.expectEqual(value.parent_last_processed_ftx_number, decoded.parent_last_processed_ftx_number);
    try std.testing.expectEqualSlices(u8, &value.parent_ftx_rolling_hash, &decoded.parent_ftx_rolling_hash);
    try std.testing.expectEqual(value.chain_config.chain_id, decoded.chain_config.chain_id);
    try std.testing.expectEqualSlices(u8, &value.chain_config.l2_message_service_address, &decoded.chain_config.l2_message_service_address);
    try std.testing.expectEqualSlices(u8, &value.chain_config.coinbase, &decoded.chain_config.coinbase);
    try std.testing.expectEqual(value.payloads.len, decoded.payloads.len);

    for (value.payloads, decoded.payloads) |want, got| {
        try std.testing.expectEqualSlices(u8, want.stateless_input_ssz, got.stateless_input_ssz);
        try std.testing.expectEqual(want.forced_transactions.len, got.forced_transactions.len);
        for (want.forced_transactions, got.forced_transactions) |want_ftx, got_ftx| {
            try std.testing.expectEqual(want_ftx.number, got_ftx.number);
            try std.testing.expectEqual(want_ftx.deadline, got_ftx.deadline);
            try std.testing.expectEqual(want_ftx.acceptance, got_ftx.acceptance);
            try std.testing.expectEqualSlices(u8, want_ftx.signed_tx_rlp, got_ftx.signed_tx_rlp);
        }
    }
}

// `encodeOutput` commits ONLY `keccak256(public_inputs)` — 32 bytes, nothing else — so this asserts
// internal self-consistency against `hashPublicInputs` directly rather than a byte-exact fixture.
test "output: encode commits ONLY hashPublicInputs(public_inputs)" {
    const public_inputs = l2_execution_ssz.L2ExecutionProofPublicInput{
        .parent_block_hash = repeat32(0x0a),
        .end_block_hash = repeat32(0x0b),
        .end_block_number = 1000503,
        .end_block_timestamp = 1763000123,
        .l2_l1_messages_hash = repeat32(0x01),
        .parent_l1_l2_bridge_rolling_hash = repeat32(0x02),
        .parent_l1_l2_bridge_rolling_hash_message_number = 0,
        .end_l1_l2_bridge_rolling_hash = repeat32(0x03),
        .end_l1_l2_bridge_rolling_hash_message_number = 5,
        .dynamic_chain_config_hash = repeat32(0xc0),
        .parent_ftx_rolling_hash = repeat32(0x04),
        .parent_processed_ftx_number = 16,
        .end_ftx_rolling_hash = repeat32(0x05),
        .end_processed_ftx_number = 18,
        .filtered_addresses_hash = repeat32(0x06),
        .tx_froms_hash = repeat32(0x07),
    };

    const out = l2_execution_ssz.encodeOutput(public_inputs);
    const encoded = &out;

    // 2(schema) + 32(hash) = 34 bytes total — ONLY the hash, nothing else.
    try std.testing.expectEqual(@as(usize, 34), encoded.len);
    try std.testing.expectEqualSlices(u8, &[_]u8{ 0x00, 0x03 }, encoded[0..2]); // OUTPUT_SCHEMA_ID

    const pi_hash = l2_execution_ssz.hashPublicInputs(public_inputs);
    try std.testing.expectEqualSlices(u8, &pi_hash, encoded[2..34]);
}

test "input: rejects a body shorter than the fixed head" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const encoded = try l2_execution_ssz.encodeInput(alloc, sampleInput());
    const truncated = encoded[0 .. 2 + 10];
    try std.testing.expectError(error.InvalidSsz, l2_execution_ssz.decodeInput(alloc, truncated));
}

test "input: rejects the wrong schema id" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const encoded = try l2_execution_ssz.encodeInput(alloc, sampleInput());
    var corrupted = try alloc.dupe(u8, encoded);
    corrupted[0] = 0x00;
    corrupted[1] = 0x03; // the output schema id, on input bytes
    try std.testing.expectError(error.InvalidSsz, l2_execution_ssz.decodeInput(alloc, corrupted));
}

// ── Adversarial-input hardening ──────────────────────────────────────────────
//
// `decodeInput` parses prover-controlled bytes inside the guest, so its bounds/ordering checks are
// this codec's actual security boundary. Each test below starts from `encodeInput(sampleInput())`
// and corrupts specific, dynamically-located byte positions — located by reading the same
// offset-table fields `decodeVariableList`/`decodeInput` themselves read, rather than hand-computed
// magic numbers, so a test keeps targeting the right byte even if `sampleInput()`'s field lengths
// change.

fn readU32LE(buf: []const u8, off: usize) u32 {
    return std.mem.readInt(u32, buf[off..][0..4], .little);
}

fn writeU32LE(buf: []u8, off: usize, value: u32) void {
    std.mem.writeInt(u32, buf[off..][0..4], value, .little);
}

fn nextMultipleOf4(n: usize) usize {
    return ((n + 3) / 4) * 4;
}

// Offsets that are protocol-fixed (documented in this module's own header comment) rather than
// attacker-influenced: a 2-byte schema id, then the input's 92-byte fixed head
// (hash(32)+u64(8)+chain_config(20+20+8)+payloads-offset(4)) — so the payloads-offset field sits at
// body offset 88, and the payloads variable region always starts right after it.
const SCHEMA_SIZE: usize = 2;
const INPUT_FIXED_SIZE: usize = 92;
const PAYLOADS_REGION_START: usize = SCHEMA_SIZE + INPUT_FIXED_SIZE;

/// Locate payload0's and payload1's absolute start offsets in an `encodeInput` buffer, by reading
/// the payloads list's own offset table instead of recomputing payload0's encoded length by hand.
fn locatePayloads(encoded: []const u8) struct { payload0_start: usize, payload1_start: usize } {
    const payload0_rel = readU32LE(encoded, PAYLOADS_REGION_START + 0);
    const payload1_rel = readU32LE(encoded, PAYLOADS_REGION_START + 4);
    std.debug.assert(payload0_rel == 8); // sanity: sampleInput() has exactly 2 payloads (table = 2*4 bytes)
    return .{
        .payload0_start = PAYLOADS_REGION_START + payload0_rel,
        .payload1_start = PAYLOADS_REGION_START + payload1_rel,
    };
}

// Class 1: a variable-section offset pointing past the end of the buffer.
test "input: rejects a payload-list offset that points past the end of the buffer" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const encoded = try l2_execution_ssz.encodeInput(alloc, sampleInput());
    const corrupted = try alloc.dupe(u8, encoded);

    // payload1's offset (table[1]) doubles as payload0's END when decodeVariableList checks item
    // 0 (end_i = the next item's start) — pushing it past the payloads region's own length trips
    // the `end_i > data.len` bound check.
    const payloads_region_len = corrupted.len - PAYLOADS_REGION_START;
    const past_end: u32 = @intCast(payloads_region_len + 1000);
    writeU32LE(corrupted, PAYLOADS_REGION_START + 4, past_end);

    try std.testing.expectError(error.InvalidSsz, l2_execution_ssz.decodeInput(alloc, corrupted));
}

// Class 2: non-monotonic/overlapping offsets — a later element's offset smaller than an earlier one's.
test "input: rejects a later payload offset smaller than an earlier one (non-monotonic)" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const encoded = try l2_execution_ssz.encodeInput(alloc, sampleInput());
    const corrupted = try alloc.dupe(u8, encoded);

    // Leave payload0's offset (table[0]) canonical; force payload1's offset (table[1]) below it.
    // decodeVariableList computes item 0's end as table[1], so this makes item 0's start sit AFTER
    // its own end — `off_i > end_i` must reject it.
    const payload0_off = readU32LE(corrupted, PAYLOADS_REGION_START + 0);
    try std.testing.expect(payload0_off >= 4); // sanity: table[0] is at least one 4-byte entry
    writeU32LE(corrupted, PAYLOADS_REGION_START + 4, payload0_off - 4);

    try std.testing.expectError(error.InvalidSsz, l2_execution_ssz.decodeInput(alloc, corrupted));
}

// Class 3: a truncated variable region, cut AFTER the fixed head (distinct from the existing
// shorter-than-fixed-head test above, which never reaches the payloads region at all).
test "input: rejects a variable region truncated mid-payload" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const encoded = try l2_execution_ssz.encodeInput(alloc, sampleInput());
    const payloads = locatePayloads(encoded);

    // Cut the buffer 4 bytes into payload1 (the LAST payload) — short of LineaPayloadInput's own
    // 8-byte fixed head. The outer payloads list accepts this trivially (the last item's end is
    // always "whatever's left"), so this exercises decodeLineaPayloadInput's OWN fixed-head length
    // guard, a different check than the outer list's bounds check covered by class 1 above.
    const cut = payloads.payload1_start + 4;
    try std.testing.expect(cut > PAYLOADS_REGION_START and cut < encoded.len); // sanity: past the fixed head, short of the real end
    const truncated = encoded[0..cut];

    try std.testing.expectError(error.InvalidSsz, l2_execution_ssz.decodeInput(alloc, truncated));
}

// Class 4: an inner (nested) structure lying about its layout — a forced-transaction list whose
// own offset is inconsistent with its ENCLOSING payload's size, not the whole buffer's.
test "input: rejects a nested forced-tx list offset that overruns its enclosing payload" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const encoded = try l2_execution_ssz.encodeInput(alloc, sampleInput());
    const corrupted = try alloc.dupe(u8, encoded);
    const payloads = locatePayloads(corrupted);

    // payload0 is NOT the last payload, so its forced_transactions region is bounded strictly
    // BELOW the rest of the buffer (it ends exactly where payload1 begins) — the right place to
    // prove an inner offset is validated against ITS OWN enclosing region, not just the overall
    // buffer (a value could look "in range" against the latter while still overrunning the former).
    const off_ftx = readU32LE(corrupted, payloads.payload0_start + 4);
    const ftx_region_start = payloads.payload0_start + off_ftx;
    const ftx_region_len = payloads.payload1_start - ftx_region_start;
    const original_ftx_offset = readU32LE(corrupted, ftx_region_start);
    try std.testing.expectEqual(@as(u32, 4), original_ftx_offset); // sanity: payload0 has exactly 1 forced tx

    // Claim the list needs more room than `ftx_region_len` actually provides — still a small,
    // plausible-looking offset well within the OVERALL buffer, so this only fails if the check is
    // properly scoped to payload0's own region rather than the whole remaining buffer.
    const lying_offset: u32 = @intCast(nextMultipleOf4(ftx_region_len + 1));
    writeU32LE(corrupted, ftx_region_start, lying_offset);

    try std.testing.expectError(error.InvalidSsz, l2_execution_ssz.decodeInput(alloc, corrupted));
}

// Class 5 (investigation finding, not a rejection): trailing garbage appended after the last
// variable field. This codec's variable-size fields ALWAYS extend "to the end of the enclosing
// region" for the last element at every nesting level (the same convention used throughout,
// matching the Python reference codec byte-for-byte) — there is no independent total-length field
// anywhere to validate against. Appended bytes are therefore indistinguishable from "the last field
// is simply longer" and are silently absorbed into the deepest-nested last variable field, not
// rejected. Documented here as current, deliberate-tradeoff behavior rather than asserted as a bug:
// every byte in this envelope is already prover-authored, so this affords no capability beyond
// directly encoding a longer field in the first place. Rejecting it would require every decoder in
// this file to report how many bytes it consumed so callers could check for a leftover remainder —
// a return-type change to the whole decode path, not a bounds/ordering guard.
test "input: currently tolerates trailing garbage, absorbed into the last variable-size field" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const value = sampleInput();
    const encoded = try l2_execution_ssz.encodeInput(alloc, value);

    const garbage = [_]u8{ 0xEE, 0xEE, 0xEE, 0xEE, 0xEE, 0xEE };
    const extended = try alloc.alloc(u8, encoded.len + garbage.len);
    @memcpy(extended[0..encoded.len], encoded);
    @memcpy(extended[encoded.len..], &garbage);

    const decoded = try l2_execution_ssz.decodeInput(alloc, extended);

    const want_last_payload = value.payloads[value.payloads.len - 1];
    const want_last_ftx = want_last_payload.forced_transactions[want_last_payload.forced_transactions.len - 1];
    const got_last_payload = decoded.payloads[decoded.payloads.len - 1];
    const got_last_ftx = got_last_payload.forced_transactions[got_last_payload.forced_transactions.len - 1];

    try std.testing.expectEqual(want_last_ftx.signed_tx_rlp.len + garbage.len, got_last_ftx.signed_tx_rlp.len);
    try std.testing.expectEqualSlices(u8, want_last_ftx.signed_tx_rlp, got_last_ftx.signed_tx_rlp[0..want_last_ftx.signed_tx_rlp.len]);
    try std.testing.expectEqualSlices(u8, &garbage, got_last_ftx.signed_tx_rlp[want_last_ftx.signed_tx_rlp.len..]);
}

// Class 6: a huge offset/length value (0xFFFFFFFF-class) must fail cleanly, never attempt a giant
// allocation, panic, or crash.
test "input: rejects a huge (~0xFFFFFFFF) offset without attempting a giant allocation" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const encoded = try l2_execution_ssz.encodeInput(alloc, sampleInput());
    const corrupted = try alloc.dupe(u8, encoded);

    // The payloads list's FIRST offset-table entry doubles as decodeVariableList's item-count
    // driver (n = first_off / 4): a hostile 0xFFFFFFFC would otherwise demand allocating room for
    // ~2^30 element slices. It must be rejected by the `first_off > data.len` bound check BEFORE
    // `alloc.alloc` is ever reached — under the test allocator, a real regression here would
    // surface as error.OutOfMemory (or a multi-GB attempt), not error.InvalidSsz.
    writeU32LE(corrupted, PAYLOADS_REGION_START + 0, 0xFFFFFFFC);

    try std.testing.expectError(error.InvalidSsz, l2_execution_ssz.decodeInput(alloc, corrupted));
}

//! EIP-7934 block RLP size derived from a stateless execution payload.

const primitives = @import("zesu_primitives");
const input = @import("zesu_input");

fn stringSize(len: usize) usize {
    if (len <= 55) return 1 + len;
    return 1 + lengthOfLength(len) + len;
}

fn listSize(payload: usize) usize {
    if (payload <= 55) return 1 + payload;
    return 1 + lengthOfLength(payload) + payload;
}

fn lengthOfLength(n: usize) usize {
    return (@as(usize, 64) - @clz(@as(u64, n)) + 7) / 8;
}

fn uintSize(v: u64) usize {
    if (v == 0) return 1;
    const bytes = lengthOfLength(v);
    if (bytes == 1 and v < 0x80) return 1;
    return 1 + bytes;
}

fn bytesSize(data: []const u8) usize {
    if (data.len == 1 and data[0] < 0x80) return 1;
    return stringSize(data.len);
}

const HASH_SIZE = 33;
const ADDRESS_SIZE = 21;
const BLOOM_SIZE = 259;
const NONCE_SIZE = 9;

fn headerSize(ep: *const input.ExecutionPayload, spec: primitives.SpecId) usize {
    var payload: usize = 6 * HASH_SIZE;
    payload += ADDRESS_SIZE;
    payload += BLOOM_SIZE;
    payload += uintSize(0);
    payload += uintSize(ep.block_number);
    payload += uintSize(ep.gas_limit);
    payload += uintSize(ep.gas_used);
    payload += uintSize(ep.timestamp);
    payload += bytesSize(ep.extra_data);
    payload += NONCE_SIZE;

    if (primitives.isEnabledIn(spec, .london)) payload += uintSize(ep.base_fee_per_gas);
    if (primitives.isEnabledIn(spec, .shanghai)) payload += HASH_SIZE;
    if (primitives.isEnabledIn(spec, .cancun)) {
        payload += uintSize(ep.blob_gas_used);
        payload += uintSize(ep.excess_blob_gas);
        payload += HASH_SIZE;
    }
    if (primitives.isEnabledIn(spec, .prague)) payload += HASH_SIZE;
    if (primitives.isEnabledIn(spec, .amsterdam)) {
        payload += HASH_SIZE;
        payload += uintSize(ep.slot_number orelse 0);
    }

    return listSize(payload);
}

fn transactionsSize(raw_transactions: []const []const u8) usize {
    var payload: usize = 0;
    for (raw_transactions) |raw| {
        payload += if (raw.len > 0 and raw[0] >= 0xc0) raw.len else stringSize(raw.len);
    }
    return listSize(payload);
}

fn withdrawalsSize(withdrawals: []const input.Withdrawal) usize {
    var payload: usize = 0;
    for (withdrawals) |wd| {
        payload += listSize(uintSize(wd.index) + uintSize(wd.validator_index) + ADDRESS_SIZE + uintSize(wd.amount));
    }
    return listSize(payload);
}

/// Returns null when decoded transactions have no matching raw transaction bytes.
pub fn compute(ep: *const input.ExecutionPayload, spec: primitives.SpecId) ?u64 {
    if (ep.raw_transactions.len != ep.transactions.len) return null;

    const payload = headerSize(ep, spec) +
        transactionsSize(ep.raw_transactions) +
        listSize(0) +
        withdrawalsSize(ep.withdrawals);
    return listSize(payload);
}

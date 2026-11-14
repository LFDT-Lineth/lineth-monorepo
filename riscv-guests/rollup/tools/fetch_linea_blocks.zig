//! Builds a Linea mainnet block dataset in the shape of the EF blockchain tests.
//!
//! `fetch_linea_blocks fetch TESTDATA_DIR` (the default command) downloads the blocks
//! `window_first..window_last` from the public RPC with full transaction objects, rebuilds every
//! header and transaction as RLP and checks each against the hash the RPC reports. Every header
//! then gets the two fields Amsterdam adds to the Prague header (the hash of an empty block access
//! list and slot number 0) with each parent hash relinked to the hash of the reshaped block before
//! it, and the guest's own truncation has to accept the reshaped block: it checks the canonical
//! block form and the transactions root, and recovers the senders, which have to be the RPC's.
//! `TESTDATA_DIR/linea_mainnet_<first>_<last>.json.gz` holds one case with `config.chainid`
//! 0xe708 and one entry per block with `rlp`, `blockHeader` (hash, parentHash, mixHash, number,
//! timestamp) and `transactions`. The hashes of the file are those of the reshaped headers, except
//! the first block's parent hash. The gzip stream carries no filename and a zero mtime, so a rerun
//! reproduces the file byte for byte.
//!
//! `fetch_linea_blocks discover` samples block transaction counts through the first week of
//! January 2026 and prints the densest window of `window_blocks` consecutive blocks, the values of
//! the constants below.

const std = @import("std");
const lineth_da = @import("lineth_da");
const zkvm_accelerators = @import("zkvm_accelerators");

const Allocator = std.mem.Allocator;
const Io = std.Io;
const Value = std.json.Value;
const truncation = lineth_da.encoding.truncation;

const rpc_url = "https://rpc.linea.build";
const user_agent = "lineth-rollup-testdata/1.0 (block fixture builder for the Lineth monorepo)";
const chain_id: u64 = 59144;
const request_spacing: Io.Duration = .fromMilliseconds(150);
const max_attempts = 6;

const window_first: u64 = 27434569;
const window_last: u64 = 27434718;
const window_blocks: u64 = 150;

const week_start: u64 = 1767225600; // 2026-01-01T00:00:00Z
const week_end: u64 = 1767830400; // 2026-01-08T00:00:00Z
const coarse_stride: u64 = 600;
const coarse_candidates = 6;
const fine_stride: u64 = 20;
const fine_span: u64 = 1500;
const dense_margin: u64 = 400;

const Failure = error{Failed};

fn fail(comptime fmt: []const u8, args: anytype) Failure {
    std.debug.print("fetch_linea_blocks: " ++ fmt ++ "\n", args);
    return error.Failed;
}

pub fn main(init: std.process.Init) u8 {
    run(init) catch |err| {
        if (err != error.Failed) std.debug.print("fetch_linea_blocks: {s}\n", .{@errorName(err)});
        return 1;
    };
    return 0;
}

fn run(init: std.process.Init) !void {
    const args = try init.minimal.args.toSlice(init.arena.allocator());
    const command = if (args.len > 1) args[1] else "fetch";
    var client: std.http.Client = .{ .allocator = init.gpa, .io = init.io };
    defer client.deinit();
    var node: Node = .{ .client = &client, .io = init.io };

    if (std.mem.eql(u8, command, "fetch")) {
        if (args.len != 3) return fail("usage: fetch_linea_blocks fetch TESTDATA_DIR | discover", .{});
        var arena = std.heap.ArenaAllocator.init(init.gpa);
        defer arena.deinit();
        try fetch(arena.allocator(), &node, args[2]);
    } else if (std.mem.eql(u8, command, "discover")) {
        try discover(init.gpa, &node);
    } else {
        return fail("unknown command '{s}' (fetch or discover)", .{command});
    }
}

/// A JSON-RPC endpoint reached over one HTTP client, with the spacing between requests.
const Node = struct {
    client: *std.http.Client,
    io: Io,
    last_request: ?Io.Timestamp = null,

    /// The `result` of `method`, allocated from `a`. A failed attempt is retried with exponential
    /// backoff, an empty result included.
    fn call(node: *Node, a: Allocator, method: []const u8, params: []const u8) !Value {
        const body = try std.fmt.allocPrint(a, "{{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"{s}\",\"params\":{s}}}", .{ method, params });
        var attempt: u6 = 0;
        while (true) : (attempt += 1) {
            return node.once(a, method, params, body) catch |err| {
                if (attempt == max_attempts - 1) return err;
                std.debug.print("retrying {s}: {s}\n", .{ method, @errorName(err) });
                try node.io.sleep(.fromSeconds(@as(i64, 1) << attempt), .awake);
                continue;
            };
        }
    }

    fn once(node: *Node, a: Allocator, method: []const u8, params: []const u8, body: []const u8) !Value {
        if (node.last_request) |last| {
            const wait = request_spacing.nanoseconds - last.durationTo(Io.Timestamp.now(node.io, .awake)).nanoseconds;
            if (wait > 0) try node.io.sleep(.fromNanoseconds(wait), .awake);
        }
        node.last_request = Io.Timestamp.now(node.io, .awake);

        var response: Io.Writer.Allocating = .init(a);
        const result = try node.client.fetch(.{
            .location = .{ .url = rpc_url },
            .payload = body,
            .headers = .{
                .content_type = .{ .override = "application/json" },
                .user_agent = .{ .override = user_agent },
            },
            .response_writer = &response.writer,
        });
        if (result.status != .ok) return fail("{s}{s}: HTTP status {d}", .{ method, params, @intFromEnum(result.status) });
        const reply = try std.json.parseFromSliceLeaky(Value, a, response.written(), .{});
        if (reply != .object) return fail("{s}{s}: reply is not an object", .{ method, params });
        if (reply.object.get("error")) |rpc_error| {
            return fail("{s}: {f}", .{ method, std.json.fmt(rpc_error, .{}) });
        }
        const value = reply.object.get("result") orelse return fail("{s}{s}: empty result", .{ method, params });
        if (value == .null) return fail("{s}{s}: empty result", .{ method, params });
        return value;
    }

    fn blockByNumber(node: *Node, a: Allocator, number: u64, full: bool) !Value {
        const params = try std.fmt.allocPrint(a, "[\"0x{x}\",{}]", .{ number, full });
        return node.call(a, "eth_getBlockByNumber", params);
    }

    fn transactionCount(node: *Node, a: Allocator, number: u64) !u64 {
        const params = try std.fmt.allocPrint(a, "[\"0x{x}\"]", .{number});
        const count = try node.call(a, "eth_getBlockTransactionCountByNumber", params);
        return std.fmt.parseInt(u64, try hexDigits(count), 16);
    }
};

// -- discovery -----------------------------------------------------------------------------------

const Sample = struct {
    count: u64,
    number: u64,

    /// Descending by count, then by number.
    fn after(_: void, lhs: Sample, rhs: Sample) bool {
        if (lhs.count != rhs.count) return lhs.count > rhs.count;
        return lhs.number > rhs.number;
    }
};

fn firstBlockAtOrAfter(node: *Node, scratch: *std.heap.ArenaAllocator, timestamp: u64, start: u64, end: u64) !u64 {
    var low = start;
    var high = end;
    while (low < high) {
        const mid = (low + high) / 2;
        _ = scratch.reset(.retain_capacity);
        const header = try node.blockByNumber(scratch.allocator(), mid, false);
        if (try quantityOf(header, "timestamp") < timestamp) {
            low = mid + 1;
        } else {
            high = mid;
        }
    }
    return low;
}

fn sampleCount(node: *Node, scratch: *std.heap.ArenaAllocator, number: u64) !u64 {
    _ = scratch.reset(.retain_capacity);
    return node.transactionCount(scratch.allocator(), number);
}

fn discover(gpa: Allocator, node: *Node) !void {
    var scratch = std.heap.ArenaAllocator.init(gpa);
    defer scratch.deinit();
    var stdout_buffer: [1024]u8 = undefined;
    var stdout = Io.File.stdout().writer(node.io, &stdout_buffer);
    const out = &stdout.interface;

    const latest_value = try node.call(scratch.allocator(), "eth_blockNumber", "[]");
    const latest = try std.fmt.parseInt(u64, try hexDigits(latest_value), 16);
    const first = try firstBlockAtOrAfter(node, &scratch, week_start, 0, latest);
    const last = try firstBlockAtOrAfter(node, &scratch, week_end, first, latest) - 1;
    try out.print("week: blocks {d}..{d}\n", .{ first, last });
    try out.flush();

    var coarse: std.ArrayList(Sample) = .empty;
    defer coarse.deinit(gpa);
    var number = first;
    while (number <= last) : (number += coarse_stride) {
        try coarse.append(gpa, .{ .count = try sampleCount(node, &scratch, number), .number = number });
    }
    std.mem.sort(Sample, coarse.items, {}, Sample.after);
    const best = coarse.items[0..@min(coarse_candidates, coarse.items.len)];
    try out.writeAll("coarse top: [");
    for (best, 0..) |sample, i| {
        try out.print("{s}({d}, {d})", .{ if (i == 0) "" else ", ", sample.count, sample.number });
    }
    try out.writeAll("]\n");
    try out.flush();

    var peak: ?Sample = null;
    for (best) |center| {
        var n = center.number -| fine_span;
        if (n < first) n = first;
        const stop = @min(last, center.number + fine_span);
        while (n <= stop) : (n += fine_stride) {
            const sample: Sample = .{ .count = try sampleCount(node, &scratch, n), .number = n };
            if (peak == null or Sample.after({}, sample, peak.?)) peak = sample;
        }
    }
    const peak_number = (peak orelse return fail("no sample in the week", .{})).number;

    const low = @max(first, peak_number -| dense_margin);
    const high = @min(last, peak_number + dense_margin);
    const counts = try gpa.alloc(u64, high - low + 1);
    defer gpa.free(counts);
    for (counts, low..) |*count, n| count.* = try sampleCount(node, &scratch, n);

    var best_sum: u64 = 0;
    var best_start: u64 = low;
    var start = low;
    while (start + window_blocks - 1 <= high) : (start += 1) {
        var sum: u64 = 0;
        for (counts[start - low ..][0..window_blocks]) |count| sum += count;
        if (sum >= best_sum) {
            best_sum = sum;
            best_start = start;
        }
    }
    try out.print("densest window: WINDOW_FIRST = {d}, WINDOW_LAST = {d}, {d} transactions\n", .{ best_start, best_start + window_blocks - 1, best_sum });
    try out.flush();
}

// -- RLP -----------------------------------------------------------------------------------------

/// Encoders for the RLP items the blocks need, each returning the full encoding of one item. The
/// output of `header` is sized for the whole item, its payload left for the caller to fill.
const rlp = struct {
    fn header(a: Allocator, offset: u8, len: usize) ![]u8 {
        if (len < 56) {
            const out = try a.alloc(u8, 1 + len);
            out[0] = offset + @as(u8, @intCast(len));
            return out;
        }
        var len_bytes: [8]u8 = undefined;
        std.mem.writeInt(u64, &len_bytes, len, .big);
        const skip = std.mem.indexOfNone(u8, &len_bytes, &.{0}).?;
        const digits = len_bytes[skip..];
        const out = try a.alloc(u8, 1 + digits.len + len);
        out[0] = offset + 55 + @as(u8, @intCast(digits.len));
        @memcpy(out[1..][0..digits.len], digits);
        return out;
    }

    fn string(a: Allocator, data: []const u8) ![]u8 {
        if (data.len == 1 and data[0] < 0x80) return a.dupe(u8, data);
        const out = try header(a, 0x80, data.len);
        @memcpy(out[headerLen(data.len)..], data);
        return out;
    }

    fn list(a: Allocator, items: []const []const u8) ![]u8 {
        var payload: usize = 0;
        for (items) |item| payload += item.len;
        const out = try header(a, 0xc0, payload);
        var rest = out[headerLen(payload)..];
        for (items) |item| {
            @memcpy(rest[0..item.len], item);
            rest = rest[item.len..];
        }
        return out;
    }

    fn headerLen(len: usize) usize {
        if (len < 56) return 1;
        var digits: usize = 0;
        var rest = len;
        while (rest != 0) : (rest >>= 8) digits += 1;
        return 1 + digits;
    }
};

// -- JSON values of the RPC ----------------------------------------------------------------------

fn field(object: Value, name: []const u8) !Value {
    if (object != .object) return fail("expected an object around field {s}", .{name});
    return object.object.get(name) orelse fail("missing field {s}", .{name});
}

fn text(value: Value) ![]const u8 {
    if (value != .string) return fail("expected a string, found {s}", .{@tagName(value)});
    return value.string;
}

/// The hex digits of the 0x-prefixed string `value`.
fn hexDigits(value: Value) ![]const u8 {
    const s = try text(value);
    if (!std.mem.startsWith(u8, s, "0x")) return fail("'{s}' is not 0x-prefixed", .{s});
    return s[2..];
}

fn quantityOf(object: Value, name: []const u8) !u64 {
    return std.fmt.parseInt(u64, try hexDigits(try field(object, name)), 16) catch fail("field {s} is not a 64-bit quantity", .{name});
}

/// The bytes of a 0x-prefixed hex string.
fn bytesOf(a: Allocator, hex: []const u8) ![]u8 {
    if (!std.mem.startsWith(u8, hex, "0x") or hex.len % 2 != 0) return fail("'{s}' is not 0x-prefixed hex bytes", .{hex});
    const out = try a.alloc(u8, (hex.len - 2) / 2);
    _ = std.fmt.hexToBytes(out, hex[2..]) catch return fail("'{s}' is not hex", .{hex});
    return out;
}

/// The minimal big-endian bytes of the quantity `hex`, empty for zero.
fn integerBytes(a: Allocator, hex: []const u8) ![]u8 {
    if (!std.mem.startsWith(u8, hex, "0x")) return fail("'{s}' is not 0x-prefixed", .{hex});
    const digits = std.mem.trimStart(u8, hex[2..], "0");
    const padded = try std.fmt.allocPrint(a, "{s}{s}", .{ if (digits.len % 2 == 1) "0" else "", digits });
    const out = try a.alloc(u8, padded.len / 2);
    _ = std.fmt.hexToBytes(out, padded) catch return fail("'{s}' is not hex", .{hex});
    return out;
}

/// The lowercase hex of the integer `hex`, left-padded with one 0 when its digit count is odd.
fn quantity(a: Allocator, hex: []const u8) ![]u8 {
    const bytes = try integerBytes(a, hex);
    if (bytes.len == 0) return a.dupe(u8, "0x00");
    return std.fmt.allocPrint(a, "0x{x}", .{bytes});
}

fn quantityNumber(a: Allocator, n: u64) ![]u8 {
    return quantity(a, try std.fmt.allocPrint(a, "0x{x}", .{n}));
}

/// The RLP string of the bytes of the 0x-prefixed hex field `name` of `object`.
fn rawItem(a: Allocator, object: Value, name: []const u8) ![]u8 {
    return rlp.string(a, try bytesOf(a, try text(try field(object, name))));
}

fn integerItem(a: Allocator, object: Value, name: []const u8) ![]u8 {
    return rlp.string(a, try integerBytes(a, try text(try field(object, name))));
}

fn hashOf(data: []const u8) [32]u8 {
    return zkvm_accelerators.keccak256Digest(data);
}

fn hexHash(a: Allocator, hash: [32]u8) ![]u8 {
    return std.fmt.allocPrint(a, "0x{x}", .{&hash});
}

// -- re-encoding ---------------------------------------------------------------------------------

/// The fields of the Prague header the RPC reports, in header order, as RLP items.
fn headerItems(a: Allocator, block: Value) ![][]u8 {
    const items = try a.alloc([]u8, 21);
    const raw_fields = [_]struct { usize, []const u8 }{
        .{ 0, "parentHash" },       .{ 1, "sha3Uncles" },   .{ 2, "miner" },            .{ 3, "stateRoot" },
        .{ 4, "transactionsRoot" }, .{ 5, "receiptsRoot" }, .{ 6, "logsBloom" },        .{ 12, "extraData" },
        .{ 13, "mixHash" },         .{ 14, "nonce" },       .{ 16, "withdrawalsRoot" }, .{ 19, "parentBeaconBlockRoot" },
        .{ 20, "requestsHash" },
    };
    for (raw_fields) |entry| items[entry[0]] = try rawItem(a, block, entry[1]);
    const integer_fields = [_]struct { usize, []const u8 }{
        .{ 7, "difficulty" }, .{ 8, "number" },         .{ 9, "gasLimit" },     .{ 10, "gasUsed" },
        .{ 11, "timestamp" }, .{ 15, "baseFeePerGas" }, .{ 17, "blobGasUsed" }, .{ 18, "excessBlobGas" },
    };
    for (integer_fields) |entry| items[entry[0]] = try integerItem(a, block, entry[1]);
    return items;
}

const Transaction = struct {
    /// The EIP-2718 wire encoding: the RLP list of a legacy transaction, the type byte and the
    /// RLP list of a typed one.
    encoding: []u8,
    /// The item of the block's transaction list: the legacy list inlined, a typed encoding as a
    /// byte string.
    item: []u8,
    kind: u8,
};

fn accessList(a: Allocator, tx: Value) ![]u8 {
    const entries = try field(tx, "accessList");
    if (entries != .array) return fail("accessList is not an array", .{});
    const items = try a.alloc([]u8, entries.array.items.len);
    for (entries.array.items, items) |entry, *item| {
        const keys_value = try field(entry, "storageKeys");
        if (keys_value != .array) return fail("storageKeys is not an array", .{});
        const keys = try a.alloc([]u8, keys_value.array.items.len);
        for (keys_value.array.items, keys) |key, *slot| slot.* = try rlp.string(a, try bytesOf(a, try text(key)));
        const pair = [_][]const u8{ try rawItem(a, entry, "address"), try rlp.list(a, keys) };
        item.* = try rlp.list(a, &pair);
    }
    return rlp.list(a, items);
}

fn authorizationList(a: Allocator, tx: Value) ![]u8 {
    const entries = try field(tx, "authorizationList");
    if (entries != .array) return fail("authorizationList is not an array", .{});
    const items = try a.alloc([]u8, entries.array.items.len);
    for (entries.array.items, items) |entry, *item| {
        const parts = [_][]const u8{
            try integerItem(a, entry, "chainId"),
            try rawItem(a, entry, "address"),
            try integerItem(a, entry, "nonce"),
            try integerItem(a, entry, "yParity"),
            try integerItem(a, entry, "r"),
            try integerItem(a, entry, "s"),
        };
        item.* = try rlp.list(a, &parts);
    }
    return rlp.list(a, items);
}

/// `to` of a transaction: the address, or the empty string of a contract creation.
fn recipient(a: Allocator, tx: Value) ![]u8 {
    const to = try field(tx, "to");
    if (to == .null) return rlp.string(a, "");
    return rlp.string(a, try bytesOf(a, try text(to)));
}

fn buildTransaction(a: Allocator, tx: Value) !Transaction {
    const kind = try quantityOf(tx, "type");
    var parts: std.ArrayList([]const u8) = .empty;
    if (kind != 0) try parts.append(a, try integerItem(a, tx, "chainId"));
    try parts.append(a, try integerItem(a, tx, "nonce"));
    switch (kind) {
        0, 1 => try parts.append(a, try integerItem(a, tx, "gasPrice")),
        2, 4 => {
            try parts.append(a, try integerItem(a, tx, "maxPriorityFeePerGas"));
            try parts.append(a, try integerItem(a, tx, "maxFeePerGas"));
        },
        else => return fail("unsupported transaction type {d}", .{kind}),
    }
    try parts.append(a, try integerItem(a, tx, "gas"));
    try parts.append(a, try recipient(a, tx));
    try parts.append(a, try integerItem(a, tx, "value"));
    try parts.append(a, try rlp.string(a, try bytesOf(a, try text(try field(tx, "input")))));
    if (kind != 0) try parts.append(a, try accessList(a, tx));
    if (kind == 4) try parts.append(a, try authorizationList(a, tx));
    try parts.append(a, try integerItem(a, tx, if (kind == 0) "v" else "yParity"));
    try parts.append(a, try integerItem(a, tx, "r"));
    try parts.append(a, try integerItem(a, tx, "s"));

    const body = try rlp.list(a, parts.items);
    if (kind == 0) return .{ .encoding = body, .item = body, .kind = 0 };
    const encoding = try std.mem.concat(a, u8, &.{ &.{@as(u8, @intCast(kind))}, body });
    return .{ .encoding = encoding, .item = try rlp.string(a, encoding), .kind = @intCast(kind) };
}

fn withdrawalItems(a: Allocator, block: Value) ![]u8 {
    const withdrawals = try field(block, "withdrawals");
    if (withdrawals != .array) return fail("withdrawals is not an array", .{});
    const items = try a.alloc([]u8, withdrawals.array.items.len);
    for (withdrawals.array.items, items) |withdrawal, *item| {
        const parts = [_][]const u8{
            try integerItem(a, withdrawal, "index"),
            try integerItem(a, withdrawal, "validatorIndex"),
            try rawItem(a, withdrawal, "address"),
            try integerItem(a, withdrawal, "amount"),
        };
        item.* = try rlp.list(a, &parts);
    }
    return rlp.list(a, items);
}

const Reshaped = struct {
    rlp: []u8,
    hash: [32]u8,
    parent_hash: [32]u8,
};

/// The Amsterdam-shaped block of the RPC block `block` descending from `parent_hash`, after the
/// checks of the RPC's own hashes. `types` counts the block's transactions by type.
fn rebuild(a: Allocator, block: Value, parent_hash: [32]u8, types: *[5]u64) !Reshaped {
    const number = try quantityOf(block, "number");
    const items = try headerItems(a, block);
    const reported_hash = try bytesOf(a, try text(try field(block, "hash")));
    const prague_hash = hashOf(try rlp.list(a, items));
    if (!std.mem.eql(u8, &prague_hash, reported_hash)) return fail("block {d}: header hash mismatch", .{number});

    const source = try field(block, "transactions");
    if (source != .array) return fail("block {d}: transactions is not an array", .{number});
    const transactions = try a.alloc([]u8, source.array.items.len);
    for (source.array.items, transactions, 0..) |tx, *item, index| {
        const built = buildTransaction(a, tx) catch |err| switch (err) {
            error.Failed => return fail("block {d}: transaction {d} cannot be rebuilt", .{ number, index }),
            else => return err,
        };
        const reported_tx_hash = try bytesOf(a, try text(try field(tx, "hash")));
        if (!std.mem.eql(u8, &hashOf(built.encoding), reported_tx_hash)) return fail("block {d}: transaction {d} hash mismatch", .{ number, index });
        types[built.kind] += 1;
        item.* = built.item;
    }
    const uncles = try field(block, "uncles");
    if (uncles != .array or uncles.array.items.len != 0) return fail("block {d}: has ommers", .{number});

    const empty_list = try rlp.list(a, &.{});
    const empty_block_access_list_hash = hashOf(empty_list);
    items[0] = try rlp.string(a, &parent_hash);
    const reshaped_items = try std.mem.concat(a, []u8, &.{
        items,
        &.{ try rlp.string(a, &empty_block_access_list_hash), try rlp.string(a, "") },
    });
    const header = try rlp.list(a, reshaped_items);
    const body = [_][]const u8{ header, try rlp.list(a, transactions), empty_list, try withdrawalItems(a, block) };
    return .{ .rlp = try rlp.list(a, &body), .hash = hashOf(header), .parent_hash = parent_hash };
}

// -- JSON in the EF shape ------------------------------------------------------------------------

fn writeTransaction(a: Allocator, json: *std.json.Stringify, tx: Value) !void {
    const kind = try quantityOf(tx, "type");
    const typed = kind >= 1;
    try json.beginObject();
    try json.objectField("type");
    try json.write(try quantityNumber(a, kind));
    try json.objectField("chainId");
    try json.write(if (tx.object.get("chainId")) |id| try quantity(a, try text(id)) else try quantityNumber(a, chain_id));
    try json.objectField("nonce");
    try json.write(try quantity(a, try text(try field(tx, "nonce"))));
    if (kind <= 1) {
        try json.objectField("gasPrice");
        try json.write(try quantity(a, try text(try field(tx, "gasPrice"))));
    } else {
        try json.objectField("maxPriorityFeePerGas");
        try json.write(try quantity(a, try text(try field(tx, "maxPriorityFeePerGas"))));
        try json.objectField("maxFeePerGas");
        try json.write(try quantity(a, try text(try field(tx, "maxFeePerGas"))));
    }
    try json.objectField("gasLimit");
    try json.write(try quantity(a, try text(try field(tx, "gas"))));
    try json.objectField("to");
    const to = try field(tx, "to");
    try json.write(if (to == .null) "" else try text(to));
    try json.objectField("value");
    try json.write(try quantity(a, try text(try field(tx, "value"))));
    try json.objectField("data");
    try json.write(try text(try field(tx, "input")));
    if (typed) {
        try json.objectField("accessList");
        try json.beginArray();
        for ((try field(tx, "accessList")).array.items) |entry| {
            try json.beginObject();
            try json.objectField("address");
            try json.write(try text(try field(entry, "address")));
            try json.objectField("storageKeys");
            try json.write((try field(entry, "storageKeys")).array.items);
            try json.endObject();
        }
        try json.endArray();
    }
    if (kind == 4) {
        try json.objectField("authorizationList");
        try json.beginArray();
        for ((try field(tx, "authorizationList")).array.items) |entry| {
            try json.beginObject();
            inline for (.{ "chainId", "address", "nonce", "yParity", "r", "s" }) |name| {
                try json.objectField(name);
                const raw = try text(try field(entry, name));
                const quantified = comptime std.mem.eql(u8, name, "chainId") or std.mem.eql(u8, name, "nonce") or std.mem.eql(u8, name, "yParity");
                try json.write(if (quantified) try quantity(a, raw) else raw);
            }
            try json.endObject();
        }
        try json.endArray();
    }
    try json.objectField("v");
    try json.write(try quantity(a, try text(try field(tx, "v"))));
    try json.objectField("r");
    try json.write(try text(try field(tx, "r")));
    try json.objectField("s");
    try json.write(try text(try field(tx, "s")));
    try json.objectField("sender");
    try json.write(try text(try field(tx, "from")));
    try json.endObject();
}

fn writeBlock(a: Allocator, json: *std.json.Stringify, block: Value, shaped: Reshaped) !void {
    try json.beginObject();
    try json.objectField("rlp");
    try json.write(try std.fmt.allocPrint(a, "0x{x}", .{shaped.rlp}));
    try json.objectField("blockHeader");
    try json.beginObject();
    try json.objectField("hash");
    try json.write(try hexHash(a, shaped.hash));
    try json.objectField("parentHash");
    try json.write(try hexHash(a, shaped.parent_hash));
    try json.objectField("mixHash");
    try json.write(try text(try field(block, "mixHash")));
    try json.objectField("number");
    try json.write(try quantity(a, try text(try field(block, "number"))));
    try json.objectField("timestamp");
    try json.write(try quantity(a, try text(try field(block, "timestamp"))));
    try json.endObject();
    try json.objectField("transactions");
    try json.beginArray();
    for ((try field(block, "transactions")).array.items) |tx| try writeTransaction(a, json, tx);
    try json.endArray();
    try json.endObject();
}

// -- fetch ---------------------------------------------------------------------------------------

/// The truncation of the reshaped block has to recover the senders the RPC reports.
fn checkTruncation(a: Allocator, block: Value, shaped: Reshaped) !void {
    const number = try quantityOf(block, "number");
    const truncated = truncation.truncateBlockRlp(a, a, shaped.rlp, chain_id) catch |err| {
        return fail("block {d}: truncation rejects the reshaped block: {s}", .{ number, @errorName(err) });
    };
    const transactions = (try field(block, "transactions")).array.items;
    if (truncated.froms.len != transactions.len) return fail("block {d}: truncation finds {d} transactions, the RPC {d}", .{ number, truncated.froms.len, transactions.len });
    for (truncated.froms, transactions, 0..) |from, tx, index| {
        const reported = try bytesOf(a, try text(try field(tx, "from")));
        if (!std.mem.eql(u8, &from, reported)) return fail("block {d}: transaction {d} sender differs from the RPC's", .{ number, index });
    }
}

fn fetch(a: Allocator, node: *Node, testdata: []const u8) !void {
    const name = try std.fmt.allocPrint(a, "linea_mainnet_{d}_{d}", .{ window_first, window_last });
    var json_buffer: Io.Writer.Allocating = .init(a);
    var json: std.json.Stringify = .{ .writer = &json_buffer.writer };
    try json.beginObject();
    try json.objectField(name);
    try json.beginObject();
    try json.objectField("config");
    try json.beginObject();
    try json.objectField("chainid");
    try json.write(try quantityNumber(a, chain_id));
    try json.endObject();
    try json.objectField("blocks");
    try json.beginArray();

    var types: [5]u64 = @splat(0);
    var block_count: u64 = 0;
    var rlp_total: usize = 0;
    var parent_hash: [32]u8 = undefined;
    var number = window_first;
    while (number <= window_last) : (number += 1) {
        const block = try node.blockByNumber(a, number, true);
        if (number == window_first) parent_hash = (try bytesOf(a, try text(try field(block, "parentHash"))))[0..32].*;
        const shaped = try rebuild(a, block, parent_hash, &types);
        try checkTruncation(a, block, shaped);
        try writeBlock(a, &json, block, shaped);
        parent_hash = shaped.hash;
        rlp_total += shaped.rlp.len;
        block_count += 1;
    }
    try json.endArray();
    try json.endObject();
    try json.endObject();

    var compressed: Io.Writer.Allocating = try .initCapacity(a, 4096);
    const window = try a.alloc(u8, std.compress.flate.max_window_len);
    var compressor = try std.compress.flate.Compress.init(&compressed.writer, window, .gzip, .best);
    try compressor.writer.writeAll(json_buffer.written());
    try compressor.finish();

    const cwd = Io.Dir.cwd();
    try cwd.createDirPath(node.io, testdata);
    const path = try std.fmt.allocPrint(a, "{s}/{s}.json.gz", .{ testdata, name });
    try cwd.writeFile(node.io, .{ .sub_path = path, .data = compressed.written() });

    var stdout_buffer: [1024]u8 = undefined;
    var stdout = Io.File.stdout().writer(node.io, &stdout_buffer);
    const out = &stdout.interface;
    var total: u64 = 0;
    for (types) |count| total += count;
    try out.print("{s}\nblocks {d}, transactions {d}, type histogram {{", .{ path, block_count, total });
    var separator: []const u8 = "";
    for (types, 0..) |count, kind| {
        if (count == 0) continue;
        try out.print("{s}{d}: {d}", .{ separator, kind, count });
        separator = ", ";
    }
    try out.print("}}\nblock RLP total {d} bytes, file {d} bytes\n", .{ rlp_total, compressed.written().len });
    try out.flush();
}

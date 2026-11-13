//! Builds a Linea mainnet block dataset in the shape of the EF blockchain tests.
//!
//! `fetch_linea_blocks TESTDATA_DIR` downloads the blocks
//! `window_first..window_last` from the public RPC with full transaction objects, rebuilds every
//! header and transaction as RLP and checks each against the hash the RPC reports, and checks that
//! the truncation of each rebuilt block recovers the senders the RPC reports.
//! `TESTDATA_DIR/linea_mainnet_<first>_<last>.json.gz` holds one case with `config.chainid`
//! 0xe708 and one entry per block with `rlp`, `blockHeader` (hash, parentHash, mixHash, number,
//! timestamp) and `transactions`. The headers and the hashes of the file are mainnet's. The gzip
//! stream carries no filename and a zero mtime, so a rerun reproduces the file byte for byte.

const std = @import("std");
const lineth_da = @import("lineth_da");
const zkvm_accelerators = @import("zkvm_accelerators");

const Allocator = std.mem.Allocator;
const Io = std.Io;
const Value = std.json.Value;
const truncation = lineth_da.encoding.truncation;
const rlp = @import("zesu_executor").executor_rlp_encode;

const rpc_url = "https://rpc.linea.build";
const user_agent = "lineth-rollup-testdata/1.0 (block fixture builder for the Lineth monorepo)";
const chain_id: u64 = 59144;
const request_spacing: Io.Duration = .fromMilliseconds(150);
const max_attempts = 6;

const window_first: u64 = 27434569;
const window_last: u64 = 27434718;
const window_blocks: u64 = 150;

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
    if (args.len != 2) return fail("usage: fetch_linea_blocks TESTDATA_DIR", .{});
    var client: std.http.Client = .{ .allocator = init.gpa, .io = init.io };
    defer client.deinit();
    var node: Node = .{ .client = &client, .io = init.io };
    var arena = std.heap.ArenaAllocator.init(init.gpa);
    defer arena.deinit();
    try fetch(arena.allocator(), &node, args[1]);
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
    if (!std.mem.startsWith(u8, hex, "0x")) return fail("'{s}' is not 0x-prefixed hex bytes", .{hex});
    const out = try a.alloc(u8, (hex.len - 2) / 2);
    _ = std.fmt.hexToBytes(out, hex[2..]) catch return fail("'{s}' is not hex bytes", .{hex});
    return out;
}

/// The value of the 0x-prefixed quantity `hex`.
fn integerOf(hex: []const u8) !u256 {
    if (!std.mem.startsWith(u8, hex, "0x")) return fail("'{s}' is not 0x-prefixed", .{hex});
    if (hex.len == 2) return 0;
    return std.fmt.parseInt(u256, hex[2..], 16) catch fail("'{s}' is not a 256-bit quantity", .{hex});
}

/// The lowercase hex of the integer `hex`, left-padded with one 0 when its digit count is odd.
fn quantity(a: Allocator, hex: []const u8) ![]u8 {
    const digits = try std.fmt.allocPrint(a, "{x}", .{try integerOf(hex)});
    return std.fmt.allocPrint(a, "0x{s}{s}", .{ if (digits.len % 2 == 1) "0" else "", digits });
}

fn quantityNumber(a: Allocator, n: u64) ![]u8 {
    return quantity(a, try std.fmt.allocPrint(a, "0x{x}", .{n}));
}

/// The RLP string of the bytes of the 0x-prefixed hex field `name` of `object`.
fn rawItem(a: Allocator, object: Value, name: []const u8) ![]u8 {
    return rlp.encodeBytes(a, try bytesOf(a, try text(try field(object, name))));
}

fn integerItem(a: Allocator, object: Value, name: []const u8) ![]u8 {
    return rlp.encodeU256(a, try integerOf(try text(try field(object, name))));
}

fn hashOf(data: []const u8) [32]u8 {
    return zkvm_accelerators.keccak256Digest(data);
}

fn hexHash(a: Allocator, hash: [32]u8) ![]u8 {
    return std.fmt.allocPrint(a, "0x{x}", .{&hash});
}

// -- re-encoding ---------------------------------------------------------------------------------

/// The fields of the header the RPC reports, in header order, as RLP items.
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
        for (keys_value.array.items, keys) |key, *slot| slot.* = try rlp.encodeBytes(a, try bytesOf(a, try text(key)));
        const pair = [_][]const u8{ try rawItem(a, entry, "address"), try rlp.encodeList(a, keys) };
        item.* = try rlp.encodeList(a, &pair);
    }
    return rlp.encodeList(a, items);
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
        item.* = try rlp.encodeList(a, &parts);
    }
    return rlp.encodeList(a, items);
}

/// `to` of a transaction: the address, or the empty string of a contract creation.
fn recipient(a: Allocator, tx: Value) ![]u8 {
    const to = try field(tx, "to");
    if (to == .null) return rlp.encodeBytes(a, "");
    return rlp.encodeBytes(a, try bytesOf(a, try text(to)));
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
    try parts.append(a, try rlp.encodeBytes(a, try bytesOf(a, try text(try field(tx, "input")))));
    if (kind != 0) try parts.append(a, try accessList(a, tx));
    if (kind == 4) try parts.append(a, try authorizationList(a, tx));
    try parts.append(a, try integerItem(a, tx, if (kind == 0) "v" else "yParity"));
    try parts.append(a, try integerItem(a, tx, "r"));
    try parts.append(a, try integerItem(a, tx, "s"));

    const body = try rlp.encodeList(a, parts.items);
    if (kind == 0) return .{ .encoding = body, .item = body, .kind = 0 };
    const encoding = try std.mem.concat(a, u8, &.{ &.{@as(u8, @intCast(kind))}, body });
    return .{ .encoding = encoding, .item = try rlp.encodeBytes(a, encoding), .kind = @intCast(kind) };
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
        item.* = try rlp.encodeList(a, &parts);
    }
    return rlp.encodeList(a, items);
}

const Rebuilt = struct {
    rlp: []u8,
    hash: [32]u8,
    parent_hash: [32]u8,
};

/// The RLP of the RPC block `block`, after the checks of the RPC's own hashes. `types` counts the
/// block's transactions by type.
fn rebuild(a: Allocator, block: Value, types: *[5]u64) !Rebuilt {
    const number = try quantityOf(block, "number");
    const items = try headerItems(a, block);
    const reported_hash = try bytesOf(a, try text(try field(block, "hash")));
    const header = try rlp.encodeList(a, items);
    const header_hash = hashOf(header);
    if (!std.mem.eql(u8, &header_hash, reported_hash)) return fail("block {d}: header hash mismatch", .{number});

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

    const body = [_][]const u8{ header, try rlp.encodeList(a, transactions), try rlp.encodeList(a, &.{}), try withdrawalItems(a, block) };
    return .{
        .rlp = try rlp.encodeList(a, &body),
        .hash = header_hash,
        .parent_hash = (try bytesOf(a, try text(try field(block, "parentHash"))))[0..32].*,
    };
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

fn writeBlock(a: Allocator, json: *std.json.Stringify, block: Value, built: Rebuilt) !void {
    try json.beginObject();
    try json.objectField("rlp");
    try json.write(try std.fmt.allocPrint(a, "0x{x}", .{built.rlp}));
    try json.objectField("blockHeader");
    try json.beginObject();
    try json.objectField("hash");
    try json.write(try hexHash(a, built.hash));
    try json.objectField("parentHash");
    try json.write(try hexHash(a, built.parent_hash));
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

/// The truncation of the rebuilt block has to recover the senders the RPC reports.
fn checkTruncation(a: Allocator, block: Value, built: Rebuilt) !void {
    const number = try quantityOf(block, "number");
    const truncated = truncation.truncateBlockRlp(a, a, built.rlp, chain_id) catch |err| {
        return fail("block {d}: truncation rejects the block: {s}", .{ number, @errorName(err) });
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
    var number = window_first;
    while (number <= window_last) : (number += 1) {
        const block = try node.blockByNumber(a, number, true);
        const built = try rebuild(a, block, &types);
        try checkTruncation(a, block, built);
        try writeBlock(a, &json, block, built);
        rlp_total += built.rlp.len;
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

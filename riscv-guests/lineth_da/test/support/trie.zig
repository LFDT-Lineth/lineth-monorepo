//! The root of a Merkle Patricia trie over a list, key `RLP(index)`: the transactions trie of a
//! block. Written without the package under test, as the textbook recursion over sorted keys.

const std = @import("std");

const rlp = @import("rlp.zig");

const Allocator = std.mem.Allocator;
const Item = rlp.Item;

pub const Hash = [32]u8;

fn keccak(bytes: []const u8) Hash {
    var hash: Hash = undefined;
    std.crypto.hash.sha3.Keccak256.hash(bytes, &hash, .{});
    return hash;
}

/// The root of the empty trie: the hash of the empty string's RLP.
pub const empty_root: Hash = .{
    0x56, 0xe8, 0x1f, 0x17, 0x1b, 0xcc, 0x55, 0xa6, 0xff, 0x83, 0x45, 0xe6, 0x92, 0xc0, 0xf8, 0x6e,
    0x5b, 0x48, 0xe0, 0x1b, 0x99, 0x6c, 0xad, 0xc0, 0x01, 0x62, 0x2f, 0xb5, 0xe3, 0x63, 0xb4, 0x21,
};

const Entry = struct {
    nibbles: []const u8,
    value: []const u8,

    fn lessThan(_: void, a: Entry, b: Entry) bool {
        return std.mem.lessThan(u8, a.nibbles, b.nibbles);
    }
};

/// The trie root over `values`, the value at position `i` under the key `RLP(i)`.
pub fn listRoot(allocator: Allocator, values: []const []const u8) Allocator.Error!Hash {
    if (values.len == 0) return empty_root;
    const entries = try allocator.alloc(Entry, values.len);
    for (values, entries, 0..) |value, *entry, index| {
        const key = try rlp.encode(allocator, try rlp.int(allocator, index));
        const nibbles = try allocator.alloc(u8, key.len * 2);
        for (key, 0..) |byte, i| {
            nibbles[2 * i] = byte >> 4;
            nibbles[2 * i + 1] = byte & 0x0f;
        }
        entry.* = .{ .nibbles = nibbles, .value = value };
    }
    std.mem.sort(Entry, entries, {}, Entry.lessThan);
    return keccak(try rlp.encode(allocator, try node(allocator, entries, 0)));
}

/// The node holding `entries`, which agree on their first `depth` nibbles. RLP keys are prefix-free,
/// so no entry ends before the others do.
fn node(allocator: Allocator, entries: []const Entry, depth: usize) Allocator.Error!Item {
    if (entries.len == 1) {
        const path = try compact(allocator, entries[0].nibbles[depth..], true);
        return rlp.list(try items(allocator, .{ rlp.string(path), rlp.string(entries[0].value) }));
    }
    var shared: usize = 0;
    while (agreeAt(entries, depth + shared)) shared += 1;
    if (shared > 0) {
        const path = try compact(allocator, entries[0].nibbles[depth..][0..shared], false);
        return rlp.list(try items(allocator, .{ rlp.string(path), try reference(allocator, try node(allocator, entries, depth + shared)) }));
    }
    const slots = try allocator.alloc(Item, 17);
    var start: usize = 0;
    for (slots[0..16], 0..) |*slot, nibble| {
        var end = start;
        while (end < entries.len and entries[end].nibbles[depth] == nibble) end += 1;
        slot.* = if (end == start) rlp.string(&.{}) else try reference(allocator, try node(allocator, entries[start..end], depth + 1));
        start = end;
    }
    slots[16] = rlp.string(&.{});
    return rlp.list(slots);
}

fn agreeAt(entries: []const Entry, position: usize) bool {
    for (entries) |entry| if (entry.nibbles[position] != entries[0].nibbles[position]) return false;
    return true;
}

fn items(allocator: Allocator, values: [2]Item) Allocator.Error![]Item {
    return allocator.dupe(Item, &values);
}

/// How a parent refers to a child node: the node itself when its encoding is under 32 bytes, else its hash.
fn reference(allocator: Allocator, child: Item) Allocator.Error!Item {
    const encoded = try rlp.encode(allocator, child);
    if (encoded.len < 32) return rlp.raw(encoded);
    return rlp.string(try allocator.dupe(u8, &keccak(encoded)));
}

/// The hex-prefix encoding of a nibble path.
fn compact(allocator: Allocator, nibbles: []const u8, is_leaf: bool) Allocator.Error![]u8 {
    const flag: u8 = if (is_leaf) 2 else 0;
    const odd = nibbles.len % 2 == 1;
    const out = try allocator.alloc(u8, nibbles.len / 2 + 1);
    var rest = nibbles;
    if (odd) {
        out[0] = (flag + 1) << 4 | nibbles[0];
        rest = nibbles[1..];
    } else {
        out[0] = flag << 4;
    }
    for (out[1..], 0..) |*byte, i| byte.* = rest[2 * i] << 4 | rest[2 * i + 1];
    return out;
}

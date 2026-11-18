const std = @import("std");

const lineth_da = @import("lineth_da");

const rolling_hash = lineth_da.encoding.rolling_hash;

const Hash = rolling_hash.Hash;

fn keccak(preimage: []const u8) Hash {
    var hash: Hash = undefined;
    std.crypto.hash.sha3.Keccak256.hash(preimage, &hash, .{});
    return hash;
}

fn step(parent: Hash, chunk_hash: Hash) Hash {
    return keccak(&(parent ++ chunk_hash));
}

fn filled(byte: u8) Hash {
    return @splat(byte);
}

test "the fold step hashes the parent and the chunk hash" {
    // keccak256 of 64 zero bytes, the first level of Ethereum's zero-hash tree.
    const zero_pair: Hash = .{
        0xad, 0x32, 0x28, 0xb6, 0x76, 0xf7, 0xd3, 0xcd, 0x42, 0x84, 0xa5, 0x44, 0x3f, 0x17, 0xf1, 0x96,
        0x2b, 0x36, 0xe4, 0x91, 0xb3, 0x0a, 0x40, 0xb2, 0x40, 0x58, 0x49, 0xe5, 0x97, 0xba, 0x5f, 0xb5,
    };
    try std.testing.expectEqualSlices(u8, &zero_pair, &rolling_hash.next(filled(0), filled(0)));
    try std.testing.expect(!std.mem.eql(u8, &rolling_hash.next(filled(1), filled(2)), &rolling_hash.next(filled(2), filled(1))));
}

test "a range starting at a chunk boundary folds every chunk onto the parent hash" {
    const parent = filled(0x10);
    const chunks = [_]Hash{ filled(0xa1), filled(0xa2), filled(0xa3) };
    const expected = step(step(step(parent, chunks[0]), chunks[1]), chunks[2]);
    try std.testing.expectEqualSlices(u8, &expected, &(try rolling_hash.fold(parent, &chunks, 0, null)));
}

test "a range without chunks keeps the parent hash" {
    const parent = filled(0x10);
    try std.testing.expectEqualSlices(u8, &parent, &(try rolling_hash.fold(parent, &.{}, 0, null)));
}

test "a boundary preimage is not consulted at a chunk boundary" {
    const parent = filled(0x10);
    const chunks = [_]Hash{filled(0xa1)};
    const expected = step(parent, chunks[0]);
    try std.testing.expectEqualSlices(u8, &expected, &(try rolling_hash.fold(parent, &chunks, 0, filled(0xee))));
}

test "a range starting inside a chunk resumes after the chunk its parent hash includes" {
    const before = filled(0x10);
    const chunks = [_]Hash{ filled(0xa1), filled(0xa2), filled(0xa3) };
    const parent = step(before, chunks[0]);
    const expected = step(step(parent, chunks[1]), chunks[2]);
    try std.testing.expectEqualSlices(u8, &expected, &(try rolling_hash.fold(parent, &chunks, 7, before)));
}

test "a range ending with the chunk its parent hash includes keeps the parent hash" {
    const before = filled(0x10);
    const first = filled(0xa1);
    const parent = step(before, first);
    try std.testing.expectEqualSlices(u8, &parent, &(try rolling_hash.fold(parent, &.{first}, 1, before)));
}

test "a range starting inside a chunk needs the preimage that opens its parent hash" {
    const before = filled(0x10);
    const chunks = [_]Hash{ filled(0xa1), filled(0xa2) };
    const parent = step(before, chunks[0]);

    try std.testing.expectError(error.BoundaryPreimageMismatch, rolling_hash.fold(parent, &chunks, 7, null));
    try std.testing.expectError(error.BoundaryPreimageMismatch, rolling_hash.fold(parent, &chunks, 7, filled(0x11)));
    // The preimage opens the hash of the first chunk, not of another one.
    try std.testing.expectError(error.BoundaryPreimageMismatch, rolling_hash.fold(parent, chunks[1..], 7, before));
    try std.testing.expectError(error.BoundaryPreimageMismatch, rolling_hash.fold(parent, &.{}, 7, before));
}

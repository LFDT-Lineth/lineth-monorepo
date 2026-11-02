//! Blob commitments and proofs computed by c-kzg-4844 under its trusted setup: the real KZG data of
//! the DA tests and samples, from a library independent of the package under test.

const std = @import("std");

const c = @import("c_kzg");
const c_kzg_paths = @import("c_kzg_paths");

pub const blob_len = 4096 * 32;

comptime {
    std.debug.assert(@sizeOf(c.Blob) == blob_len);
}

/// The EIP-4844 commitment of a blob and the blob proof of that commitment.
pub const Opening = struct {
    commitment: [48]u8,
    proof: [48]u8,
};

/// Loaded on first use. The tests and tools that use it are single-threaded.
var settings: ?c.KZGSettings = null;
/// Openings by the sha256 of their blob, kept for the life of the process.
var openings: std.AutoHashMapUnmanaged([32]u8, Opening) = .empty;

fn loadedSettings() *const c.KZGSettings {
    if (settings == null) {
        var path: [std.fs.max_path_bytes]u8 = undefined;
        const trusted_setup = std.fmt.bufPrintZ(&path, "{s}", .{c_kzg_paths.trusted_setup}) catch @panic("trusted setup path too long");
        const file = c.fopen(trusted_setup, "rb") orelse @panic("cannot open the trusted setup");
        defer _ = c.fclose(file);
        var loaded: c.KZGSettings = undefined;
        check(c.load_trusted_setup_file(&loaded, file, 8));
        settings = loaded;
    }
    return &settings.?;
}

fn check(result: c.C_KZG_RET) void {
    if (result != c.C_KZG_OK) std.debug.panic("c-kzg-4844 failed with code {d}", .{result});
}

/// The commitment of `blob` and its blob proof.
pub fn open(blob: *const [blob_len]u8) Opening {
    var digest: [32]u8 = undefined;
    std.crypto.hash.sha2.Sha256.hash(blob, &digest, .{});
    if (openings.get(digest)) |known| return known;

    const s = loadedSettings();
    const as_blob: *const c.Blob = @ptrCast(blob);
    var commitment: c.KZGCommitment = undefined;
    check(c.blob_to_kzg_commitment(&commitment, as_blob, s));
    var proof: c.KZGProof = undefined;
    check(c.compute_blob_kzg_proof(&proof, as_blob, &commitment, s));
    const opening: Opening = .{ .commitment = commitment.bytes, .proof = proof.bytes };
    openings.put(std.heap.page_allocator, digest, opening) catch @panic("out of memory");
    return opening;
}

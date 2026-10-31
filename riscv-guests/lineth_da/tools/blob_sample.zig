//! `blob_sample` — builds the input of the `blob` smoke guest: a case of the official EIP-4844
//! reference vectors whose proof verifies.
//!
//! usage: blob_sample <input-out>
//!
//! The input is the 48-byte commitment, the 48-byte proof and the 131072-byte blob of the case.

const std = @import("std");

const reference_vectors = @import("reference_vectors");

const suite = "verify_blob_kzg_proof";
const case_name = "verify_blob_kzg_proof_case_correct_proof_0";

pub fn main(init: std.process.Init) !void {
    const args = try init.minimal.args.toSlice(init.arena.allocator());
    if (args.len != 2) {
        std.debug.print("error: expected <input-out>\nusage: blob_sample <input-out>\n", .{});
        std.process.exit(2);
    }

    var cases = try reference_vectors.loadSuite(init.gpa, init.io, suite);
    defer reference_vectors.freeCases(init.gpa, &cases);
    for (cases.items) |case| {
        if (!std.mem.eql(u8, case.name, case_name)) continue;
        const input = try std.mem.concat(init.gpa, u8, &.{ case.commitment.?, case.proof.?, case.blob.? });
        defer init.gpa.free(input);
        std.Io.Dir.cwd().writeFile(init.io, .{ .sub_path = args[1], .data = input }) catch |err| {
            std.debug.print("error: cannot write '{s}': {s}\n", .{ args[1], @errorName(err) });
            std.process.exit(1);
        };
        return;
    }
    std.debug.print("error: the reference vectors have no case {s}\n", .{case_name});
    std.process.exit(1);
}

const std = @import("std");

pub fn main(init: std.process.Init) !void {
    const args = try init.minimal.args.toSlice(init.arena.allocator());
    if (args.len != 4) return error.ExpectedElfOutputDirectoryAndIdFile;

    const id = try stageElf(init.io, init.gpa, args[1], args[2]);
    const hex = std.fmt.bytesToHex(id, .lower);
    try std.Io.Dir.cwd().writeFile(init.io, .{ .sub_path = args[3], .data = &hex });
}

fn stageElf(io: std.Io, allocator: std.mem.Allocator, source: []const u8, output_directory: []const u8) ![32]u8 {
    const cwd = std.Io.Dir.cwd();
    const file = try cwd.openFile(io, source, .{});
    defer file.close(io);

    var hasher = std.crypto.hash.sha3.Keccak256.init(.{});
    var buffer: [64 * 1024]u8 = undefined;
    while (true) {
        const n = file.readStreaming(io, &.{&buffer}) catch |err| switch (err) {
            error.EndOfStream => break,
            else => return err,
        };
        if (n == 0) break;
        hasher.update(buffer[0..n]);
    }
    var id: [32]u8 = undefined;
    hasher.final(&id);

    try cwd.createDirPath(io, output_directory);
    const asset_name = try std.fmt.allocPrint(allocator, "{s}/{s}.elf", .{ output_directory, std.fmt.bytesToHex(id, .lower) });
    defer allocator.free(asset_name);
    try cwd.copyFile(source, cwd, asset_name, io, .{ .replace = false });
    return id;
}

test "stage complete ELF and refuse replacement" {
    const io = std.testing.io;
    const cwd = std.Io.Dir.cwd();
    const allocator = std.testing.allocator;
    var random_bytes: [8]u8 = undefined;
    io.random(&random_bytes);
    const suffix = std.fmt.bytesToHex(random_bytes, .lower);
    const source = try std.fmt.allocPrint(allocator, "guest-program-id-test-{s}.elf", .{suffix});
    defer allocator.free(source);
    defer cwd.deleteFile(io, source) catch {};
    const output_directory = try std.fmt.allocPrint(allocator, "guest-program-id-assets-{s}", .{suffix});
    defer allocator.free(output_directory);
    defer cwd.deleteTree(io, output_directory) catch {};

    const elf = "\x7fELFguest data";
    try cwd.writeFile(io, .{ .sub_path = source, .data = elf });
    const id = try stageElf(io, allocator, source, output_directory);
    try std.testing.expectEqualStrings("8e7dae55483e7b63d384e441238245d76cf07ab4dd89eaac76e00ce9e811a665", &std.fmt.bytesToHex(id, .lower));

    const asset_name = try std.fmt.allocPrint(allocator, "{s}/{s}.elf", .{ output_directory, std.fmt.bytesToHex(id, .lower) });
    defer allocator.free(asset_name);
    const asset = try cwd.readFileAlloc(io, asset_name, allocator, .limited(1024));
    defer allocator.free(asset);
    try std.testing.expectEqualStrings(elf, asset);
    try std.testing.expectError(error.PathAlreadyExists, stageElf(io, allocator, source, output_directory));
}

const std = @import("std");

pub fn main(init: std.process.Init) !void {
    const args = try init.minimal.args.toSlice(init.arena.allocator());
    if (args.len != 6) return error.ExpectedElfOutputDirectoryIdFileGuestAndVersion;

    const staged = try stageElf(init.io, init.gpa, args[1], args[2], args[4], args[5]);
    defer init.gpa.free(staged.filename);
    const hex = std.fmt.bytesToHex(staged.id, .lower);
    try writeProgramId(init.io, args[3], &hex);
    const filename_file = try std.fmt.allocPrint(init.gpa, "{s}.asset", .{args[3]});
    defer init.gpa.free(filename_file);
    try writeProgramId(init.io, filename_file, staged.filename);
}

fn writeProgramId(io: std.Io, path: []const u8, hex: []const u8) !void {
    const cwd = std.Io.Dir.cwd();
    if (std.fs.path.dirname(path)) |parent| try cwd.createDirPath(io, parent);
    try cwd.writeFile(io, .{ .sub_path = path, .data = hex });
}

const StagedElf = struct { id: [32]u8, filename: []const u8 };

fn stageElf(io: std.Io, allocator: std.mem.Allocator, source: []const u8, output_directory: []const u8, guest: []const u8, version: []const u8) !StagedElf {
    if (guest.len == 0 or version.len == 0 or version[0] == 'v') return error.InvalidReleaseName;
    for (guest) |c| if (!std.ascii.isAlphanumeric(c) and c != '-') return error.InvalidReleaseName;
    for (version) |c| if (!std.ascii.isAlphanumeric(c) and c != '.' and c != '-' and c != '+') return error.InvalidReleaseName;
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
    const filename = try std.fmt.allocPrint(allocator, "{s}-v{s}-{s}.elf", .{ guest, version, std.fmt.bytesToHex(id, .lower) });
    errdefer allocator.free(filename);
    const asset_name = try std.fmt.allocPrint(allocator, "{s}/{s}", .{ output_directory, filename });
    defer allocator.free(asset_name);
    try cwd.copyFile(source, cwd, asset_name, io, .{ .replace = false });
    return .{ .id = id, .filename = filename };
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
    const staged = try stageElf(io, allocator, source, output_directory, "rollup", "0.0.1");
    defer allocator.free(staged.filename);
    try std.testing.expectEqualStrings("8e7dae55483e7b63d384e441238245d76cf07ab4dd89eaac76e00ce9e811a665", &std.fmt.bytesToHex(staged.id, .lower));

    const asset_name = try std.fmt.allocPrint(allocator, "{s}/{s}", .{ output_directory, staged.filename });
    defer allocator.free(asset_name);
    const asset = try cwd.readFileAlloc(io, asset_name, allocator, .limited(1024));
    defer allocator.free(asset);
    try std.testing.expectEqualStrings(elf, asset);
    try std.testing.expectError(error.PathAlreadyExists, stageElf(io, allocator, source, output_directory, "rollup", "0.0.1"));

    for ([_][]const u8{ "l2-execution", "rollup" }) |guest| {
        const released = try stageElf(io, allocator, source, output_directory, guest, "0.0.1-debug");
        defer allocator.free(released.filename);
        try std.testing.expectEqual(staged.id, released.id);
        const expected_name = try std.fmt.allocPrint(allocator, "{s}-v0.0.1-debug-{s}.elf", .{ guest, std.fmt.bytesToHex(released.id, .lower) });
        defer allocator.free(expected_name);
        try std.testing.expectEqualStrings(expected_name, released.filename);
        const released_path = try std.fmt.allocPrint(allocator, "{s}/{s}", .{ output_directory, released.filename });
        defer allocator.free(released_path);
        const bytes = try cwd.readFileAlloc(io, released_path, allocator, .limited(1024));
        defer allocator.free(bytes);
        try std.testing.expectEqualStrings(elf, bytes);
        try std.testing.expectError(error.PathAlreadyExists, stageElf(io, allocator, source, output_directory, guest, "0.0.1-debug"));
    }
    try std.testing.expectError(error.InvalidReleaseName, stageElf(io, allocator, source, output_directory, "", "0.0.1"));
    try std.testing.expectError(error.InvalidReleaseName, stageElf(io, allocator, source, output_directory, "rollup", ""));
    try std.testing.expectError(error.InvalidReleaseName, stageElf(io, allocator, source, output_directory, "", ""));
    try std.testing.expectError(error.InvalidReleaseName, stageElf(io, allocator, source, output_directory, "rollup", "v0.0.1"));
    try std.testing.expectError(error.InvalidReleaseName, stageElf(io, allocator, source, output_directory, "rollup", "0.0.1/unsafe"));
}

test "write Program ID into a fresh build directory" {
    const io = std.testing.io;
    const cwd = std.Io.Dir.cwd();
    const allocator = std.testing.allocator;
    var random_bytes: [8]u8 = undefined;
    io.random(&random_bytes);
    const directory = try std.fmt.allocPrint(allocator, "guest-program-id-build-{s}", .{std.fmt.bytesToHex(random_bytes, .lower)});
    defer allocator.free(directory);
    defer cwd.deleteTree(io, directory) catch {};

    const path = try std.fmt.allocPrint(allocator, "{s}/zig-out/program-id", .{directory});
    defer allocator.free(path);
    try writeProgramId(io, path, "1234");
    const stored = try cwd.readFileAlloc(io, path, allocator, .limited(64));
    defer allocator.free(stored);
    try std.testing.expectEqualStrings("1234", stored);
}

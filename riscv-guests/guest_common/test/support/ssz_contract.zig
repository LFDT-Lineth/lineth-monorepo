//! Host-only checks that a decoded wire struct holds exactly what the spec's JSON fixture holds.
//!
//! `expectMatches` walks the struct's fields, looks each one up in the JSON object under the
//! camelCase of its name and compares by Zig type. The list views are recognised by shape:
//! `bytes` is a byte list, `items` over arrays or integers a fixed list, `items` over structs a
//! variable list and a lone `value: bool` a strict boolean. A fixture that names or shapes a field
//! differently from the wire is described by a `rules` type declaring
//! `pub fn rule(comptime T: type, comptime field: []const u8) Rule`.

const std = @import("std");

const Allocator = std.mem.Allocator;
const Value = std.json.Value;

/// How the JSON holds one field of a struct.
pub const Rule = union(enum) {
    /// The camelCase key, compared by the field's type.
    default,
    /// The dotted path of the key, in place of the camelCase of the field name.
    path: []const u8,
    /// Not compared; the declaring `rules` says what covers the field instead.
    skip,
    /// The field is a struct whose fields sit in the same JSON object as its parent's.
    flatten,
    /// A list of at most one element that the JSON holds as `null` or the element itself.
    nullable,
    /// An integer that the JSON holds as the name at that index.
    enum_names: []const []const u8,
};

pub const NoRules = struct {};

/// Whether `field` of `T` is the field `name` of `Owner`.
pub fn isField(comptime T: type, comptime field: []const u8, comptime Owner: type, comptime name: []const u8) bool {
    return T == Owner and std.mem.eql(u8, field, name);
}

/// Fails unless `value` holds what `json` holds, field by field.
pub fn expectMatches(comptime T: type, comptime Rules: type, alloc: Allocator, value: T, json: Value) !void {
    try compare(T, Rules, alloc, value, json, "$");
}

/// The bytes of the fixture `name` under the directory `dir`.
pub fn readFixture(alloc: Allocator, io: std.Io, dir: []const u8, name: []const u8) ![]u8 {
    const path = try std.fs.path.join(alloc, &.{ dir, name });
    defer alloc.free(path);
    return std.Io.Dir.cwd().readFileAlloc(io, path, alloc, .limited(1 << 30));
}

/// The parsed JSON fixture `name` under the directory `dir`; it lives as long as `arena`.
pub fn readJson(arena: Allocator, io: std.Io, dir: []const u8, name: []const u8) !Value {
    const text = try readFixture(arena, io, dir, name);
    return std.json.parseFromSliceLeaky(Value, arena, text, .{});
}

/// The frame of a guest output `keccak256(frame) || frame`, after its commitment and schema id are
/// checked.
pub fn framedBody(output: []const u8, schema_id: u16) ![]const u8 {
    try std.testing.expect(output.len >= 34);
    const frame = output[32..];
    var digest: [32]u8 = undefined;
    std.crypto.hash.sha3.Keccak256.hash(frame, &digest, .{});
    try std.testing.expectEqualSlices(u8, &digest, output[0..32]);
    try std.testing.expectEqual(schema_id, std.mem.readInt(u16, frame[0..2], .big));
    return frame[2..];
}

fn fail(path: []const u8, comptime fmt: []const u8, args: anytype) anyerror {
    std.debug.print("contract mismatch at {s}: " ++ fmt ++ "\n", .{path} ++ args);
    return error.ContractMismatch;
}

fn camelCase(comptime snake: []const u8) []const u8 {
    comptime {
        var out: [snake.len]u8 = undefined;
        var len: usize = 0;
        var upper = false;
        for (snake) |c| {
            if (c == '_') {
                upper = true;
                continue;
            }
            out[len] = if (upper) std.ascii.toUpper(c) else c;
            len += 1;
            upper = false;
        }
        const final = out[0..len].*;
        return &final;
    }
}

fn ruleFor(comptime Rules: type, comptime T: type, comptime field: []const u8) Rule {
    if (@hasDecl(Rules, "rule")) return Rules.rule(T, field);
    return .default;
}

fn lookup(json: Value, path: []const u8) ?Value {
    var current = json;
    var parts = std.mem.splitScalar(u8, path, '.');
    while (parts.next()) |part| {
        if (current != .object) return null;
        current = current.object.get(part) orelse return null;
    }
    return current;
}

fn isByteList(comptime T: type) bool {
    const fields = std.meta.fields(T);
    return fields.len == 1 and std.mem.eql(u8, fields[0].name, "bytes");
}

fn isStrictBool(comptime T: type) bool {
    const fields = std.meta.fields(T);
    return fields.len == 1 and std.mem.eql(u8, fields[0].name, "value") and fields[0].type == bool;
}

fn isList(comptime T: type) bool {
    const fields = std.meta.fields(T);
    return fields.len == 1 and std.mem.eql(u8, fields[0].name, "items");
}

fn compare(comptime T: type, comptime Rules: type, alloc: Allocator, value: T, json: Value, path: []const u8) anyerror!void {
    switch (@typeInfo(T)) {
        .array => |array| {
            comptime std.debug.assert(array.child == u8);
            try expectHex(alloc, &value, json, path);
        },
        .int => try expectInt(T, value, json, path),
        .@"struct" => if (comptime isByteList(T)) {
            try expectHex(alloc, value.bytes, json, path);
        } else if (comptime isStrictBool(T)) {
            if (json != .bool) return fail(path, "expected a JSON boolean", .{});
            if (json.bool != value.value) return fail(path, "{} != {}", .{ value.value, json.bool });
        } else if (comptime isList(T)) {
            try compareList(@typeInfo(@TypeOf(value.items)).pointer.child, Rules, alloc, value.items, json, path);
        } else {
            try compareStruct(T, Rules, alloc, value, json, path);
        },
        else => @compileError("no contract comparison for " ++ @typeName(T)),
    }
}

fn compareList(comptime E: type, comptime Rules: type, alloc: Allocator, items: anytype, json: Value, path: []const u8) anyerror!void {
    if (json != .array) return fail(path, "expected a JSON array", .{});
    if (json.array.items.len != items.len) return fail(path, "{d} elements != {d} in the JSON", .{ items.len, json.array.items.len });
    for (items, json.array.items, 0..) |item, element, i| {
        var buf: [256]u8 = undefined;
        const child = std.fmt.bufPrint(&buf, "{s}[{d}]", .{ path, i }) catch path;
        try compare(E, Rules, alloc, item, element, child);
    }
}

fn compareStruct(comptime T: type, comptime Rules: type, alloc: Allocator, value: T, json: Value, path: []const u8) anyerror!void {
    if (json != .object) return fail(path, "expected a JSON object", .{});
    inline for (std.meta.fields(T)) |field| {
        const rule = comptime ruleFor(Rules, T, field.name);
        var buf: [256]u8 = undefined;
        const child = std.fmt.bufPrint(&buf, "{s}.{s}", .{ path, field.name }) catch path;
        const own = @field(value, field.name);
        switch (rule) {
            .skip => {},
            .flatten => try compare(field.type, Rules, alloc, own, json, child),
            .default, .path, .nullable, .enum_names => {
                const key = comptime switch (rule) {
                    .path => |p| p,
                    else => camelCase(field.name),
                };
                const element = lookup(json, key) orelse return fail(child, "no JSON key {s}", .{key});
                switch (rule) {
                    .nullable => try expectNullable(field.type, Rules, alloc, own, element, child),
                    .enum_names => |names| try expectEnum(names, own, element, child),
                    else => try compare(field.type, Rules, alloc, own, element, child),
                }
            },
        }
    }
}

fn expectNullable(comptime T: type, comptime Rules: type, alloc: Allocator, value: T, json: Value, path: []const u8) !void {
    if (value.items.len > 1) return fail(path, "{d} elements in a nullable list", .{value.items.len});
    if (json == .null) {
        if (value.items.len != 0) return fail(path, "{d} elements != null in the JSON", .{value.items.len});
        return;
    }
    if (value.items.len != 1) return fail(path, "absent != the JSON element", .{});
    const E = @typeInfo(@TypeOf(value.items)).pointer.child;
    try compare(E, Rules, alloc, value.items[0], json, path);
}

fn expectEnum(comptime names: []const []const u8, value: anytype, json: Value, path: []const u8) !void {
    if (json != .string) return fail(path, "expected a JSON string", .{});
    if (value >= names.len) return fail(path, "{d} names no variant", .{value});
    if (!std.mem.eql(u8, names[value], json.string)) return fail(path, "{s} != {s} in the JSON", .{ names[value], json.string });
}

fn expectHex(alloc: Allocator, bytes: []const u8, json: Value, path: []const u8) !void {
    if (json != .string) return fail(path, "expected a 0x-hex JSON string", .{});
    const text = json.string;
    if (!std.mem.startsWith(u8, text, "0x") or text.len % 2 != 0) return fail(path, "not 0x-hex: {s}", .{text});
    const want = try alloc.alloc(u8, (text.len - 2) / 2);
    defer alloc.free(want);
    _ = std.fmt.hexToBytes(want, text[2..]) catch return fail(path, "not 0x-hex: {s}", .{text});
    if (!std.mem.eql(u8, want, bytes)) return fail(path, "bytes differ from the JSON hex", .{});
}

fn expectInt(comptime T: type, value: T, json: Value, path: []const u8) !void {
    const parsed: T = switch (json) {
        .integer => |n| std.math.cast(T, n) orelse return fail(path, "{d} out of range", .{n}),
        .number_string => |s| std.fmt.parseInt(T, s, 10) catch return fail(path, "not an integer: {s}", .{s}),
        .string => |s| (if (std.mem.startsWith(u8, s, "0x"))
            std.fmt.parseInt(T, s[2..], 16)
        else
            std.fmt.parseInt(T, s, 10)) catch return fail(path, "not an integer: {s}", .{s}),
        else => return fail(path, "expected an integer", .{}),
    };
    if (parsed != value) return fail(path, "{d} != {d} in the JSON", .{ value, parsed });
}

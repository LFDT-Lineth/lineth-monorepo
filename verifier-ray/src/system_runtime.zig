const std = @import("std");
const protocol = @import("protocol/root.zig");
const verifier = @import("verifier.zig");

const magic = "VRS1";

/// The complete verifier metadata bundle. Codegen writes this schema to a
/// compact binary and the guest decodes it into an arena once at startup.
pub const Bundle = struct {
    spec: protocol.Spec,
    systems: verifier.Systems,
};

pub const Error = error{
    InvalidMagic,
    InvalidBoolean,
    InvalidTag,
    InvalidVarint,
    NonCanonicalVarint,
    IntegerOverflow,
    TrailingBytes,
    OutOfMemory,
};

/// Decodes a generated bundle into caller-owned storage. Every slice in the
/// returned value points into `allocator`; the encoded bytes need only remain
/// alive for the duration of this call.
pub fn decodeBundle(bytes: []const u8, allocator: std.mem.Allocator) Error!Bundle {
    if (bytes.len < magic.len or !std.mem.eql(u8, bytes[0..magic.len], magic)) return error.InvalidMagic;
    var reader = Reader{ .bytes = bytes, .cursor = magic.len, .allocator = allocator };
    const bundle = try reader.decodeValue(Bundle);
    if (reader.cursor != bytes.len) return error.TrailingBytes;
    return bundle;
}

const Reader = struct {
    bytes: []const u8,
    cursor: usize,
    allocator: std.mem.Allocator,

    fn readByte(self: *Reader) Error!u8 {
        if (self.cursor >= self.bytes.len) return error.InvalidVarint;
        const byte = self.bytes[self.cursor];
        self.cursor += 1;
        return byte;
    }

    fn readVarint(self: *Reader) Error!u64 {
        var value: u64 = 0;
        var shift: u6 = 0;
        var count: usize = 0;
        while (count < 10) : (count += 1) {
            const byte = try self.readByte();
            const payload = byte & 0x7f;
            if (shift == 63 and payload > 1) return error.IntegerOverflow;
            value |= @as(u64, payload) << shift;
            if (byte & 0x80 == 0) {
                if (count > 0 and payload == 0) return error.NonCanonicalVarint;
                return value;
            }
            if (shift >= 63) return error.IntegerOverflow;
            shift += 7;
        }
        return error.InvalidVarint;
    }

    fn decodeInteger(self: *Reader, comptime T: type) Error!T {
        const encoded = try self.readVarint();
        return switch (@typeInfo(T).int.signedness) {
            .unsigned => std.math.cast(T, encoded) orelse error.IntegerOverflow,
            .signed => blk: {
                const magnitude = encoded >> 1;
                const wide: i128 = if (encoded & 1 == 0)
                    @intCast(magnitude)
                else
                    -@as(i128, @intCast(magnitude)) - 1;
                break :blk std.math.cast(T, wide) orelse error.IntegerOverflow;
            },
        };
    }

    fn decodeValue(self: *Reader, comptime T: type) Error!T {
        return switch (@typeInfo(T)) {
            .int => try self.decodeInteger(T),
            .bool => switch (try self.readByte()) {
                0 => false,
                1 => true,
                else => error.InvalidBoolean,
            },
            .@"enum" => |enum_info| blk: {
                const raw = try self.decodeInteger(enum_info.tag_type);
                inline for (enum_info.fields) |field| {
                    if (raw == field.value) break :blk @as(T, @enumFromInt(raw));
                }
                return error.InvalidTag;
            },
            .optional => |opt| switch (try self.readByte()) {
                0 => null,
                1 => try self.decodeValue(opt.child),
                else => error.InvalidTag,
            },
            .array => |array| blk: {
                var result: T = undefined;
                for (&result) |*item| item.* = try self.decodeValue(array.child);
                break :blk result;
            },
            .pointer => |pointer| blk: {
                if (pointer.size != .slice) @compileError("system-runtime only supports slice pointers");
                const len = try self.decodeInteger(usize);
                const result = self.allocator.alloc(pointer.child, len) catch return error.OutOfMemory;
                for (result) |*item| item.* = try self.decodeValue(pointer.child);
                break :blk result;
            },
            .@"struct" => |structure| blk: {
                var result: T = undefined;
                inline for (structure.fields) |field| @field(result, field.name) = try self.decodeValue(field.type);
                break :blk result;
            },
            .@"union" => |union_info| blk: {
                const Tag = union_info.tag_type orelse @compileError("system-runtime requires tagged unions");
                const active = try self.decodeValue(Tag);
                inline for (union_info.fields) |field| {
                    if (active == @field(Tag, field.name)) {
                        break :blk @unionInit(T, field.name, try self.decodeValue(field.type));
                    }
                }
                return error.InvalidTag;
            },
            else => @compileError("unsupported system-runtime type: " ++ @typeName(T)),
        };
    }
};

const std = @import("std");
const r5_mem = @import("verifier_ray").r5_mem;

// Every length up to a few words, at every source and destination offset
// within an 8-byte word, so each of the 8-byte, 4-byte and byte paths runs,
// including their tails.
const max_len = 40;
const pattern_len = max_len + 16;

fn pattern() [pattern_len]u8 {
    var p: [pattern_len]u8 = undefined;
    for (&p, 0..) |*b, i| b.* = @intCast((i * 37 + 11) % 251);
    return p;
}

test "r5 memcpy matches a byte copy at every length and alignment" {
    const src_buf align(8) = pattern();
    for (0..max_len + 1) |len| {
        for (0..8) |s_off| {
            for (0..8) |d_off| {
                var dest align(8) = [_]u8{0xee} ** pattern_len;
                _ = r5_mem.memcpy(dest[d_off..].ptr, src_buf[s_off..].ptr, len);
                try std.testing.expectEqualSlices(u8, src_buf[s_off..][0..len], dest[d_off..][0..len]);
                // Nothing outside [d_off, d_off + len) is touched.
                for (dest[0..d_off]) |b| try std.testing.expectEqual(@as(u8, 0xee), b);
                for (dest[d_off + len ..]) |b| try std.testing.expectEqual(@as(u8, 0xee), b);
            }
        }
    }
}

test "r5 memmove matches a reference for overlapping ranges in both directions" {
    for (0..max_len + 1) |len| {
        for (0..16) |s_off| {
            for (0..16) |d_off| {
                var buf align(8) = pattern();
                var want = pattern();
                // Reference: copy through a temporary, which is what memmove means.
                var tmp: [pattern_len]u8 = undefined;
                @memcpy(tmp[0..len], want[s_off..][0..len]);
                @memcpy(want[d_off..][0..len], tmp[0..len]);
                _ = r5_mem.memmove(buf[d_off..].ptr, buf[s_off..].ptr, len);
                try std.testing.expectEqualSlices(u8, &want, &buf);
            }
        }
    }
}

test "r5 memset fills exactly the requested bytes" {
    for (0..max_len + 1) |len| {
        for (0..8) |d_off| {
            var dest align(8) = [_]u8{0xee} ** pattern_len;
            _ = r5_mem.memset(dest[d_off..].ptr, 0x5a, len);
            for (dest, 0..) |b, i| {
                const inside = i >= d_off and i < d_off + len;
                try std.testing.expectEqual(@as(u8, if (inside) 0x5a else 0xee), b);
            }
        }
    }
}

// FROZEN test vectors: NOT auto-generated. See fri_test.zig's doc comment
// for why, and the follow-up issue that replaces this with real generation.
//
// Values converted from the honest fold as it stood at prover-ray commit
// 44452c8d0cdf7276ac0f4c8f797b77d7aac9e12a.

const verifier_ray = @import("verifier_ray");
const fri = verifier_ray.query.fri;

pub const FoldCase = struct {
    name: []const u8,
    params: fri.Params,
    fold_alphas: []const [6]u32,
    round_roots: []const [8]u32,
    final_poly: []const [6]u32,
    position: usize,
    running_branches: []const RawBranch,
    expected_rounds: []const RawPair,
    aux: []const ?RawPair,
};

pub const RawBranch = struct { leaf: [8]u32, siblings: []const [8]u32 };
pub const RawPair = struct { self: [6]u32, sibling: [6]u32 };

pub const fold_cases = [_]FoldCase{
    .{
        .name = "single_level_3rounds",
        .params = .{ .log_codeword_size = 4, .log_plaintext_size = 3, .log_final_poly_size = 0, .num_queries = 1 },
        .fold_alphas = &.{ .{ 7, 0, 0, 0, 0, 0 }, .{ 11, 0, 0, 0, 0, 0 }, .{ 17, 0, 0, 0, 0, 0 } },
        .round_roots = &.{ .{ 1503149432, 380418181, 1223702784, 519253104, 1450947468, 1189824450, 2046021059, 1095007226 }, .{ 1559958242, 1649082880, 1896083092, 698469895, 1870263476, 277089838, 624376519, 229237959 } },
        .final_poly = &.{.{ 0, 0, 0, 0, 0, 0 }},
        .position = 13,
        .running_branches = &.{ .{ .leaf = .{ 0, 0, 0, 0, 0, 0, 0, 0 }, .siblings = &.{ .{ 1559958242, 1649082880, 1896083092, 698469895, 1870263476, 277089838, 624376519, 229237959 }, .{ 106343301, 994005636, 56797207, 2116123491, 205490027, 300435510, 1363520036, 570836988 }, .{ 0, 0, 0, 0, 0, 0, 0, 0 } } }, .{ .leaf = .{ 0, 0, 0, 0, 0, 0, 0, 0 }, .siblings = &.{ .{ 106343301, 994005636, 56797207, 2116123491, 205490027, 300435510, 1363520036, 570836988 }, .{ 0, 0, 0, 0, 0, 0, 0, 0 } } } },
        .expected_rounds = &.{ .{ .self = .{ 0, 0, 0, 0, 0, 0 }, .sibling = .{ 0, 0, 0, 0, 0, 0 } }, .{ .self = .{ 0, 0, 0, 0, 0, 0 }, .sibling = .{ 0, 0, 0, 0, 0, 0 } }, .{ .self = .{ 0, 0, 0, 0, 0, 0 }, .sibling = .{ 0, 0, 0, 0, 0, 0 } } },
        .aux = &.{ .{ .self = .{ 0, 0, 0, 0, 0, 0 }, .sibling = .{ 0, 0, 0, 0, 0, 0 } }, null, null, null },
    },
    .{
        .name = "two_levels_3rounds",
        .params = .{ .log_codeword_size = 4, .log_plaintext_size = 3, .log_final_poly_size = 0, .num_queries = 1 },
        .fold_alphas = &.{ .{ 7, 0, 0, 0, 0, 0 }, .{ 11, 0, 0, 0, 0, 0 }, .{ 17, 0, 0, 0, 0, 0 } },
        .round_roots = &.{ .{ 1503149432, 380418181, 1223702784, 519253104, 1450947468, 1189824450, 2046021059, 1095007226 }, .{ 1559958242, 1649082880, 1896083092, 698469895, 1870263476, 277089838, 624376519, 229237959 } },
        .final_poly = &.{.{ 0, 0, 0, 0, 0, 0 }},
        .position = 6,
        .running_branches = &.{ .{ .leaf = .{ 0, 0, 0, 0, 0, 0, 0, 0 }, .siblings = &.{ .{ 1559958242, 1649082880, 1896083092, 698469895, 1870263476, 277089838, 624376519, 229237959 }, .{ 106343301, 994005636, 56797207, 2116123491, 205490027, 300435510, 1363520036, 570836988 }, .{ 0, 0, 0, 0, 0, 0, 0, 0 } } }, .{ .leaf = .{ 0, 0, 0, 0, 0, 0, 0, 0 }, .siblings = &.{ .{ 106343301, 994005636, 56797207, 2116123491, 205490027, 300435510, 1363520036, 570836988 }, .{ 0, 0, 0, 0, 0, 0, 0, 0 } } } },
        .expected_rounds = &.{ .{ .self = .{ 0, 0, 0, 0, 0, 0 }, .sibling = .{ 0, 0, 0, 0, 0, 0 } }, .{ .self = .{ 0, 0, 0, 0, 0, 0 }, .sibling = .{ 0, 0, 0, 0, 0, 0 } }, .{ .self = .{ 0, 0, 0, 0, 0, 0 }, .sibling = .{ 0, 0, 0, 0, 0, 0 } } },
        .aux = &.{ .{ .self = .{ 0, 0, 0, 0, 0, 0 }, .sibling = .{ 0, 0, 0, 0, 0, 0 } }, null, .{ .self = .{ 0, 0, 0, 0, 0, 0 }, .sibling = .{ 0, 0, 0, 0, 0, 0 } }, null },
    },
};

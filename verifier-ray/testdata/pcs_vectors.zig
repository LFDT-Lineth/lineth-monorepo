// Static PCS integration vectors for verifier-ray.
//
// These cases are synthetic but fixed: they pin the integrated PCS control flow
// across DEEP-alpha binding, FRI, point-sampling Merkle checks, and the DEEP
// bridge. Loom-captured primitive vectors cover the individual hashing and
// transcript encodings used here.

pub const PcsIntegrationValue = struct { name: []const u8, value: [6]u32 };
pub const PcsIntegrationSlot = struct { name: []const u8, tree_idx: u32, poly_idx: u32 };

pub const PcsIntegrationCase = struct {
    n: u32,
    d: u32,
    num_queries: u32,
    num_rounds: u32,
    query_position: u32,
    zeta_bindings: []const u32,
    zeta_digest: [8]u32,
    zeta: [6]u32,
    alpha_deep_digest: [8]u32,
    alpha_deep: [6]u32,
    fri_alpha_digest: [8]u32,
    query_digest: [8]u32,
    level_ds: []const u32,
    tree_sizes: []const u32,
    values_at_zeta: []const PcsIntegrationValue,
    air_chunk_slots: []const PcsIntegrationSlot,
    deep_quotient_commitment: []const [8]u32,
    point_sampling_roots: []const [8]u32,
    point_sample_leaf_idx: u32,
    point_sample_ext_pairs: []const [2][6]u32,
    point_sample_siblings: []const [8]u32,
    fri_layer_leaf_idx: u32,
    fri_layer_leaf_p_ext: [6]u32,
    fri_layer_leaf_q_ext: [6]u32,
    fri_layer_siblings: []const [8]u32,
    final_poly_ext: []const [6]u32,
};

pub const pcs_integration_cases = [_]PcsIntegrationCase{
    .{
        .n = 8,
        .d = 2,
        .num_queries = 1,
        .num_rounds = 1,
        .query_position = 2,
        .zeta_bindings = &.{ 3, 1, 4, 1 },
        .zeta_digest = .{ 1917250265, 622384269, 2073146317, 1819043742, 640412108, 317252013, 1545741056, 225608101 },
        .zeta = .{ 1917250265, 622384269, 2073146317, 1819043742, 640412108, 317252013 },
        .alpha_deep_digest = .{ 1488707062, 1972828663, 1845697837, 1378876639, 278187330, 1685395255, 99296853, 1124465556 },
        .alpha_deep = .{ 1488707062, 1972828663, 1845697837, 1378876639, 278187330, 1685395255 },
        .fri_alpha_digest = .{ 1340927696, 127747015, 542095034, 72911870, 1232570873, 1011156828, 442642762, 842131438 },
        .query_digest = .{ 182475865, 1238244818, 1090252346, 1028680778, 1771492716, 1952150256, 1764574442, 1622032540 },
        .level_ds = &.{2},
        .tree_sizes = &.{2},
        .values_at_zeta = &.{
            .{ .name = "air0", .value = .{ 41, 43, 47, 53, 59, 61 } },
            .{ .name = "air1", .value = .{ 67, 71, 73, 79, 83, 89 } },
        },
        .air_chunk_slots = &.{
            .{ .name = "air0", .tree_idx = 0, .poly_idx = 0 },
            .{ .name = "air1", .tree_idx = 0, .poly_idx = 1 },
        },
        .deep_quotient_commitment = &.{
            .{ 1496164768, 1205538328, 1402350172, 1785169190, 130019805, 754166036, 1098227100, 15031640 },
        },
        .point_sampling_roots = &.{
            .{ 213411369, 1649647609, 99696598, 558976110, 1675558522, 602421832, 1903538030, 478377460 },
        },
        .point_sample_leaf_idx = 2,
        .point_sample_ext_pairs = &.{
            .{
                .{ 123, 456, 789, 1011, 1213, 1415 },
                .{ 1617, 1819, 2021, 2223, 2425, 2627 },
            },
            .{
                .{ 314, 159, 265, 358, 979, 323 },
                .{ 846, 264, 338, 327, 950, 288 },
            },
        },
        .point_sample_siblings = &.{
            .{ 1107362463, 1573550007, 1342472795, 1303430808, 468458645, 315833478, 1423489390, 1262354139 },
            .{ 1870697469, 753935385, 852744565, 47297598, 402913697, 1585220689, 972363861, 1160947119 },
        },
        .fri_layer_leaf_idx = 2,
        .fri_layer_leaf_p_ext = .{ 1503976375, 1151553836, 672763162, 1077636545, 233055809, 1089562730 },
        .fri_layer_leaf_q_ext = .{ 245641453, 285696038, 1656546867, 1391713972, 1561734495, 1416500325 },
        .fri_layer_siblings = &.{
            .{ 1411898715, 1111733232, 1079677681, 471992148, 1026436490, 1740950843, 756816665, 2036254647 },
            .{ 573244221, 682689094, 1833902962, 1521943939, 976670167, 1115387071, 1897232546, 21417501 },
        },
        .final_poly_ext = &.{
            .{ 15684043, 55433209, 1877003055, 455389992, 1279945421, 1426302445 },
            .{ 166972062, 1761103516, 247589620, 1157881429, 639438401, 1953279724 },
            .{ 252468631, 1670569613, 448585916, 91398980, 1681767884, 981240517 },
            .{ 806334150, 729601507, 1730633585, 1137903628, 1208670272, 1540735504 },
        },
    },
};

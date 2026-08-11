const std = @import("std");
const verifier_ray = @import("verifier_ray");

const rowlimit = verifier_ray.query.rowlimit;

// The golden fixtures already cover honest end-to-end proofs across every
// Lookup/RangeCheckCompiler scenario; these hand-built cases pin the row-limit
// check (and its error path) directly, including the dynamic-module tamper
// scenario prover-ray's own lookup_rowlimit_tamper_test.go guards against: a
// dynamic module's runtime size is prover-declared and unbound by PCS, so this
// check is what stops a prover from claiming a far larger size than it
// compiled against.

const static_a: []const rowlimit.ModuleSize = &[_]rowlimit.ModuleSize{.{ .static = 4 }};
const static_b: []const rowlimit.ModuleSize = &[_]rowlimit.ModuleSize{.{ .static = 4 }};
const dynamic_a: []const rowlimit.ModuleSize = &[_]rowlimit.ModuleSize{.{ .dynamic = 0 }};

fn oneCheckSystem(comptime a_modules: []const rowlimit.ModuleSize, comptime b_modules: []const rowlimit.ModuleSize, comptime limit: u64) rowlimit.System {
    return .{ .checks = &[_]rowlimit.Check{.{ .a_modules = a_modules, .b_modules = b_modules, .limit = limit }} };
}

test "rowlimit accepts a check well under budget" {
    try rowlimit.verify(oneCheckSystem(static_a, static_b, 1 << 30), &.{});
}

test "rowlimit rejects an A side reaching the limit" {
    try std.testing.expectError(
        error.RowLimitExceeded,
        rowlimit.verify(oneCheckSystem(static_a, static_b, 4), &.{}),
    );
}

test "rowlimit rejects a B side reaching the limit" {
    // A side (2) stays under a limit of 4; B side (4) reaches it.
    const small_a: []const rowlimit.ModuleSize = &[_]rowlimit.ModuleSize{.{ .static = 2 }};
    try std.testing.expectError(
        error.RowLimitExceeded,
        rowlimit.verify(oneCheckSystem(small_a, static_b, 4), &.{}),
    );
}

test "rowlimit sums multiple fragments on the same side" {
    const two_fragments: []const rowlimit.ModuleSize = &[_]rowlimit.ModuleSize{ .{ .static = 2 }, .{ .static = 2 } };
    // Each fragment contributes its own pass over the module's rows, so two
    // size-2 fragments sum to 4 rows, not 2 — this must reject a limit of 4
    // (the sum reaches the limit) exactly as two separate size-4 fragments would.
    try std.testing.expectError(
        error.RowLimitExceeded,
        rowlimit.verify(oneCheckSystem(two_fragments, static_b, 4), &.{}),
    );
}

test "rowlimit resolves a dynamic module size from module_sizes" {
    // The dynamic module's honest runtime size (4) is well under the limit.
    try rowlimit.verify(oneCheckSystem(dynamic_a, static_b, 1 << 30), &[_]usize{4});
}

test "rowlimit rejects a dynamic module whose claimed runtime size reaches the limit" {
    // This is the tamper scenario: a prover declares a dynamic module's runtime
    // size as far larger than what it actually compiled against. PCS never binds
    // the claimed size to anything, so this check is the only thing that catches it.
    try std.testing.expectError(
        error.RowLimitExceeded,
        rowlimit.verify(oneCheckSystem(dynamic_a, static_b, 1 << 30), &[_]usize{1 << 30}),
    );
}

test "rowlimit rejects a proof missing a dynamic module's size" {
    try std.testing.expectError(
        error.MissingDynamicModuleSize,
        rowlimit.verify(oneCheckSystem(dynamic_a, static_b, 1 << 30), &.{}),
    );
}

test "rowlimit accepts a system with no checks" {
    try rowlimit.verify(.{}, &.{});
}

//! Canonical column layout for the multi-degree FRI PCS.
//!
//! Ports the verifier-relevant parts of prover-ray `crypto/koalabear/fri`
//! `pcs.go`: `deepEntry`, `sizeBundle`, `layout`, `canonicalLayout`, the
//! per-row shift validation, `batchOrders`, and `maxSizeLog2`.
//!
//! In verifier-ray the batch shapes and shift schedule are FIXED at
//! protocol-compile time, so the whole canonical enumeration — including the
//! alpha_DEEP power schedule that AddOpening/Verify consume — is a comptime
//! value carried inside the compiled PCS `System`. `buildLayout` runs at
//! comptime from a `[]const Shape` + `[]const BatchShifts` and produces the
//! frozen `Layout` the verifier walks.
//!
//! Canonical order (frozen, matches Go): for each native size N = 2^sizeLog2 in
//! DESCENDING order, then for each batch in DECLARATION order, then base rows
//! before ext rows in declaration order. The alpha_DEEP power counter resets to
//! 0 at each new size and increments once per column (all shifts of a column
//! share that power).

const std = @import("std");

pub const Error = error{
    ShapesShiftsLengthMismatch,
    ShapeShiftsRowMismatch,
    NegativeWidth,
    EmptyShiftList,
    DuplicateShift,
    ShiftOutOfRange,
    SizeTooLarge,
};

/// Per-size row counts of one batch's SizedTable. A size is "present" iff
/// base_width + ext_width > 0. Mirrors prover-ray `SizedShape`.
pub const SizedShape = struct {
    base_width: usize = 0,
    ext_width: usize = 0,
};

/// One batch's per-size row counts, indexed by size_log2 (Shape[i] applies to
/// size 2^i). Mirrors prover-ray `Shape`.
pub const Shape = []const SizedShape;

/// Per-row shift schedule for one SizedTable: `base[k]` / `ext[k]` is the shift
/// list for base/ext row k. Mirrors prover-ray `SizedShifts`.
pub const SizedShifts = struct {
    base: []const []const usize = &.{},
    ext: []const []const usize = &.{},
};

/// One batch's per-size shift schedule, indexed by size_log2. Mirrors
/// prover-ray `BatchShifts`.
pub const BatchShifts = []const SizedShifts;

/// One opened column: the (batch, size, row) it comes from, whether it is an
/// ext row, the alpha_DEEP power it consumes, and the rotation shifts it is
/// opened at. Mirrors prover-ray `deepEntry`.
pub const DeepEntry = struct {
    batch_idx: usize,
    size_log2: u8,
    row_idx: usize,
    is_ext: bool,
    alpha_power: usize,
    shifts: []const usize,
};

/// All columns introduced at one native size, plus the distinct batches that
/// contribute (in canonical order), used to route input-tree openings.
/// Mirrors prover-ray `sizeBundle` + the per-bundle `batchOrders` entry.
pub const SizeBundle = struct {
    size_log2: u8,
    entries: []const DeepEntry,
    /// Distinct batch indices contributing to this bundle, in canonical order.
    batch_order: []const usize,
};

/// The frozen canonical layout: size bundles in descending-size order.
pub const Layout = []const SizeBundle;

/// Largest size_log2 present in a layout — the top of the FRI schedule the
/// verifier needs. Ports `layout.maxSizeLog2`.
pub fn maxSizeLog2(layout: Layout) u8 {
    var m: u8 = 0;
    for (layout) |bundle| {
        if (bundle.size_log2 > m) m = bundle.size_log2;
    }
    return m;
}

/// Validate one SizedTable's shift schedule against its shape. Ports
/// `validateSizedLayout` + `validateColumnShifts`.
fn validateSizedLayout(
    comptime shape: SizedShape,
    comptime shifts: SizedShifts,
    comptime size_log2: usize,
) Error!void {
    if (size_log2 > 255) return Error.SizeTooLarge;
    if (shifts.base.len != shape.base_width) return Error.ShapeShiftsRowMismatch;
    if (shifts.ext.len != shape.ext_width) return Error.ShapeShiftsRowMismatch;
    const size = @as(usize, 1) << @intCast(size_log2);
    for (shifts.base) |row_shifts| try validateColumnShifts(row_shifts, size);
    for (shifts.ext) |row_shifts| try validateColumnShifts(row_shifts, size);
}

fn validateColumnShifts(comptime shifts: []const usize, comptime size: usize) Error!void {
    if (shifts.len == 0) return Error.EmptyShiftList;
    for (shifts, 0..) |s, i| {
        if (s >= size) return Error.ShiftOutOfRange;
        for (shifts[0..i]) |prev| {
            if (prev == s) return Error.DuplicateShift;
        }
    }
}

/// Build the canonical layout from batch shapes + shift schedules. Must run at
/// comptime (it materializes comptime-sized slices). Ports `canonicalLayout`
/// followed by `batchOrders`.
pub fn buildLayout(comptime shapes: []const Shape, comptime shifts: []const BatchShifts) Error!Layout {
    comptime {
        if (shapes.len != shifts.len) return Error.ShapesShiftsLengthMismatch;

        // Largest size_log2 across all batches.
        var max_size_log2: isize = -1;
        for (shapes, shifts) |shape, batch_shifts| {
            if (shape.len != batch_shifts.len) return Error.ShapeShiftsRowMismatch;
            if (@as(isize, @intCast(shape.len)) > max_size_log2 + 1) {
                max_size_log2 = @as(isize, @intCast(shape.len)) - 1;
            }
        }

        var bundles: []const SizeBundle = &.{};
        var size_log2: isize = max_size_log2;
        while (size_log2 >= 0) : (size_log2 -= 1) {
            const s: usize = @intCast(size_log2);
            var entries: []const DeepEntry = &.{};
            var batch_order: []const usize = &.{};
            var alpha_power: usize = 0;

            for (shapes, shifts, 0..) |shape, batch_shifts, batch_idx| {
                if (s >= shape.len) continue;
                const sized_shape = shape[s];
                const sized_shifts = batch_shifts[s];
                try validateSizedLayout(sized_shape, sized_shifts, s);

                var contributed = false;
                for (0..sized_shape.base_width) |row_idx| {
                    entries = entries ++ [_]DeepEntry{.{
                        .batch_idx = batch_idx,
                        .size_log2 = @intCast(s),
                        .row_idx = row_idx,
                        .is_ext = false,
                        .alpha_power = alpha_power,
                        .shifts = sized_shifts.base[row_idx],
                    }};
                    alpha_power += 1;
                    contributed = true;
                }
                for (0..sized_shape.ext_width) |row_idx| {
                    entries = entries ++ [_]DeepEntry{.{
                        .batch_idx = batch_idx,
                        .size_log2 = @intCast(s),
                        .row_idx = row_idx,
                        .is_ext = true,
                        .alpha_power = alpha_power,
                        .shifts = sized_shifts.ext[row_idx],
                    }};
                    alpha_power += 1;
                    contributed = true;
                }
                if (contributed) batch_order = batch_order ++ [_]usize{batch_idx};
            }

            if (entries.len > 0) {
                bundles = bundles ++ [_]SizeBundle{.{
                    .size_log2 = @intCast(s),
                    .entries = entries,
                    .batch_order = batch_order,
                }};
            }
        }

        return bundles;
    }
}

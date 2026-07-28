//! DEEP-quotient reconstruction at a single FRI query point.
//!
//! Ports the verifier-relevant parts of prover-ray `crypto/koalabear/fri`
//! `pcs.go`: `shiftedPoint`, `quotientAtValue`, `rowValue`, and
//! `reconstructQueryValueAt`.
//!
//! At a query point x the verifier reconstructs the batched DEEP quotient
//!
//!   F(x) = Σ_i alpha_DEEP^i · Σ_j (f_i(x) - y_ij) / (x - z_ij)
//!
//! where i runs over a size bundle's columns (highest alpha power first, i.e.
//! reverse entry order, so the Horner accumulation lands each column on its
//! declared power), j runs over that column's opened shifts, f_i(x) is the
//! opened row value, y_ij the claimed evaluation, and z_ij = zeta·omega_N^shift
//! the claim point. The accumulation is seeded by `running` (the running FRI
//! codeword value at this round), matching `fri.Level.EvalsAt`.
//!
//! Unlike prover-ray this does per-point `inverse()` rather than a batched
//! Montgomery inversion: the verifier only ever evaluates at a handful of query
//! points, so there is nothing to amortize.

const field = @import("../field/koalabear.zig");
const ext = @import("../field/koalabear_ext.zig");
const layout_mod = @import("layout.zig");
const paired_leaf = @import("paired_leaf.zig");

pub const Error = error{
    ShiftOutOfRange,
    ClaimPointOnQueryPoint,
    BadDomainCardinality,
};

/// One opened claim: the point z = zeta·omega_N^shift and the claimed value y.
pub const Claim = struct {
    point: ext.Ext,
    value: ext.Ext,
};

/// The claim point for a row opened at rotation `shift` of the size-`2^size_log2`
/// domain: `zeta · omega_N^shift`, where omega_N is the canonical
/// `2^size_log2`-th root of unity. Ports `pcs.shiftedPoint`.
pub fn shiftedPoint(size_log2: u8, shift: usize, zeta: ext.Ext) Error!ext.Ext {
    const size = @as(usize, 1) << @intCast(size_log2);
    if (shift >= size) return Error.ShiftOutOfRange;
    if (size_log2 == 0) return zeta; // omega^0 = 1, only shift 0 is valid
    const omega = field.rootOfUnityBy(size) catch return Error.BadDomainCardinality;
    const rotation = omega.pow(@as(u64, shift));
    return zeta.mulByBase(rotation);
}

/// The single-column DEEP quotient at x: Σ_j (value - claim_j.value)/(x - claim_j.point).
/// Ports `pcs.quotientAtValue`.
pub fn quotientAtValue(value: ext.Ext, x: ext.Ext, claims: []const Claim) Error!ext.Ext {
    var res = ext.Ext.zero();
    for (claims) |claim| {
        var denominator = x.sub(claim.point);
        if (denominator.isZero()) return Error.ClaimPointOnQueryPoint;
        denominator = denominator.inverse();
        const numerator = value.sub(claim.value);
        res = res.add(numerator.mul(denominator));
    }
    return res;
}

/// The opened row value for an entry: the base cell lifted, or the ext cell.
/// Ports `pcs.rowValue`.
pub fn rowValueForEntry(row: paired_leaf.RowOpening, entry: layout_mod.DeepEntry) ext.Ext {
    if (entry.is_ext) return row.ext[entry.row_idx];
    return ext.Ext.lift(row.base[entry.row_idx]);
}

/// One entry already resolved for a query point: the opened row value f_i(x)
/// and this column's claims (points + values). The verify layer builds one per
/// bundle entry (selecting self/sibling row, computing claim points from zeta
/// and the entry's shifts) before calling `reconstructQueryValueAt`.
pub const ResolvedEntry = struct {
    row_value: ext.Ext,
    claims: []const Claim,
};

/// Reconstruct the batched DEEP quotient of one size bundle at point `x`,
/// seeded by `running`. `entries` is parallel to `bundle.entries` (index i is
/// the resolution of `bundle.entries[i]`). Walks highest-alpha-power-first
/// (reverse order) so each column lands on its declared alpha power under the
/// Horner accumulation. Ports `pcs.reconstructQueryValueAt`.
pub fn reconstructQueryValueAt(
    entries: []const ResolvedEntry,
    alpha_deep: ext.Ext,
    x: ext.Ext,
    running: ext.Ext,
) Error!ext.Ext {
    var value = running;
    var i = entries.len;
    while (i != 0) {
        i -= 1;
        const term = try quotientAtValue(entries[i].row_value, x, entries[i].claims);
        value = value.mul(alpha_deep).add(term);
    }
    return value;
}

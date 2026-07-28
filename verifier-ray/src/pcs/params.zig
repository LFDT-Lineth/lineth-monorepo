//! FRI parameters and evaluation-domain arithmetic for the verifier.
//!
//! Ports the verifier-relevant parts of prover-ray `crypto/koalabear/fri`
//! `fri.go`: the `Params` shape, `domainPoint`, `numRounds`, and the
//! `restrictTo` schedule offset. Prover-only pieces (fft.Domain, precomputed
//! twiddles, encoders) are omitted — the verifier only needs a domain's
//! generator and the bit-reversed point at a queried position.

const std = @import("std");
const field = @import("../field/koalabear.zig");
const ext = @import("../field/koalabear_ext.zig");

pub const Error = error{
    CodewordSizeTooLarge,
    PlaintextNotSmallerThanCodeword,
    ZeroQueries,
    FinalPolyExceedsPlaintext,
    RestrictOutOfRange,
    BadDomainCardinality,
};

/// Largest supported log2 codeword size. Matches prover-ray's
/// `fri.MaxLogCodewordSize`; also bounded by the KoalaBear two-adicity
/// (`field.max_order_root == 24`), so an FRI domain must satisfy
/// `LogCodewordSize <= 24`.
pub const max_log_codeword_size: u8 = 24;

/// FRI configuration. All fields are the log2 sizes and query count that a
/// protocol fixes at compile time, so a `Params` is a comptime value carried
/// inside a compiled PCS `System`.
pub const Params = struct {
    /// log2 of the codeword (Reed–Solomon evaluation) domain size.
    log_codeword_size: u8,
    /// log2 of the plaintext polynomial size; equals numRounds + logFinalPolySize.
    log_plaintext_size: u8,
    /// number of independent FRI queries (soundness ≈ (1-δ)^num_queries).
    num_queries: usize,
    /// log2 of the final folded polynomial size (0 → fold to a constant).
    log_final_poly_size: u8 = 0,

    /// Number of FRI folding rounds.
    pub fn numRounds(self: Params) u8 {
        return self.log_plaintext_size - self.log_final_poly_size;
    }

    /// Validate the parameter shape. Call once (may run at comptime).
    pub fn validate(self: Params) Error!void {
        if (self.log_codeword_size > max_log_codeword_size) return Error.CodewordSizeTooLarge;
        if (self.log_plaintext_size >= self.log_codeword_size) return Error.PlaintextNotSmallerThanCodeword;
        if (self.num_queries == 0) return Error.ZeroQueries;
        if (self.log_final_poly_size > self.log_plaintext_size) return Error.FinalPolyExceedsPlaintext;
    }

    /// Restrict the FRI schedule to the top sub-domain of plaintext size
    /// 2^top_log2. The codeword shrinks by the same offset the plaintext does,
    /// so the inverse rate (blow-up) is preserved and the fold count becomes
    /// top_log2 - log_final_poly_size. Mirrors prover-ray `Params.restrictTo`.
    pub fn restrictTo(self: Params, top_log2: u8) Error!Params {
        if (top_log2 < self.log_final_poly_size or top_log2 > self.log_plaintext_size) {
            return Error.RestrictOutOfRange;
        }
        const offset = self.log_plaintext_size - top_log2;
        return .{
            .log_codeword_size = self.log_codeword_size - offset,
            .log_plaintext_size = top_log2,
            .num_queries = self.num_queries,
            .log_final_poly_size = self.log_final_poly_size,
        };
    }
};

/// The natural-order exponent for the bit-reversed layer position `pos` in a
/// domain of `log_size` bits: `bitReverse(pos)` truncated to `log_size` bits.
///
/// Codewords are stored in bit-reversed order (see prover-ray RSEncoder), so the
/// domain point at slot `pos` is `generator ^ bitReverse_{log_size}(pos)`.
pub fn bitReversedExponent(pos: usize, log_size: u6) u64 {
    if (log_size == 0) return 0;
    const reversed = @bitReverse(@as(u64, pos));
    return reversed >> @intCast(64 - @as(u32, log_size));
}

/// The domain point at bit-reversed position `pos` of the size-`2^log_size`
/// evaluation domain: `generator ^ bitReversedExponent(pos)`, where `generator`
/// is the canonical `2^log_size`-th root of unity. Ports `fri.domainPoint`.
pub fn domainPoint(log_size: u6, pos: usize) Error!field.Element {
    if (log_size == 0) return field.Element.one();
    const cardinality = @as(usize, 1) << log_size;
    const generator = field.rootOfUnityBy(cardinality) catch return Error.BadDomainCardinality;
    const exponent = bitReversedExponent(pos, log_size);
    return generator.pow(exponent);
}

/// `domainPoint` lifted into the extension field. Ports `fri.domainPointExt`.
pub fn domainPointExt(log_size: u6, pos: usize) Error!ext.Ext {
    return ext.Ext.lift(try domainPoint(log_size, pos));
}

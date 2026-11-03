//! EIP-4844 blob KZG proof verification without recomputing the commitment: the Fiat-Shamir
//! challenge z and the blob polynomial's evaluation y at z are derived from the blob, and the
//! point-evaluation check binds (commitment, z, y) to the proof. sha256 and the check come from
//! the zkvm accelerator interface; the field arithmetic is Constantine's.

const std = @import("std");

const accelerators = @import("zesu_accelerators");
const blob_payload = @import("blob_payload.zig");
const scalar_field = @import("scalar_field.zig");

pub const Fr = scalar_field.Fr;

pub const blob_len = blob_payload.blob_len;
const field_elements = blob_payload.field_elements;

pub const commitment_len = 48;
pub const proof_len = 48;

const challenge_domain = "FSBLOBVERIFY_V1_";

/// `0x01 ‖ sha256(commitment)[1..]`.
pub fn versionedHash(commitment: *const [commitment_len]u8) [32]u8 {
    var hash: [32]u8 = undefined;
    accelerators.sha256(commitment, &hash);
    hash[0] = 0x01;
    return hash;
}

/// The EIP-4844 `compute_challenge`: sha256 over the domain separator, the 16-byte big-endian
/// element count, the blob and the commitment, reduced into the field.
pub fn computeChallenge(blob: *const [blob_len]u8, commitment: *const [commitment_len]u8) Fr {
    var transcript: [challenge_domain.len + 16 + blob_len + commitment_len]u8 = undefined;
    @memcpy(transcript[0..challenge_domain.len], challenge_domain);
    std.mem.writeInt(u128, transcript[challenge_domain.len..][0..16], field_elements, .big);
    @memcpy(transcript[challenge_domain.len + 16 ..][0..blob_len], blob);
    @memcpy(transcript[challenge_domain.len + 16 + blob_len ..], commitment);
    var digest: [32]u8 = undefined;
    accelerators.sha256(&transcript, &digest);
    return Fr.fromBytesReduced(&digest);
}

/// The EIP-4844 primitive root of the scalar field, whose power of (r - 1) / 4096 generates the
/// 4096th roots of unity.
const primitive_root = 7;

/// The canonical big-endian encodings of the step ω = 7^((r - 1) / 4096) between consecutive roots
/// of unity and of 4096^-1.
const root_step_bytes = encodedPower(primitive_root, (scalar_field.modulus - 1) / field_elements);
const inverse_width_bytes = encodedPower(field_elements, scalar_field.modulus - 2);

fn encodedPower(base: u256, exponent: u256) [32]u8 {
    @setEvalBranchQuota(10_000);
    var result: u256 = 1;
    var square = base;
    var rest = exponent;
    while (rest > 0) : (rest >>= 1) {
        if (rest & 1 == 1) result = @intCast(@as(u512, result) * square % scalar_field.modulus);
        square = @intCast(@as(u512, square) * square % scalar_field.modulus);
    }
    var bytes: [32]u8 = undefined;
    std.mem.writeInt(u256, &bytes, result, .big);
    return bytes;
}

/// The EIP-4844 `evaluate_polynomial_in_evaluation_form`: the value at `z` of the polynomial whose
/// evaluations over the bit-reversed 4096th roots of unity are the blob's elements, by the
/// barycentric formula. Null when an element is not a canonical field element.
pub fn evaluate(blob: *const [blob_len]u8, z: Fr) ?Fr {
    for (0..field_elements) |i| {
        if (!Fr.isCanonical(blob[i * 32 ..][0..32])) return null;
    }

    // y = (z^n - 1) / n * sum_k f_k w^k / (z - w^k), for the roots w^k in natural order and f_k
    // the element at their bit-reversed position. The sum is accumulated as one fraction
    // numerator / denominator, so a single inversion remains.
    const root_step = Fr.fromBytes(&root_step_bytes).?;
    var root = Fr.one();
    var numerator = Fr.zero();
    var denominator = Fr.one();
    for (0..field_elements) |k| {
        const index: usize = @bitReverse(@as(u12, @intCast(k)));
        const element = Fr.fromBytes(blob[index * 32 ..][0..32]).?;
        const difference = z.sub(root);
        if (difference.isZero()) return element;
        const term = element.mul(root);
        numerator = numerator.mul(difference).add(term.mul(denominator));
        denominator = denominator.mul(difference);
        root = root.mul(root_step);
    }

    // z^4096 by 12 squarings.
    var z_power = z;
    for (0..std.math.log2_int(usize, field_elements)) |_| z_power.squareInPlace();
    const inverse_width = Fr.fromBytes(&inverse_width_bytes).?;
    return numerator.mul(denominator.inverse()).mul(z_power.sub(Fr.one())).mul(inverse_width);
}

/// Whether `proof` opens `commitment` to the blob's evaluation at the blob's challenge, i.e.
/// whether `commitment` commits to `blob`. A blob with a non-canonical element, an invalid
/// commitment or an invalid proof does not verify.
pub fn verify(
    blob: *const [blob_len]u8,
    commitment: *const [commitment_len]u8,
    proof: *const [proof_len]u8,
) bool {
    const z = computeChallenge(blob, commitment);
    const y = evaluate(blob, z) orelse return false;
    const z_bytes = z.toBytes();
    const y_bytes = y.toBytes();
    var verified = false;
    if (!accelerators.kzg_point_eval(commitment, &z_bytes, &y_bytes, proof, &verified)) return false;
    return verified;
}

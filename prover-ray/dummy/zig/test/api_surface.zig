//! Uses every declaration a guest uses from verifier-ray, the way
//! verifier-ray/src/main.zig uses them. It compiles against the dummy verifier
//! here; any drift in the dummy's API surface breaks this file.

const verifier_ray = @import("verifier_ray");
const riscv_system = @import("riscv_system");

const verifier = verifier_ray.verifier;
const image_relocation = verifier_ray.image_relocation;

/// Length of the pair statement written by `verifyPairImage`.
pub const statement_len = verifier.pairStatementEncodedLen(riscv_system.system_0_public_input.refs.len);

var workspace: verifier.Workspace(riscv_system.system_0_public_input) = undefined;

/// Relocates, validates and verifies a single-proof image encoded for
/// `encoded_base`, in place. Returns 0 on success, 1 on any failure.
pub fn verifyImage(img: [*]u8, img_len: usize, encoded_base: usize) u8 {
    const mapped = @intFromPtr(img);
    image_relocation.rebase(img, img_len, encoded_base, mapped);
    image_relocation.validateImage(img, img_len, mapped) catch return 1;
    const input: *const verifier.VerifyInput = @ptrCast(@alignCast(img));
    verifier.verifyWithWorkspace(
        riscv_system.system_0_spec,
        riscv_system.system_0_systems,
        input.proof,
        input.public_inputs,
        &workspace,
    ) catch return 1;
    return 0;
}

/// Relocates, validates and verifies a pair image encoded for `encoded_base`,
/// in place, and writes the pair statement to `out`. Returns 0 on success, 1
/// on any failure.
pub fn verifyPairImage(img: [*]u8, img_len: usize, encoded_base: usize, out: *[statement_len]u8) u8 {
    const mapped = @intFromPtr(img);
    image_relocation.rebasePair(img, img_len, encoded_base, mapped) catch return 1;
    const pair_offsets = image_relocation.validatePair(img, img_len, mapped) catch return 1;
    const pair = verifier.AggregatorInput{
        .a = @ptrFromInt(mapped + pair_offsets.a),
        .b = @ptrFromInt(mapped + pair_offsets.b),
    };
    verifier.verifyPairWithWorkspace(
        riscv_system.system_0_spec,
        riscv_system.system_0_systems,
        pair.a.*,
        pair.b.*,
        &workspace,
    ) catch return 1;
    verifier.encodePairStatement(pair.a.public_inputs, out) catch return 1;
    return 0;
}

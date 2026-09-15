//! FFI tests for the Constantine-backed guest crypto bindings.
//! Vectors cover EIP-2537, EIP-4844, and secp256k1 recovery.

const std = @import("std");
const gc = @import("guest_crypto");

const Bls12G1MsmPair = extern struct { point: [96]u8, scalar: [32]u8 };
const Bls12G2MsmPair = extern struct { point: [192]u8, scalar: [32]u8 };
const Bls12PairingPair = extern struct { g1: [96]u8, g2: [192]u8 };
const Bn254PairingPair = extern struct { g1: [64]u8, g2: [128]u8 };

fn hexArr(comptime n: usize, hex: []const u8) [n]u8 {
    var out: [n]u8 = undefined;
    _ = std.fmt.hexToBytes(&out, hex) catch unreachable;
    return out;
}

// Decodes a byte window from a CSV column.
fn hexField(comptime n: usize, comptime hex: []const u8, comptime offset: usize) [n]u8 {
    return hexArr(n, hex[offset * 2 ..][0 .. n * 2]);
}

// Strips the 16-byte limb padding from EIP-2537 coordinates.
fn stripFp(comptime n: usize, comptime wire: []const u8, comptime off: usize) [n]u8 {
    return hexArr(n, wire[(off + 16) * 2 ..][0 .. n * 2]);
}

fn stripG1(comptime wire: []const u8, comptime off: usize) [96]u8 {
    return stripFp(48, wire, off) ++ stripFp(48, wire, off + 64);
}

fn stripG2(comptime wire: []const u8, comptime off: usize) [192]u8 {
    return stripFp(48, wire, off) ++ stripFp(48, wire, off + 64) ++
        stripFp(48, wire, off + 128) ++ stripFp(48, wire, off + 192);
}

const CsvRow = struct { input: []const u8, result: []const u8 };

// Selects a success row with the requested input and result widths.
fn csvRow(comptime csv: []const u8, comptime skip: usize, comptime input_hex: usize, comptime result_hex: usize) CsvRow {
    @setEvalBranchQuota(1_000_000);
    var it = std.mem.splitScalar(u8, csv, '\n');
    _ = it.next(); // header
    var i: usize = 0;
    while (it.next()) |line| {
        if (line.len == 0) continue;
        var cols = std.mem.splitScalar(u8, line, ',');
        const input = cols.next().?;
        const result = cols.next().?;
        _ = cols.next(); // gas
        const notes = std.mem.trim(u8, cols.rest(), " \r");
        if (notes.len != 0) continue; // expect-reject row
        if (input.len != input_hex or result.len != result_hex) continue;
        if (i < skip) {
            i += 1;
            continue;
        }
        return .{ .input = input, .result = result };
    }
    @compileError("no matching success row in CSV");
}

// EIP-2537 vectors are comptime-parsed from CSV fixtures.

const eip2537_g1_add_csv = @embedFile("eip2537_g1_add_csv");
const eip2537_g2_add_csv = @embedFile("eip2537_g2_add_csv");
const eip2537_pairing_csv = @embedFile("eip2537_pairing_csv");

const g1_add_a = stripG1(csvRow(eip2537_g1_add_csv, 0, 512, 256).input, 0);

test "g1 add: known vector" {
    const row = comptime csvRow(eip2537_g1_add_csv, 0, 512, 256);
    const b = stripG1(row.input, 128);
    const expected = stripG1(row.result, 0);
    var out: [96]u8 = undefined;
    try std.testing.expect(gc.g1Add(&g1_add_a, &b, &out));
    try std.testing.expectEqualSlices(u8, &expected, &out);
}

test "g1 add: identity handling (all-zero point)" {
    const zero: [96]u8 = .{0} ** 96;
    var out: [96]u8 = undefined;
    try std.testing.expect(gc.g1Add(&g1_add_a, &zero, &out));
    try std.testing.expectEqualSlices(u8, &g1_add_a, &out);
}

test "g1 add: rejects non-canonical Fp (>= p) and off-curve points" {
    var bad: [96]u8 = .{0xFF} ** 96;
    var out: [96]u8 = undefined;
    try std.testing.expect(!gc.g1Add(&bad, &bad, &out));
    bad = .{0} ** 96;
    bad[47] = 1; // x = 1, y = 0 — off-curve
    try std.testing.expect(!gc.g1Add(&bad, &bad, &out));
}

// MSM vectors come from go-ethereum testdata; the two-pair cases stay inline.
test "g1 msm: official two-pair vector" {
    const raw = hexArr(256, "044c8141f453400b5eaecd905a163ff950775a1a147e68eaccff25dff1d77c0fe9b2abf8311cf04993a615c1209a2a20" ++
        "0e6728c19f90dcbb3477112effe8bc4d65eff34814c2945170c7843d72702b90a1d97adc9a1a857e95f69a9ce56d2d4f" ++
        "1824b159acc5056f998c4fefecbc4ff55884b7fa0003480200000001fffffffd15011119f24cc9325aa4b578d9fa3430" ++
        "ccba523ca0bf0359b221b172afa32a8c80135bce8e04cb774f3cd04999409cff12978dda7d55bca7498a4c797bec5d17" ++
        "0cbc90f739b1e20985262d422ea154ff35cf5c71bc23791efb5148fbdc47d63f1824b159acc5056f998c4fefecbc4ff5" ++
        "5884b7fa0003480200000001fffffffd");
    var pairs: [2]Bls12G1MsmPair = undefined;
    @memcpy(std.mem.asBytes(&pairs), &raw);
    const expected = hexArr(96, "179c5193a1eec7522458270c65d4fefe0cf72774498687925a47b21e7060975d02de34832fb709ae8ce98b396a9b8eb0" ++
        "04d522468140afde2f3d40841e020b0b240a817ff4ee1cfd13c62497f0d9446132dd5c0f36ba226a397721ba4a6e7be6");
    var out: [96]u8 = undefined;
    try std.testing.expect(gc.g1Msm(&pairs, &out));
    try std.testing.expectEqualSlices(u8, &expected, &out);
}

test "g1 msm: empty input fails" {
    const pairs = [_]Bls12G1MsmPair{};
    var out: [96]u8 = undefined;
    try std.testing.expect(!gc.g1Msm(&pairs, &out));
}

test "g2 add: known vector" {
    const row = comptime csvRow(eip2537_g2_add_csv, 0, 1024, 512);
    const a = stripG2(row.input, 0);
    const b = stripG2(row.input, 256);
    const expected = stripG2(row.result, 0);
    var out: [192]u8 = undefined;
    try std.testing.expect(gc.g2Add(&a, &b, &out));
    try std.testing.expectEqualSlices(u8, &expected, &out);
}

test "g2 msm: official vector including an infinity point" {
    const raw = hexArr(448, "024aa2b2f08f0a91260805272dc51051c6e47ad4fa403b02b4510b647ae3d1770bac0326a805bbefd48056c8c121bdb8" ++
        "13e02b6052719f607dacd3a088274f65596bd0d09920b61ab5da61bbdc7f5049334cf11213945d57e5ac7d055d042b7e" ++
        "0ce5d527727d6e118cc9cdc6da2e351aadfd9baa8cbdd3a76d429a695160d12c923ac9cc3baca289e193548608b82801" ++
        "0606c4a02ea734cc32acd2b02bc28b99cb3e287e85a763af267492ab572e99ab3f370d275cec1da1aaa9075ff05f79be" ++
        "000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000" ++
        "000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000" ++
        "000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000" ++
        "000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000" ++
        "000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000" ++
        "00000000000000000000000000000002");
    var pairs: [2]Bls12G2MsmPair = undefined;
    @memcpy(std.mem.asBytes(&pairs), &raw);
    const expected = hexArr(192, "1638533957d540a9d2370f17cc7ed5863bc0b995b8825e0ee1ea1e1e4d00dbae81f14b0bf3611b78c952aacab827a053" ++
        "0a4edef9c1ed7f729f520e47730a124fd70662a904ba1074728114d1031e1572c6c886f6b57ec72a6178288c47c33577" ++
        "0468fb440d82b0630aeb8dca2b5256789a66da69bf91009cbfe6bd221e47aa8ae88dece9764bf3bd999d95d71e4c9899" ++
        "0f6d4552fa65dd2638b361543f887136a43253d9c66c411697003f7a13c308f5422e1aa0a59c8967acdefd8b6e36ccf3");
    var out: [192]u8 = undefined;
    try std.testing.expect(gc.g2Msm(&pairs, &out));
    try std.testing.expectEqualSlices(u8, &expected, &out);
}

// Decodes padded EIP-2537 pairing rows into the wrapper's raw layout.
fn pairingPairs(comptime n: usize, comptime wire: []const u8) [n]Bls12PairingPair {
    var out: [n]Bls12PairingPair = undefined;
    inline for (0..n) |i| {
        out[i] = .{ .g1 = stripG1(wire, i * 384), .g2 = stripG2(wire, i * 384 + 128) };
    }
    return out;
}

test "pairing: official vectors (verifying and non-verifying) and empty input" {
    // Rows 0 and 16 have pairing verdicts 1 and 0.
    const true_row = comptime csvRow(eip2537_pairing_csv, 0, 1536, 64);
    const false_row = comptime csvRow(eip2537_pairing_csv, 16, 1536, 64);
    comptime {
        std.debug.assert(true_row.result[63] == '1');
        std.debug.assert(false_row.result[63] == '0');
    }
    var verified = false;

    const true_pairs = pairingPairs(2, true_row.input);
    try std.testing.expect(gc.pairingCheck(&true_pairs, &verified));
    try std.testing.expect(verified);

    const false_pairs = pairingPairs(2, false_row.input);
    try std.testing.expect(gc.pairingCheck(&false_pairs, &verified));
    try std.testing.expect(!verified);

    const empty = [_]Bls12PairingPair{};
    verified = false;
    try std.testing.expect(gc.pairingCheck(&empty, &verified));
    try std.testing.expect(verified);
}

const eip2537_fp_to_g1_csv = @embedFile("eip2537_fp_to_g1_csv");
const eip2537_fp2_to_g2_csv = @embedFile("eip2537_fp2_to_g2_csv");

// The first two fp_to_g1 rows reuse the first fp2_to_g2 row's coordinates.
const map_row0 = csvRow(eip2537_fp_to_g1_csv, 0, 128, 256);
const map_row1 = csvRow(eip2537_fp_to_g1_csv, 1, 128, 256);
const map_row2 = csvRow(eip2537_fp2_to_g2_csv, 0, 256, 512);
comptime {
    std.debug.assert(std.mem.eql(u8, map_row0.input[32..], map_row2.input[32..128]));
    std.debug.assert(std.mem.eql(u8, map_row1.input[32..], map_row2.input[160..]));
}

test "map fp to g1: official vectors" {
    var out: [96]u8 = undefined;
    try std.testing.expect(gc.mapFpToG1(&stripFp(48, map_row0.input, 0), &out));
    try std.testing.expectEqualSlices(u8, &stripG1(map_row0.result, 0), &out);
    try std.testing.expect(gc.mapFpToG1(&stripFp(48, map_row1.input, 0), &out));
    try std.testing.expectEqualSlices(u8, &stripG1(map_row1.result, 0), &out);
}

test "map fp2 to g2: official vector" {
    const fe = stripFp(48, map_row2.input, 0) ++ stripFp(48, map_row2.input, 64);
    const expected = stripG2(map_row2.result, 0);
    var out: [192]u8 = undefined;
    try std.testing.expect(gc.mapFp2ToG2(&fe, &out));
    try std.testing.expectEqualSlices(u8, &expected, &out);
}

test "map rejects non-canonical field element" {
    const fe: [48]u8 = .{0xFF} ** 48;
    var out: [96]u8 = undefined;
    try std.testing.expect(!gc.mapFpToG1(&fe, &out));
}

// Official go-ethereum point-evaluation vector, split into the wrapper's four arguments.
const kzg_commitment = hexArr(48, "8f59a8d2a1a625a17f3fea0fe5eb8c896db3764f3185481bc22f91b4aaffcca25f26936857bc3a7c2539ea8ec3a952b7");
const kzg_z = hexArr(32, "564c0a11a0f704f4fc3e8acfe0f8245f0ad1347b378fbf96e206da11a5d36306");
const kzg_y = hexArr(32, "24d25032e67a7e6a4910df5834b8fe70e6bcfeeac0352434196bdf4b2485d5a1");
const kzg_proof = hexArr(48, "873033e038326e87ed3e1276fd140253fa08e9fc25fb2d9a98527fc22a2c9612fbeafdad446cbc7bcdbdcd780af2c16a");

test "kzg point evaluation: official vector verifies" {
    try std.testing.expect(gc.kzgPointEvalVerify(&kzg_commitment, &kzg_z, &kzg_y, &kzg_proof));
}

test "kzg point evaluation: tampered claimed value fails verification" {
    var y_bad = kzg_y;
    y_bad[31] ^= 0x01;
    try std.testing.expect(!gc.kzgPointEvalVerify(&kzg_commitment, &kzg_z, &y_bad, &kzg_proof));
}

const Secp256k1 = std.crypto.ecc.Secp256k1;
const Scalar = Secp256k1.scalar.Scalar;

fn scalarFromU64(v: u64) Scalar {
    var b = [_]u8{0} ** 32;
    std.mem.writeInt(u64, b[24..32], v, .big);
    return Scalar.fromBytes(b, .big) catch unreachable;
}

/// Textbook ECDSA sign: R = k·G, r = R.x mod n, s = k⁻¹(z + r·d), recid = parity of R.y.
fn signOnce(d: Scalar, k: Scalar, z: Scalar) ?struct { sig: [64]u8, recid: u8 } {
    const R = Secp256k1.basePoint.mul(k.toBytes(.little), .little) catch return null;
    const aff = R.affineCoordinates();
    const r = Scalar.fromBytes(aff.x.toBytes(.big), .big) catch return null; // skip R.x ≥ n
    if (r.isZero()) return null;
    const s = k.invert().mul(z.add(r.mul(d)));
    if (s.isZero()) return null;
    var sig: [64]u8 = undefined;
    sig[0..32].* = r.toBytes(.big);
    sig[32..].* = s.toBytes(.big);
    return .{ .sig = sig, .recid = if (aff.y.isOdd()) 1 else 0 };
}

test "ecrecover round-trips signatures back to the signing key, and verify accepts them" {
    const keys = [_]u64{ 1, 2, 0xDEAD_BEEF, 0x0123_4567_89AB_CDEF, 0xFFFF_FFFF_FFFF_FFFF };
    const msgs = [_]u64{ 0xABC, 0x9999, 1, 0x0BAD_C0DE, 0xFEED_FACE_CAFE_BEEF };

    for (keys, msgs) |dv, zv| {
        const d = scalarFromU64(dv);
        const z = scalarFromU64(zv);

        const P = try Secp256k1.basePoint.mul(d.toBytes(.little), .little);
        const Paff = P.affineCoordinates();
        var expected: [64]u8 = undefined;
        expected[0..32].* = Paff.x.toBytes(.big);
        expected[32..].* = Paff.y.toBytes(.big);

        var k = scalarFromU64(2);
        var attempts: usize = 0;
        const found = while (attempts < 256) : (attempts += 1) {
            if (signOnce(d, k, z)) |sg| break sg;
            k = k.add(Scalar.one);
        } else null;
        try std.testing.expect(found != null);

        const zb = z.toBytes(.big);
        var out: [64]u8 = undefined;
        try std.testing.expect(gc.ecrecover(&zb, &found.?.sig, found.?.recid, &out));
        try std.testing.expectEqualSlices(u8, &expected, &out);

        var verified = false;
        gc.secp256k1Verify(&zb, &found.?.sig, &expected, &verified);
        try std.testing.expect(verified);

        var wrong_msg = zb;
        wrong_msg[0] ^= 0x55;
        gc.secp256k1Verify(&wrong_msg, &found.?.sig, &expected, &verified);
        try std.testing.expect(!verified);
    }
}

test "ecrecover rejects malformed signatures" {
    const z = scalarFromU64(0x1234).toBytes(.big);
    var out: [64]u8 = undefined;

    // r = 0 and s = 0 are invalid.
    try std.testing.expect(!gc.ecrecover(&z, &([_]u8{0} ** 64), 0, &out));

    // r ≥ n (all 0xFF) is non-canonical.
    var sig_bad_r: [64]u8 = undefined;
    sig_bad_r[0..32].* = [_]u8{0xFF} ** 32;
    sig_bad_r[32..].* = scalarFromU64(9).toBytes(.big);
    try std.testing.expect(!gc.ecrecover(&z, &sig_bad_r, 0, &out));

    // Recovery ids outside {0, 1} are invalid.
    var sig_ok: [64]u8 = undefined;
    sig_ok[0..32].* = scalarFromU64(7).toBytes(.big);
    sig_ok[32..].* = scalarFromU64(9).toBytes(.big);
    try std.testing.expect(!gc.ecrecover(&z, &sig_ok, 2, &out));
}

// BN254 smoke tests pin the raw EIP-196 layout and return contract.

const eip196_g1_add_csv = @embedFile("eip196_g1_add_csv");
const eip196_g1_mul_csv = @embedFile("eip196_g1_mul_csv");
const eip196_pairing_csv = @embedFile("eip196_pairing_csv");

test "bn254 g1 add: known vector" {
    const row = comptime csvRow(eip196_g1_add_csv, 0, 256, 128);
    const p1 = hexField(64, row.input, 0);
    const p2 = hexField(64, row.input, 64);
    const expected = hexField(64, row.result, 0);
    var out: [64]u8 = undefined;
    try std.testing.expect(gc.bn254G1Add(&p1, &p2, &out));
    try std.testing.expectEqualSlices(u8, &expected, &out);
}

test "bn254 g1 add: rejects off-curve point" {
    // (1,1) is in range but off curve. All-zero encodes the point at infinity.
    const bad = hexArr(64, "00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000001");
    const zero: [64]u8 = .{0} ** 64;
    var out: [64]u8 = undefined;
    try std.testing.expect(!gc.bn254G1Add(&bad, &zero, &out));
    try std.testing.expect(gc.bn254G1Add(&zero, &zero, &out));
}

test "bn254 g1 mul: known vector" {
    const row = comptime csvRow(eip196_g1_mul_csv, 0, 192, 128);
    const point = hexField(64, row.input, 0);
    const scalar = hexField(32, row.input, 64);
    const expected = hexField(64, row.result, 0);
    var out: [64]u8 = undefined;
    try std.testing.expect(gc.bn254G1Mul(&point, &scalar, &out));
    try std.testing.expectEqualSlices(u8, &expected, &out);
}

test "bn254 pairing: verifying and non-verifying" {
    // Rows 0 and 3 have pairing verdicts 1 and 0.
    const true_row = comptime csvRow(eip196_pairing_csv, 0, 768, 64);
    const false_row = comptime csvRow(eip196_pairing_csv, 3, 768, 64);
    comptime {
        std.debug.assert(true_row.result[63] == '1');
        std.debug.assert(false_row.result[63] == '0');
    }

    var pairs: [2]Bn254PairingPair = undefined;
    var verified = false;

    @memcpy(std.mem.asBytes(&pairs), &hexField(384, true_row.input, 0));
    try std.testing.expect(gc.bn254PairingCheck(&pairs, &verified));
    try std.testing.expect(verified);

    @memcpy(std.mem.asBytes(&pairs), &hexField(384, false_row.input, 0));
    try std.testing.expect(gc.bn254PairingCheck(&pairs, &verified));
    try std.testing.expect(!verified);

    const empty = [_]Bn254PairingPair{};
    verified = false;
    try std.testing.expect(gc.bn254PairingCheck(&empty, &verified));
    try std.testing.expect(verified);
}

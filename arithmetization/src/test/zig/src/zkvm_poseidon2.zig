const lineth_accel = @import("lineth_zkvm_accel");

// Number of field elements in a Poseidon2 state (the zkvm_bytes_64 state holds
// STATE_WIDTH little-endian 32-bit felt words).
const STATE_WIDTH = @sizeOf(lineth_accel.zkvm_bytes_64) / @sizeOf(u32);

// Applies the Poseidon2 permutation to `input` and compares the result against
// `expected`, lane by lane. On any mismatch the program halts with exit code
// `fail_code` (a per-case identifier); on success it returns so the caller can
// continue to the next case. A passing run reaches `wrappers.zkvm_exit(0)`.
//
// Expected outputs match the verified zkc fixtures in
// arithmetization/src/test/zkc/poseidon2/permutation.accepts and were
// cross-checked against the gnark-crypto / verifier-ray references.
fn check(input: [STATE_WIDTH]u32, expected: [STATE_WIDTH]u32, fail_code: u32) void {
    // The state is passed as zkvm_bytes_64; on this little-endian target the
    // bytes of a [16]u32 array are exactly the 16 little-endian felt words.
    var in_state: lineth_accel.zkvm_bytes_64 = .{ .data = @bitCast(input) };
    var out_state: lineth_accel.zkvm_bytes_64 = undefined;

    _ = lineth_accel.lineth_zkvm_poseidon2_permutation(&in_state, &out_state);

    const got: [STATE_WIDTH]u32 = @bitCast(out_state.data);
    for (got, expected) |got_lane, want_lane| {
        if (got_lane != want_lane) {
            lineth_accel.zkvm_exit(fail_code);
        }
    }
}

export fn main() noreturn {
    // case 1: zero
    check([_]u32{ 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0 }, [_]u32{ 895050135, 581962365, 1304751820, 330596323, 1247898963, 210658690, 1822445167, 1120256524, 106343301, 994005636, 56797207, 2116123491, 205490027, 300435510, 1363520036, 570836988 }, 1);
    // case 2: sequential 0..15
    check([_]u32{ 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15 }, [_]u32{ 1787013330, 260359776, 1416256036, 877206309, 1055246032, 1286424965, 1454862201, 1305538460, 932254705, 1939458209, 918369280, 1249949503, 673662315, 1800209043, 549432727, 1013224035 }, 2);
    // case 3: all ones
    check([_]u32{ 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1 }, [_]u32{ 723566910, 1016991109, 863411560, 1911294016, 1425870610, 725387529, 1725353406, 1588265133, 1236438539, 1081427646, 1748254504, 741848887, 1371133995, 1588717445, 1684567106, 2083239366 }, 3);
    // case 4: input lanes >= the KoalaBear modulus (exercises the reduction path)
    check([_]u32{ 2130706433, 2130706434, 4294967295, 2147483648, 2130706431, 2130706432, 2130706433, 2130706434, 4294967295, 2130706433, 0, 1, 2, 3, 4, 5 }, [_]u32{ 1627113515, 1366436550, 850260409, 1321578390, 2084501578, 896126233, 711499229, 1081742586, 710081214, 1196985225, 1835359965, 30926083, 321918783, 556146250, 380073118, 152525150 }, 4);
    // case 5: random 1
    check([_]u32{ 305419896, 2596069104, 591751049, 1985229328, 4275878552, 19088743, 2309737967, 1432778632, 1985229328, 591751049, 2596069104, 305419896, 3735928559, 195951310, 2309737967, 4275878552 }, [_]u32{ 580288816, 181857902, 1928576042, 402195196, 1262829930, 1273377661, 2039768798, 1521594546, 2059807884, 914810886, 164661647, 1331022192, 834764301, 586742101, 740571318, 685847116 }, 5);
    // case 6: random 2
    check([_]u32{ 2130706432, 2130706431, 1, 2, 1065353216, 1065353217, 3, 4, 2122383361, 1864368129, 2130706306, 8323072, 266338304, 133169152, 127, 1000000007 }, [_]u32{ 531247973, 1889552100, 357166750, 26070520, 1095542703, 259894082, 811856935, 503760263, 296235402, 619227490, 1271674584, 645125619, 836680447, 202813437, 869488616, 1543260919 }, 6);

    lineth_accel.zkvm_exit(0);
}

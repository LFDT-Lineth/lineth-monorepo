const lineth_accel = @import("lineth_zkvm_accel");

export fn main() noreturn {
    const msg = lineth_accel.zkvm_secp256k1_hash{ .data = [_]u8{0} ** 32 };
    const sig = lineth_accel.zkvm_secp256k1_signature{ .data = [_]u8{0} ** 64 };
    const pubkey = lineth_accel.zkvm_secp256k1_pubkey{ .data = [_]u8{0} ** 64 };
    var verified = false;

    _ = lineth_accel.zkvm_secp256k1_verify(&msg, &sig, &pubkey, &verified);

    lineth_accel.zkvm_exit(0);
}

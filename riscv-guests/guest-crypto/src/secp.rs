//! secp256k1 ECDSA with EVM semantics: compact r ‖ s signatures (32 bytes each, big-endian),
//! 64-byte uncompressed public keys (x ‖ y, no SEC1 prefix), 32-byte pre-hashed messages, and
//! recovery ids in {0, 1}. High-s signatures are accepted (EVM ecrecover imposes no low-s rule);
//! they are normalized before hitting `k256`, flipping the recovery id to preserve the recovered
//! key.

use k256::ecdsa::signature::hazmat::PrehashVerifier;
use k256::ecdsa::{RecoveryId, Signature, VerifyingKey};

pub fn ecrecover(msg: &[u8; 32], sig: &[u8; 64], recid: u8, output: &mut [u8; 64]) -> bool {
    if recid > 1 {
        return false;
    }
    let Ok(mut signature) = Signature::from_slice(sig) else {
        return false;
    };
    let mut recid = recid;
    if let Some(normalized) = signature.normalize_s() {
        signature = normalized;
        recid ^= 1;
    }
    let Some(recovery_id) = RecoveryId::from_byte(recid) else {
        return false;
    };
    let Ok(key) = VerifyingKey::recover_from_prehash(msg, &signature, recovery_id) else {
        return false;
    };
    let point = key.to_encoded_point(false);
    output.copy_from_slice(&point.as_bytes()[1..65]);
    true
}

pub fn verify(msg: &[u8; 32], sig: &[u8; 64], pubkey: &[u8; 64]) -> bool {
    let mut sec1 = [0u8; 65];
    sec1[0] = 0x04;
    sec1[1..].copy_from_slice(pubkey);
    let Ok(key) = VerifyingKey::from_sec1_bytes(&sec1) else {
        return false;
    };
    let Ok(signature) = Signature::from_slice(sig) else {
        return false;
    };
    // (r, n−s) verifies iff (r, s) does, so normalizing preserves the verdict while satisfying
    // k256's low-s verification policy.
    let signature = signature.normalize_s().unwrap_or(signature);
    key.verify_prehash(msg, &signature).is_ok()
}

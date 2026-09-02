//! Vector tests: official EIP-2537 vectors (via go-ethereum's precompile testdata), the EIP-4844
//! point-evaluation KAT, EVM ecrecover vectors, and the `[s]₂` pin. The BLS vectors run through
//! the extern C entry points, exercising the same code path the guest links.

use super::*;
use crate::{kzg, secp};
use ark_ff::PrimeField;

fn h(s: &str) -> alloc::vec::Vec<u8> {
    hex::decode(s).unwrap()
}

fn arr<const N: usize>(s: &str) -> [u8; N] {
    h(s).try_into().unwrap()
}

fn fq_to_be(x: &ark_bls12_381::Fq, out: &mut [u8; 48]) {
    let limbs = x.into_bigint().0;
    for (i, limb) in limbs.iter().enumerate() {
        let hi = 48 - 8 * i;
        out[hi - 8..hi].copy_from_slice(&limb.to_be_bytes());
    }
}

fn g1_add(a: &[u8; 96], b: &[u8; 96], out: &mut [u8; 96]) -> bool {
    unsafe { guest_crypto_bls12_g1_add(a, b, out) == OK }
}

fn g2_add(a: &[u8; 192], b: &[u8; 192], out: &mut [u8; 192]) -> bool {
    unsafe { guest_crypto_bls12_g2_add(a, b, out) == OK }
}

fn g1_msm(pairs: &[u8], out: &mut [u8; 96]) -> bool {
    unsafe { guest_crypto_bls12_g1_msm(pairs.as_ptr(), pairs.len() / G1_MSM_PAIR_LEN, out) == OK }
}

fn g2_msm(pairs: &[u8], out: &mut [u8; 192]) -> bool {
    unsafe { guest_crypto_bls12_g2_msm(pairs.as_ptr(), pairs.len() / G2_MSM_PAIR_LEN, out) == OK }
}

fn pairing_check(pairs: &[u8], verified: &mut bool) -> bool {
    unsafe {
        guest_crypto_bls12_pairing(pairs.as_ptr(), pairs.len() / PAIRING_PAIR_LEN, verified) == OK
    }
}

fn map_fp_to_g1(fe: &[u8; 48], out: &mut [u8; 96]) -> bool {
    unsafe { guest_crypto_bls12_map_fp_to_g1(fe, out) == OK }
}

fn map_fp2_to_g2(fe: &[u8; 96], out: &mut [u8; 192]) -> bool {
    unsafe { guest_crypto_bls12_map_fp2_to_g2(fe, out) == OK }
}

#[test]
fn g1_add_official_vector() {
    let a: [u8; 96] = arr("0572cbea904d67468808c8eb50a9450c9721db309128012543902d0ac358a62ae28f75bb8f1c7c42c39a8c5529bf0f4e166a9d8cabc673a322fda673779d8e3822ba3ecb8670e461f73bb9021d5fd76a4c56d9d4cd16bd1bba86881979749d28");
    let b: [u8; 96] = arr("09ece308f9d1f0131765212deca99697b112d61f9be9a5f1f3780a51335b3ff981747a0b2ca2179b96d2c0c9024e5224032b80d3a6f5b09f8a84623389c5f80ca69a0cddabc3097f9d9c27310fd43be6e745256c634af45ca3473b0590ae30d1");
    let mut out = [0u8; 96];
    assert!(g1_add(&a, &b, &mut out));
    assert_eq!(out.to_vec(), h("10e7791fb972fe014159aa33a98622da3cdc98ff707965e536d8636b5fcc5ac7a91a8c46e59a00dca575af0f18fb13dc16ba437edcc6551e30c10512367494bfb6b01cc6681e8a4c3cd2501832ab5c4abc40b4578b85cbaffbf0bcd70d67c6e2"));
}

#[test]
fn g1_add_identity_and_rejections() {
    let a: [u8; 96] = arr("0572cbea904d67468808c8eb50a9450c9721db309128012543902d0ac358a62ae28f75bb8f1c7c42c39a8c5529bf0f4e166a9d8cabc673a322fda673779d8e3822ba3ecb8670e461f73bb9021d5fd76a4c56d9d4cd16bd1bba86881979749d28");
    let zero = [0u8; 96];
    let mut out = [0u8; 96];
    assert!(g1_add(&a, &zero, &mut out));
    assert_eq!(out, a);
    let bad = [0xffu8; 96];
    assert!(!g1_add(&bad, &bad, &mut out));
    let mut off_curve = [0u8; 96];
    off_curve[47] = 1;
    assert!(!g1_add(&off_curve, &off_curve, &mut out));
}

#[test]
fn g1_msm_official_vector() {
    let pairs = h("044c8141f453400b5eaecd905a163ff950775a1a147e68eaccff25dff1d77c0fe9b2abf8311cf04993a615c1209a2a200e6728c19f90dcbb3477112effe8bc4d65eff34814c2945170c7843d72702b90a1d97adc9a1a857e95f69a9ce56d2d4f1824b159acc5056f998c4fefecbc4ff55884b7fa0003480200000001fffffffd15011119f24cc9325aa4b578d9fa3430ccba523ca0bf0359b221b172afa32a8c80135bce8e04cb774f3cd04999409cff12978dda7d55bca7498a4c797bec5d170cbc90f739b1e20985262d422ea154ff35cf5c71bc23791efb5148fbdc47d63f1824b159acc5056f998c4fefecbc4ff55884b7fa0003480200000001fffffffd");
    let mut out = [0u8; 96];
    assert!(g1_msm(&pairs, &mut out));
    assert_eq!(out.to_vec(), h("179c5193a1eec7522458270c65d4fefe0cf72774498687925a47b21e7060975d02de34832fb709ae8ce98b396a9b8eb004d522468140afde2f3d40841e020b0b240a817ff4ee1cfd13c62497f0d9446132dd5c0f36ba226a397721ba4a6e7be6"));
    assert!(!g1_msm(&[], &mut out));
}

#[test]
fn g2_add_official_vector() {
    let a: [u8; 192] = arr("1638533957d540a9d2370f17cc7ed5863bc0b995b8825e0ee1ea1e1e4d00dbae81f14b0bf3611b78c952aacab827a0530a4edef9c1ed7f729f520e47730a124fd70662a904ba1074728114d1031e1572c6c886f6b57ec72a6178288c47c335770468fb440d82b0630aeb8dca2b5256789a66da69bf91009cbfe6bd221e47aa8ae88dece9764bf3bd999d95d71e4c98990f6d4552fa65dd2638b361543f887136a43253d9c66c411697003f7a13c308f5422e1aa0a59c8967acdefd8b6e36ccf3");
    let b: [u8; 192] = arr("122915c824a0857e2ee414a3dccb23ae691ae54329781315a0c75df1c04d6d7a50a030fc866f09d516020ef82324afae09380275bbc8e5dcea7dc4dd7e0550ff2ac480905396eda55062650f8d251c96eb480673937cc6d9d6a44aaa56ca66dc0b21da7955969e61010c7a1abc1a6f0136961d1e3b20b1a7326ac738fef5c721479dfd948b52fdf2455e44813ecfd89208f239ba329b3967fe48d718a36cfe5f62a7e42e0bf1c1ed714150a166bfbd6bcf6b3b58b975b9edea56d53f23a0e849");
    let mut out = [0u8; 192];
    assert!(g2_add(&a, &b, &mut out));
    assert_eq!(out.to_vec(), h("0411a5de6730ffece671a9f21d65028cc0f1102378de124562cb1ff49db6f004fcd14d683024b0548eff3d1468df268800fb837804dba8213329db46608b6c121d973363c1234a86dd183baff112709cf97096c5e9a1a770ee9d7dc641a894d619b5e8f5d4a72f2b75811ac084a7f814317360bac52f6aab15eed416b4ef9938e0bdc4865cc2c4d0fd947e7c6925fd14093567b4228be17ee62d11a254edd041ee4b953bffb8b8c7f925bd6662b4298bac2822b446f5b5de3b893e1be5aa4986"));
}

#[test]
fn g2_msm_official_vector_with_infinity_point() {
    let pairs = h("024aa2b2f08f0a91260805272dc51051c6e47ad4fa403b02b4510b647ae3d1770bac0326a805bbefd48056c8c121bdb813e02b6052719f607dacd3a088274f65596bd0d09920b61ab5da61bbdc7f5049334cf11213945d57e5ac7d055d042b7e0ce5d527727d6e118cc9cdc6da2e351aadfd9baa8cbdd3a76d429a695160d12c923ac9cc3baca289e193548608b828010606c4a02ea734cc32acd2b02bc28b99cb3e287e85a763af267492ab572e99ab3f370d275cec1da1aaa9075ff05f79be00000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002");
    let mut out = [0u8; 192];
    assert!(g2_msm(&pairs, &mut out));
    assert_eq!(out.to_vec(), h("1638533957d540a9d2370f17cc7ed5863bc0b995b8825e0ee1ea1e1e4d00dbae81f14b0bf3611b78c952aacab827a0530a4edef9c1ed7f729f520e47730a124fd70662a904ba1074728114d1031e1572c6c886f6b57ec72a6178288c47c335770468fb440d82b0630aeb8dca2b5256789a66da69bf91009cbfe6bd221e47aa8ae88dece9764bf3bd999d95d71e4c98990f6d4552fa65dd2638b361543f887136a43253d9c66c411697003f7a13c308f5422e1aa0a59c8967acdefd8b6e36ccf3"));
}

#[test]
fn pairing_official_vectors() {
    let mut verified = false;
    assert!(pairing_check(&h("0572cbea904d67468808c8eb50a9450c9721db309128012543902d0ac358a62ae28f75bb8f1c7c42c39a8c5529bf0f4e166a9d8cabc673a322fda673779d8e3822ba3ecb8670e461f73bb9021d5fd76a4c56d9d4cd16bd1bba86881979749d28122915c824a0857e2ee414a3dccb23ae691ae54329781315a0c75df1c04d6d7a50a030fc866f09d516020ef82324afae09380275bbc8e5dcea7dc4dd7e0550ff2ac480905396eda55062650f8d251c96eb480673937cc6d9d6a44aaa56ca66dc0b21da7955969e61010c7a1abc1a6f0136961d1e3b20b1a7326ac738fef5c721479dfd948b52fdf2455e44813ecfd89208f239ba329b3967fe48d718a36cfe5f62a7e42e0bf1c1ed714150a166bfbd6bcf6b3b58b975b9edea56d53f23a0e84906e82f6da4520f85c5d27d8f329eccfa05944fd1096b20734c894966d12a9e2a9a9744529d7212d33883113a0cadb90917d81038f7d60bee9110d9c0d6d1102fe2d998c957f28e31ec284cc04134df8e47e8f82ff3af2e60a6d9688a4563477c024aa2b2f08f0a91260805272dc51051c6e47ad4fa403b02b4510b647ae3d1770bac0326a805bbefd48056c8c121bdb813e02b6052719f607dacd3a088274f65596bd0d09920b61ab5da61bbdc7f5049334cf11213945d57e5ac7d055d042b7e0d1b3cc2c7027888be51d9ef691d77bcb679afda66c73f17f9ee3837a55024f78c71363275a75d75d86bab79f74782aa13fa4d4a0ad8b1ce186ed5061789213d993923066dddaf1040bc3ff59f825c78df74f2d75467e25e0f55f8a00fa030ed"), &mut verified));
    assert!(verified);
    assert!(pairing_check(&h("0572cbea904d67468808c8eb50a9450c9721db309128012543902d0ac358a62ae28f75bb8f1c7c42c39a8c5529bf0f4e166a9d8cabc673a322fda673779d8e3822ba3ecb8670e461f73bb9021d5fd76a4c56d9d4cd16bd1bba86881979749d28122915c824a0857e2ee414a3dccb23ae691ae54329781315a0c75df1c04d6d7a50a030fc866f09d516020ef82324afae09380275bbc8e5dcea7dc4dd7e0550ff2ac480905396eda55062650f8d251c96eb480673937cc6d9d6a44aaa56ca66dc0b21da7955969e61010c7a1abc1a6f0136961d1e3b20b1a7326ac738fef5c721479dfd948b52fdf2455e44813ecfd89208f239ba329b3967fe48d718a36cfe5f62a7e42e0bf1c1ed714150a166bfbd6bcf6b3b58b975b9edea56d53f23a0e84910e7791fb972fe014159aa33a98622da3cdc98ff707965e536d8636b5fcc5ac7a91a8c46e59a00dca575af0f18fb13dc16ba437edcc6551e30c10512367494bfb6b01cc6681e8a4c3cd2501832ab5c4abc40b4578b85cbaffbf0bcd70d67c6e2024aa2b2f08f0a91260805272dc51051c6e47ad4fa403b02b4510b647ae3d1770bac0326a805bbefd48056c8c121bdb813e02b6052719f607dacd3a088274f65596bd0d09920b61ab5da61bbdc7f5049334cf11213945d57e5ac7d055d042b7e0d1b3cc2c7027888be51d9ef691d77bcb679afda66c73f17f9ee3837a55024f78c71363275a75d75d86bab79f74782aa13fa4d4a0ad8b1ce186ed5061789213d993923066dddaf1040bc3ff59f825c78df74f2d75467e25e0f55f8a00fa030ed"), &mut verified));
    assert!(!verified);
    assert!(pairing_check(&[], &mut verified));
    assert!(verified);
}

#[test]
fn map_fp_to_g1_official_vector_0() {
    let fe: [u8; 48] = arr("14406e5bfb9209256a3820879a29ac2f62d6aca82324bf3ae2aa7d3c54792043bd8c791fccdb080c1a52dc68b8b69350");
    let mut out = [0u8; 96];
    assert!(map_fp_to_g1(&fe, &mut out));
    assert_eq!(out.to_vec(), h("0d7721bcdb7ce1047557776eb2659a444166dc6dd55c7ca6e240e21ae9aa18f529f04ac31d861b54faf3307692545db7108286acbdf4384f67659a8abe89e712a504cb3ce1cba07a716869025d60d499a00d1da8cdc92958918c222ea93d87f0"));
}

#[test]
fn map_fp_to_g1_official_vector_1() {
    let fe: [u8; 48] = arr("0e885bb33996e12f07da69073e2c0cc880bc8eff26d2a724299eb12d54f4bcf26f4748bb020e80a7e3794a7b0e47a641");
    let mut out = [0u8; 96];
    assert!(map_fp_to_g1(&fe, &mut out));
    assert_eq!(out.to_vec(), h("191ba6e4c4dafa22c03d41b050fe8782629337641be21e0397dc2553eb8588318a21d30647182782dee7f62a22fd020c0a721510a67277eabed3f153bd91df0074e1cbd37ef65b85226b1ce4fb5346d943cf21c388f0c5edbc753888254c760a"));
}

#[test]
fn map_fp2_to_g2_official_vector_0() {
    let fe: [u8; 96] = arr("14406e5bfb9209256a3820879a29ac2f62d6aca82324bf3ae2aa7d3c54792043bd8c791fccdb080c1a52dc68b8b693500e885bb33996e12f07da69073e2c0cc880bc8eff26d2a724299eb12d54f4bcf26f4748bb020e80a7e3794a7b0e47a641");
    let mut out = [0u8; 192];
    assert!(map_fp2_to_g2(&fe, &mut out));
    assert_eq!(out.to_vec(), h("0d029393d3a13ff5b26fe52bd8953768946c5510f9441f1136f1e938957882db6adbd7504177ee49281ecccba596f2bf1993f668fb1ae603aefbb1323000033fcb3b65d8ed3bf09c84c61e27704b745f540299a1872cd697ae45a5afd780f1d6079cb41060ef7a128d286c9ef8638689a49ca19da8672ea5c47b6ba6dbde193ee835d3b87a76a689966037c07159c10d17c688ae9a8b59a7069c27f2d58dd2196cb414f4fb89da8510518a1142ab19d158badd1c3bad03408fafb1669903cd6c"));
}

#[test]
fn map_fp2_to_g2_official_vector_1() {
    let fe: [u8; 96] = arr("0ba1b6d79150bdc368a14157ebfe8b5f691cf657a6bbe30e79b6654691136577d2ef1b36bfb232e3336e7e4c9352a8ed0f12847f7787f439575031bcdb1f03cfb79f942f3a9709306e4bd5afc73d3f78fd1c1fef913f503c8cbab58453fb7df2");
    let mut out = [0u8; 192];
    assert!(map_fp2_to_g2(&fe, &mut out));
    assert_eq!(out.to_vec(), h("0a2bca68ca23f3f03c678140d87465b5b336dbd50926d1219fcc0def162280765fe1093c117d52483d3d8cdc7ab765290fe83e3a958d6038569da6132bfa19f0e3dae3bee0d8a60e7cc33e4d7084a9e8c32fe31ec6e617277e2e450699eba1f805602683f0ef231cc0b7c8c695765d7933f4efa7503ed9f2aa3c774284eabcdd32fd287b6a3539c9749f2e15b58f5cd500b4f17de0db6e9d081723b613b23864c1eeae91b7cbda40ecd24823022aee7fc4068adc41947b97e17009fad9d0d4de"));
}

#[test]
fn map_rejects_non_canonical_field_element() {
    let fe = [0xffu8; 48];
    let mut out = [0u8; 96];
    assert!(!map_fp_to_g1(&fe, &mut out));
}

#[test]
fn kzg_point_eval_official_vector() {
    let commitment: [u8; 48] = arr("8f59a8d2a1a625a17f3fea0fe5eb8c896db3764f3185481bc22f91b4aaffcca25f26936857bc3a7c2539ea8ec3a952b7");
    let z: [u8; 32] = arr("564c0a11a0f704f4fc3e8acfe0f8245f0ad1347b378fbf96e206da11a5d36306");
    let y: [u8; 32] = arr("24d25032e67a7e6a4910df5834b8fe70e6bcfeeac0352434196bdf4b2485d5a1");
    let proof: [u8; 48] = arr("873033e038326e87ed3e1276fd140253fa08e9fc25fb2d9a98527fc22a2c9612fbeafdad446cbc7bcdbdcd780af2c16a");
    let mut verified = false;
    assert!(kzg::point_eval(&commitment, &z, &y, &proof, &mut verified));
    assert!(verified);

    let mut y_bad = y;
    y_bad[31] ^= 1;
    let ok = kzg::point_eval(&commitment, &z, &y_bad, &proof, &mut verified);
    assert!(!ok || !verified);
}

#[test]
fn kzg_point_eval_rejects_malformed_points() {
    let commitment = [0xffu8; 48];
    let z = [0u8; 32];
    let y = [0u8; 32];
    let proof = [0u8; 48];
    let mut verified = true;
    assert!(!kzg::point_eval(&commitment, &z, &y, &proof, &mut verified));
}

#[test]
fn ecrecover_official_vector() {
    let msg: [u8; 32] = arr("18c547e4f7b0f325ad1e56f57e26c745b09a3e503d86e00e5255ff7f715d3d1c");
    let v_word: [u8; 32] = arr("000000000000000000000000000000000000000000000000000000000000001c");
    let sig: [u8; 64] = arr("73b1693892219d736caba55bdb67216e485557ea6b6af75f37096c9aa6a5a75feeb940b1d03b21e36b0e47e79769f095fe2ab855bd91e3a38756b7d75a9c4549");
    assert!(v_word[..31].iter().all(|&b| b == 0));
    let recid = v_word[31] - 27;
    let mut pubkey = [0u8; 64];
    assert!(secp::ecrecover(&msg, &sig, recid, &mut pubkey));
    // The precompile's output is keccak256(pubkey)[12..]; compare addresses.
    use sha3::{Digest, Keccak256};
    let hash = Keccak256::digest(pubkey);
    assert_eq!(
        hash[12..].to_vec(),
        h("000000000000000000000000a94f5374fce5edbc8e2a8697c15331677e6ebf0b")[12..].to_vec()
    );

    // The recovered key verifies the same signature, and high-s acceptance holds: n - s with the
    // flipped recovery id recovers the same key.
    assert!(secp::verify(&msg, &sig, &pubkey));
    let n = h("fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141");
    let mut s = [0u8; 32];
    s.copy_from_slice(&sig[32..]);
    let mut borrow = 0i32;
    let mut high_s = [0u8; 32];
    for j in (0..32).rev() {
        let d = n[j] as i32 - s[j] as i32 - borrow;
        borrow = if d < 0 { 1 } else { 0 };
        high_s[j] = (d + if d < 0 { 256 } else { 0 }) as u8;
    }
    let mut sig_high = sig;
    sig_high[32..].copy_from_slice(&high_s);
    let mut pubkey_high = [0u8; 64];
    assert!(secp::ecrecover(
        &msg,
        &sig_high,
        recid ^ 1,
        &mut pubkey_high
    ));
    assert_eq!(pubkey, pubkey_high);
    assert!(secp::verify(&msg, &sig_high, &pubkey));
}

#[test]
fn ecrecover_rejects_invalid_inputs() {
    let msg = [0x11u8; 32];
    let mut out = [0u8; 64];
    assert!(!secp::ecrecover(&msg, &[0u8; 64], 0, &mut out));
    let mut sig_bad_r = [0xffu8; 64];
    sig_bad_r[32..].copy_from_slice(&[1u8; 32]);
    assert!(!secp::ecrecover(&msg, &sig_bad_r, 0, &mut out));
    let good_sig = [0x22u8; 64];
    assert!(!secp::ecrecover(&msg, &good_sig, 2, &mut out));
}

#[test]
fn s_g2_constant_matches_ceremony_compressed_encoding() {
    use ark_ec::AffineRepr;
    let s = kzg::s_g2();
    assert!(s.is_on_curve());
    assert!(s.is_in_correct_subgroup_assuming_on_curve());
    // Re-derive the ceremony's ZCash-compressed encoding: x as c1 ‖ c0 with the compression flag
    // and the y-sign flag.
    let mut compressed = [0u8; 96];
    fq_to_be(
        &s.x().unwrap().c1,
        (&mut compressed[..48]).try_into().unwrap(),
    );
    fq_to_be(
        &s.x().unwrap().c0,
        (&mut compressed[48..]).try_into().unwrap(),
    );
    let y = s.y().unwrap();
    let neg_y = -y;
    let larger =
        (y.c1.into_bigint(), y.c0.into_bigint()) > (neg_y.c1.into_bigint(), neg_y.c0.into_bigint());
    compressed[0] |= 0x80 | if larger { 0x20 } else { 0 };
    assert_eq!(
        hex::encode(compressed),
        "b5bfd7dd8cdeb128843bc287230af38926187075cbfbefa81009a2ce615ac53d2914e5870cb452d2afaaab24f3499f72185cbfee53492714734429b7b38608e23926c911cceceac9a36851477ba4c60b087041de621000edc98edada20c1def2"
    );
}

#[test]
fn kzg_constant_polynomial_first_principles() {
    // p(x) = 2: commitment = [2]G1 compressed, quotient = 0 so the proof is the compressed
    // identity; p(z) = 2 for every z.
    use ark_bls12_381::G1Affine;
    use ark_ec::{AffineRepr, CurveGroup};
    let two_g1 = (G1Affine::generator().into_group() + G1Affine::generator()).into_affine();
    let mut commitment = [0u8; 48];
    {
        let mut x = [0u8; 48];
        fq_to_be(&two_g1.x().unwrap(), &mut x);
        commitment.copy_from_slice(&x);
        let y = two_g1.y().unwrap();
        let larger = y.into_bigint() > (-y).into_bigint();
        commitment[0] |= 0x80 | if larger { 0x20 } else { 0 };
    }
    let mut proof = [0u8; 48];
    proof[0] = 0xc0;
    let mut z = [0u8; 32];
    z[31] = 2;
    let mut y = [0u8; 32];
    y[31] = 2;
    let mut verified = false;
    assert!(kzg::point_eval(&commitment, &z, &y, &proof, &mut verified));
    assert!(verified);
    y[31] = 3;
    assert!(kzg::point_eval(&commitment, &z, &y, &proof, &mut verified));
    assert!(!verified);
}

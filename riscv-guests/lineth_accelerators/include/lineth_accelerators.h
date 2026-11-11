/**
 * Lineth Cryptographic Accelerators C Interface
 *
 * This header defines Lineth-specific accelerators that are NOT part of the
 * upstream zkVM accelerator standard. They live in the `lineth_zkvm_` namespace
 * to keep them distinct from the standard `zkvm_` accelerators.
 *
 * Status codes and the alignment helper are reused from the standard interface
 * in zkvm_accelerators.h.
 */

#ifndef LINETH_ACCELERATORS_H
#define LINETH_ACCELERATORS_H

#include <stddef.h>
#include <stdint.h>

#include "zkvm_accelerators.h" /* zkvm_status, zkvm_bytes_64 */

#ifdef __cplusplus
extern "C" {
#endif

/* ============================================================================
 * Poseidon2 (KoalaBear)
 * ============================================================================ */

/**
 * Apply the Poseidon2 permutation.
 *
 * The state is 16 KoalaBear field elements. Each element is a canonical value
 * in [0, 2^31 - 2^24 + 1) laid out as a native little-endian 32-bit word, so
 * the 16 * 4 = 64-byte state is passed as a zkvm_bytes_64.
 *
 * Reads the state at *input, applies the permutation, and writes the permuted
 * state to *output. input and output may alias.
 *
 * @param input Pointer to the input state
 * @param[out] output Pointer to the output state
 * @return ZKVM_EOK on success, ZKVM_EFAIL on failure
 */
zkvm_status lineth_zkvm_poseidon2_permutation(const zkvm_bytes_64* input,
                                              zkvm_bytes_64* output);

/* ============================================================================
 * KZG blob verification
 * ============================================================================ */

/**
 * Verify EIP-4844 blob proofs as one batch.
 *
 * @param blobs Pointer to n contiguous blobs of 131072 bytes each
 * @param commitments Pointer to n KZG commitments
 * @param proofs Pointer to n KZG blob proofs
 * @param n Number of blobs, commitments and proofs
 * @param[out] verified Pointer to bool indicating if every proof opens its commitment
 *        to its blob (the consensus specs' verify_blob_kzg_proof_batch; true for
 *        n = 0); false for a blob element of at least the scalar field's order and
 *        for an invalid commitment or proof
 * @return ZKVM_EOK on success, ZKVM_EFAIL on failure
 */
zkvm_status lineth_zkvm_kzg_verify_blob_batch(const uint8_t* blobs,
                                              const zkvm_kzg_commitment* commitments,
                                              const zkvm_kzg_proof* proofs,
                                              size_t n,
                                              bool* verified);

/* ============================================================================
 * Field arithmetic
 * ============================================================================ */

/* Operations */
#define LINETH_FIELD_OP_MUL 0  /* out = a * b */
#define LINETH_FIELD_OP_DIV 1  /* out = a / b; out = 0 and status 1 if b = 0 */
#define LINETH_FIELD_OP_SQRT 2 /* out^2 = a; if a is not a square, out^2 = -a and status 1 */

/* Fields */
#define LINETH_FIELD_SECP256K1_FP 1
#define LINETH_FIELD_SECP256K1_FR 2
#define LINETH_FIELD_SECP256R1_FP 3
#define LINETH_FIELD_SECP256R1_FR 4
#define LINETH_FIELD_BN254_FP 5
#define LINETH_FIELD_BN254_FR 6
#define LINETH_FIELD_BLS12_381_FR 7

/**
 * One operation in a known 256-bit prime field, proven by the arithmetization.
 *
 * Elements are four little-endian 64-bit limbs of the plain value (not in
 * Montgomery form). Inputs must be below 2p (any 256-bit value for the
 * secp256k1 and secp256r1 fields); out is reduced and may alias an input. b is
 * read for every operation (pass a for SQRT). SQRT is only supported where
 * p = 3 (mod 4) (secp256k1 and secp256r1 Fp, bn254 Fp) and returns either root.
 * An inverse is DIV with a = 1.
 *
 * An unknown op or field is unprovable.
 *
 * @param op LINETH_FIELD_OP_*
 * @param field LINETH_FIELD_*
 * @param[out] out Pointer to the result
 * @param a Pointer to the first operand
 * @param b Pointer to the second operand
 * @return 0, or 1 if there is no result (b = 0, or a is not a square)
 */
uint32_t lineth_zkvm_field_op(uint32_t op, uint32_t field, uint64_t out[4],
                              const uint64_t a[4], const uint64_t b[4]);

#ifdef __cplusplus
}
#endif

#endif /* LINETH_ACCELERATORS_H */

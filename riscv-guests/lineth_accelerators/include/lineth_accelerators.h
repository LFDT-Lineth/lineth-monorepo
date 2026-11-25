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

#ifdef __cplusplus
}
#endif

#endif /* LINETH_ACCELERATORS_H */

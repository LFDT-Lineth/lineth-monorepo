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
 * Verify an EIP-4844 blob proof.
 *
 * @param blob Pointer to the blob of 131072 bytes
 * @param commitment Pointer to the KZG commitment
 * @param proof Pointer to the KZG blob proof
 * @param[out] verified Pointer to bool indicating if the proof opens the commitment
 *        to the blob; false for a blob element of at least the scalar field's order
 *        and for an invalid commitment or proof
 * @return ZKVM_EOK on success, ZKVM_EFAIL on failure
 */
zkvm_status lineth_zkvm_kzg_verify_blob(const uint8_t* blob,
                                        const zkvm_kzg_commitment* commitment,
                                        const zkvm_kzg_proof* proof,
                                        bool* verified);

#ifdef __cplusplus
}
#endif

#endif /* LINETH_ACCELERATORS_H */

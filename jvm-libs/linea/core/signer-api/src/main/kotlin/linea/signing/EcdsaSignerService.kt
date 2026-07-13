package linea.signing

import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * A secp256k1 signing service whose private key never enters this process (remote KMS/HSM).
 */
interface EcdsaSignerService {
  /** The signing key's 64-byte uncompressed public key point (X || Y, no 0x04 prefix). */
  fun publicKey(): ByteArray

  /**
   * Signs a 32-byte digest without blocking the caller: signing is a network round-trip to the
   * remote backend, and callers (e.g. the coordinator) must not stall on it. Hashing is the
   * caller's responsibility: Ethereum signs keccak256 digests, and remote services must not be
   * allowed to substitute their own hash.
   */
  fun signDigest(digest: ByteArray): SafeFuture<SignatureRS>
}

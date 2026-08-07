package lineth.crypto

import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Signs payloads with a key that may live outside this process (remote KMS/HSM).
 * [T] is the signature type the backend produces, e.g. [Secp256k1Signature].
 */
interface Signer<T> {
  /** The signing key's public key, in the implementation's encoding. */
  fun publicKey(): ByteArray

  /**
   * Signs [bytes] without blocking the caller: signing may be a network round-trip to a remote
   * backend whose private key never enters this process, and callers (e.g. the coordinator)
   * must not stall on it.
   */
  fun sign(bytes: ByteArray): SafeFuture<T>
}

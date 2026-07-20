package linea.coordinator.extensions

import linea.crypto.Secp256k1Signature
import linea.crypto.Signer

/** Resolves custom signer names using an implementation supplied by a downstream distribution. */
fun interface CustomSignerFactory {
  fun create(name: String): Signer<Secp256k1Signature>
}

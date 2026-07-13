package linea.signing

import java.math.BigInteger

/**
 * A canonical secp256k1 ECDSA signature. Canonical means low-s (EIP-2): s <= n/2, so each message
 * has a single valid signature encoding and downstream recovery-id derivation is well-defined.
 * Backends must normalize before constructing this type; the constructor enforces it.
 */
data class SignatureRS(val r: BigInteger, val s: BigInteger) {
  init {
    require(r > BigInteger.ZERO && r < Secp256k1.N) { "r must be in [1, n-1]" }
    require(s > BigInteger.ZERO && s <= Secp256k1.HALF_N) { "s must be in [1, n/2] (EIP-2 low-s)" }
  }
}

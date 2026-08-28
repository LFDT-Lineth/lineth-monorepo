package linea.crypto

import java.math.BigInteger

/**
 * A canonical secp256k1 ECDSA signature. Canonical means low-s (EIP-2): s <= n/2, so each message
 * has a single valid signature encoding and downstream recovery-id derivation is well-defined.
 * Backends must normalize before constructing this type; the constructor enforces it.
 */
data class Secp256k1Signature(val r: BigInteger, val s: BigInteger) {
  init {
    require(r > BigInteger.ZERO && r < Secp256k1.N) { "r must be in [1, n-1]" }
    require(s > BigInteger.ZERO && s <= Secp256k1.HALF_N) { "s must be in [1, n/2] (EIP-2 low-s)" }
  }

  /** Encodes as the 64-byte r || s wire format, each component left-padded to 32 bytes. */
  fun toRSBytes(): ByteArray = padTo32(r) + padTo32(s)

  companion object {
    const val COMPONENT_SIZE_BYTES = 32
    const val SIZE_BYTES = 2 * COMPONENT_SIZE_BYTES

    fun fromRSBytes(bytes: ByteArray): Secp256k1Signature {
      require(bytes.size == SIZE_BYTES) { "signature must be $SIZE_BYTES bytes (r || s), got ${bytes.size}" }
      return Secp256k1Signature(
        r = BigInteger(1, bytes.copyOfRange(0, COMPONENT_SIZE_BYTES)),
        s = BigInteger(1, bytes.copyOfRange(COMPONENT_SIZE_BYTES, SIZE_BYTES)),
      )
    }

    private fun padTo32(value: BigInteger): ByteArray {
      val raw = value.toByteArray().dropWhile { it == 0.toByte() }.toByteArray()
      return ByteArray(COMPONENT_SIZE_BYTES - raw.size) + raw
    }
  }
}

package linea.crypto

import java.math.BigInteger

fun interface Signer {
  /**
   * Signs data and returns (R, S) tuples.
   */
  fun sign(bytes: ByteArray): Pair<BigInteger, BigInteger>
}

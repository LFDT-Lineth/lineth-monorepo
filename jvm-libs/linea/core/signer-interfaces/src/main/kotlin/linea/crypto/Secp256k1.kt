package linea.crypto

import java.math.BigInteger

/** secp256k1 curve constants needed to state (and enforce) the signature contract. */
object Secp256k1 {
  /** The curve (group) order n. */
  val N: BigInteger = BigInteger("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)

  /** floor(n / 2) - the canonical (EIP-2) upper bound for s. */
  val HALF_N: BigInteger = N.shiftRight(1)
}

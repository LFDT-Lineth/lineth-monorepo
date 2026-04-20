package linea.crypto

import java.math.BigInteger

interface Signer {
  fun sign(bytes: ByteArray): Pair<BigInteger, BigInteger>
}

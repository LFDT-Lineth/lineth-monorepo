package linea.crypto

import org.web3j.crypto.ECKeyPair
import java.math.BigInteger

class ECKeypairSigner(private val keyPair: ECKeyPair) : Signer {
  override fun sign(bytes: ByteArray): Pair<BigInteger, BigInteger> {
    val signature = keyPair.sign(bytes)
    return signature.r to signature.s
  }
}

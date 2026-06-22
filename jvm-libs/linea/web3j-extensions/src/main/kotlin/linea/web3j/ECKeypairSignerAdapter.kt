package linea.web3j

import org.web3j.crypto.ECDSASignature
import org.web3j.crypto.ECKeyPair
import java.math.BigInteger

class ECKeypairSignerAdapter(
  private val signerDelegate: (ByteArray) -> Pair<BigInteger, BigInteger>,
  publicKey: BigInteger,
) :
  ECKeyPair(BigInteger.ZERO, publicKey) {
  override fun equals(other: Any?): Boolean {
    if (this === other) {
      return true
    }
    if (other == null || javaClass != other.javaClass) {
      return false
    }

    val ecKeyPair = other as ECKeypairSignerAdapter

    return super.equals(other) && signerDelegate == ecKeyPair.signerDelegate
  }

  override fun hashCode(): Int {
    var result = super.hashCode()
    result = 31 * result + signerDelegate.hashCode()
    return result
  }

  override fun getPrivateKey(): BigInteger {
    throw RuntimeException("Key is managed by delegated Signer: $signerDelegate!")
  }

  override fun sign(transaction: ByteArray): ECDSASignature {
    val (r, s) = signerDelegate(transaction)
    return ECDSASignature(r, s)
  }
}

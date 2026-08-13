package linea.web3j

import linea.crypto.Secp256k1Signature
import linea.crypto.Signer
import org.web3j.crypto.ECDSASignature
import org.web3j.crypto.ECKeyPair
import java.math.BigInteger

class ECKeypairSignerAdapter(
  private val signerDelegate: Signer<Secp256k1Signature>,
) :
  ECKeyPair(BigInteger.ZERO, BigInteger(1, signerDelegate.publicKey())) {
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
    // web3j's TxSignService seam is synchronous, so this is where the async signer is awaited.
    val signature = signerDelegate.sign(transaction).get()
    return ECDSASignature(signature.r, signature.s)
  }
}

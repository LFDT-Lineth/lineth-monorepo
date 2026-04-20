package net.consensys.zkevm.ethereum.signing

import linea.crypto.Signer
import org.web3j.crypto.ECDSASignature
import org.web3j.crypto.ECKeyPair
import java.math.BigInteger

class ECKeypairSignerAdapter(private val adaptee: Signer, publicKey: BigInteger) :
  ECKeyPair(BigInteger.ZERO, publicKey) {
  override fun equals(other: Any?): Boolean {
    if (this === other) {
      return true
    }
    if (other == null || javaClass != other.javaClass) {
      return false
    }

    val ecKeyPair = other as ECKeypairSignerAdapter

    return super.equals(other) && adaptee == ecKeyPair.adaptee
  }

  override fun hashCode(): Int {
    var result = super.hashCode()
    result = 31 * result + adaptee.hashCode()
    return result
  }

  override fun getPrivateKey(): BigInteger {
    throw Exception("Key is managed by Signer adaptee, $adaptee!")
  }

  override fun sign(transaction: ByteArray): ECDSASignature {
    val (r, s) = adaptee.sign(transaction)
    return ECDSASignature(r, s)
  }
}

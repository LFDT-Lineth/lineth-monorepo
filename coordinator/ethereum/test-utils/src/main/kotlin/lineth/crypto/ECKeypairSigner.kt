package lineth.crypto

import org.web3j.crypto.ECKeyPair
import org.web3j.crypto.Hash
import org.web3j.utils.Numeric
import tech.pegasys.teku.infrastructure.async.SafeFuture

class ECKeypairSigner(private val keyPair: ECKeyPair) : Signer<Secp256k1Signature> {
  override fun publicKey(): ByteArray = Numeric.toBytesPadded(keyPair.publicKey, 64)

  override fun sign(bytes: ByteArray): SafeFuture<Secp256k1Signature> {
    val signature = keyPair.sign(Hash.sha3(bytes))
    return SafeFuture.completedFuture(Secp256k1Signature(signature.r, signature.s))
  }
}

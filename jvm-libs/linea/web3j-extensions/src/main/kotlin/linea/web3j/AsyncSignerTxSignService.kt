package linea.web3j

import linea.crypto.Secp256k1Signature
import linea.crypto.Signer
import org.web3j.crypto.ECDSASignature
import org.web3j.crypto.Hash
import org.web3j.crypto.Keys
import org.web3j.crypto.RawTransaction
import org.web3j.crypto.Sign
import org.web3j.crypto.TransactionEncoder
import org.web3j.service.TxSignService
import org.web3j.utils.Numeric
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger

/** [TxSignService] backed by an async [Signer]; use [signAsync] from async code. */
class AsyncSignerTxSignService(
  private val signer: Signer<Secp256k1Signature>,
) : TxSignService {
  private val publicKey = BigInteger(1, signer.publicKey())
  private val address = Numeric.prependHexPrefix(Keys.getAddress(publicKey))

  fun signAsync(rawTransaction: RawTransaction, chainId: Long): SafeFuture<ByteArray> {
    val eip155 = chainId > NO_CHAIN_ID && rawTransaction.type.isLegacy
    val encoded = when {
      eip155 -> TransactionEncoder.encode(rawTransaction, chainId)
      rawTransaction.type.isEip4844 -> TransactionEncoder.encode4844(rawTransaction)
      else -> TransactionEncoder.encode(rawTransaction)
    }
    val digest = Hash.sha3(encoded)
    return signer.sign(digest).thenApply { signature ->
      val signatureData = Sign.createSignatureData(ECDSASignature(signature.r, signature.s), publicKey, digest)
        .let { if (eip155) TransactionEncoder.createEip155SignatureData(it, chainId) else it }
      TransactionEncoder.encode(rawTransaction, signatureData)
    }
  }

  override fun sign(rawTransaction: RawTransaction, chainId: Long): ByteArray {
    return signAsync(rawTransaction, chainId).get()
  }

  override fun getAddress(): String = address

  companion object {
    const val NO_CHAIN_ID = -1L
  }
}

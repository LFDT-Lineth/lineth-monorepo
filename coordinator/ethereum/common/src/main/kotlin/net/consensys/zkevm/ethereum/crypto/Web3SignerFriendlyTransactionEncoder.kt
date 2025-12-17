package net.consensys.zkevm.ethereum.crypto

import org.web3j.crypto.Credentials
import org.web3j.crypto.ECKeyPair
import org.web3j.crypto.Hash
import org.web3j.crypto.RawTransaction
import org.web3j.crypto.Sign
import org.web3j.crypto.TransactionEncoder
import org.web3j.crypto.TransactionEncoder.encode4844

object Web3SignerFriendlyTransactionEncoder {
  fun signMessage(rawTransaction: RawTransaction, credentials: Credentials): ByteArray {
    val encodedTransaction =
      if (rawTransaction.transaction.type.isEip4844) {
        encode4844(rawTransaction)
      } else {
        TransactionEncoder.encode(rawTransaction)
      }
    val signatureData = signMessage(encodedTransaction, credentials.ecKeyPair)

    return TransactionEncoder.encode(rawTransaction, signatureData)
  }

  private fun signMessage(message: ByteArray, keyPair: ECKeyPair): Sign.SignatureData {
    val publicKey = keyPair.publicKey
    val messageHash: ByteArray = Hash.sha3(message)
    val sig = keyPair.sign(message)
    return Sign.createSignatureData(sig, publicKey, messageHash)
  }
}

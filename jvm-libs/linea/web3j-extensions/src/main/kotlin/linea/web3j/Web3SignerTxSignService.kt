package linea.web3j

import org.web3j.crypto.Credentials
import org.web3j.crypto.RawTransaction
import org.web3j.service.TxSignService

class Web3SignerTxSignService(private val credentials: Credentials) : TxSignService {
  override fun sign(rawTransaction: RawTransaction, chainId: Long): ByteArray {
    return Web3SignerFriendlyTransactionEncoder.signMessage(rawTransaction, credentials)
  }

  override fun getAddress(): String {
    return credentials.address
  }
}

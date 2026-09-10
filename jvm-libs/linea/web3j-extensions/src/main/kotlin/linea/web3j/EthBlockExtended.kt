package linea.web3j

import org.web3j.protocol.core.Response
import org.web3j.protocol.core.methods.response.EthBlock

/** Header fields not yet exposed by Web3j, populated by the original block RPC. */
class EthBlockExtended : Response<EthBlockExtended.Block>() {
  class Block : EthBlock.Block() {
    var slotNumber: String? = null
    var blockAccessListHash: String? = null
    var requestsHash: String? = null
  }
}

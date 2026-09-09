package linea.web3j.ethapi

import linea.domain.BlockNumberAndHash
import linea.domain.ExecutionPayload
import linea.domain.toExecutionPayload
import linea.ethapi.ExecutionPayloadClient
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import linea.kotlin.toHexString
import linea.rlp.RLP
import linea.web3j.requestAsync
import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.datatypes.Hash
import org.web3j.protocol.Web3jService
import org.web3j.protocol.core.Request
import org.web3j.protocol.core.Response
import tech.pegasys.teku.infrastructure.async.SafeFuture

class Web3jExecutionPayloadClient(
  private val web3jService: Web3jService,
) : ExecutionPayloadClient {
  override fun getExecutionPayload(block: BlockNumberAndHash): SafeFuture<ExecutionPayload> {
    // debug_getRawBlock only accepts block numbers/tags; verify the hash before using its result.
    return getRawData("debug_getRawBlock", block.number.toHexString()).thenCompose { rawBlock ->
      val decoded = RLP.decodeBlockWithMainnetFunctions(rawBlock)
      require(
        decoded.header.number.toULong() == block.number &&
          decoded.header.hash.bytes.toArray().contentEquals(block.hash),
      ) {
        "Raw block does not match requested block $block"
      }
      if (decoded.header.balHash.isPresent) {
        getRawData("debug_getRawBlockAccessList", block.hash.encodeHex()).thenApply { blockAccessList ->
          require(Hash.hash(Bytes.wrap(blockAccessList)) == decoded.header.balHash.get()) {
            "Block access list hash does not match header for block $block"
          }
          decoded.toExecutionPayload(blockAccessList)
        }
      } else {
        SafeFuture.completedFuture(decoded.toExecutionPayload(ByteArray(0)))
      }
    }
  }

  private fun getRawData(method: String, block: String): SafeFuture<ByteArray> =
    Request(method, listOf(block), web3jService, RawDataResponse::class.java).requestAsync { response ->
      val result = requireNotNull(response.result) { "No $method data available for block $block" }.decodeHex()
      require(result.isNotEmpty()) { "Empty $method data for block $block" }
      result
    }
}

class RawDataResponse : Response<String>()

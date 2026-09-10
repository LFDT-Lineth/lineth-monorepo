package linea.web3j.ethapi

import linea.domain.Block
import linea.domain.ExecutionPayload
import linea.domain.toExecutionPayload
import linea.ethapi.ExecutionPayloadClient
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
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
  override fun getExecutionPayload(block: Block): SafeFuture<ExecutionPayload> {
    val blockAccessList = when {
      block.blockAccessListHash == null -> SafeFuture.completedFuture(ByteArray(0))
      block.blockAccessListHash.contentEquals(Hash.EMPTY_BAL_HASH.bytes.toArray()) -> {
        // The commitment identifies the canonical empty RLP list; no historical lookup is needed.
        SafeFuture.completedFuture(byteArrayOf(0xc0.toByte()))
      }
      else -> getBlockAccessList(block.hash.encodeHex()).thenApply { blockAccessList ->
        require(Hash.hash(Bytes.wrap(blockAccessList)).bytes.toArray().contentEquals(block.blockAccessListHash)) {
          "Block access list hash does not match header for block ${block.number}"
        }
        blockAccessList
      }
    }
    return blockAccessList.thenApply(block::toExecutionPayload)
  }

  private fun getBlockAccessList(blockHash: String): SafeFuture<ByteArray> =
    Request("debug_getRawBlockAccessList", listOf(blockHash), web3jService, RawDataResponse::class.java)
      .requestAsync { response ->
        val result = requireNotNull(response.result) { "No block access list for block $blockHash" }.decodeHex()
        require(result.isNotEmpty()) { "Empty block access list for block $blockHash" }
        result
      }
}

private class RawDataResponse : Response<String>()

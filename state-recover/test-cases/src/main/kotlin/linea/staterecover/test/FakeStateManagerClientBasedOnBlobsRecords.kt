package linea.staterecover.test

import build.linea.clients.GetZkEVMStateMerkleProofResponse
import build.linea.clients.StateManagerClientV1
import build.linea.clients.StateManagerErrorType
import build.linea.domain.BlockInterval
import com.fasterxml.jackson.databind.node.ArrayNode
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import linea.EthLogsSearcher
import linea.staterecover.DataFinalizedV3
import net.consensys.linea.BlockParameter
import net.consensys.linea.errors.ErrorResponse
import net.consensys.toHexStringUInt256
import net.consensys.zkevm.domain.BlobRecord
import tech.pegasys.teku.infrastructure.async.SafeFuture

open class FakeStateManagerClient(
  private val blocksStateRootHashes: MutableMap<ULong, ByteArray> = mutableMapOf<ULong, ByteArray>(),
  var headBlockNumber: ULong = blocksStateRootHashes.keys.maxOrNull() ?: 0UL
) : StateManagerClientV1 {

  fun setBlockStateRootHash(blockNumber: ULong, stateRootHash: ByteArray) {
    blocksStateRootHashes[blockNumber] = stateRootHash
    headBlockNumber = blocksStateRootHashes.keys.maxOrNull() ?: 0UL
  }

  open fun getStateRootHash(blockNumber: ULong): SafeFuture<ByteArray> {
    return blocksStateRootHashes[blockNumber]
      ?.let { SafeFuture.completedFuture(it) }
      ?: SafeFuture.failedFuture(RuntimeException("StateRootHash not found for block=$blockNumber"))
  }

  override fun rollupGetHeadBlockNumber(): SafeFuture<ULong> {
    return SafeFuture.completedFuture(headBlockNumber)
  }

  override fun rollupGetStateMerkleProofWithTypedError(
    blockInterval: BlockInterval
  ): SafeFuture<Result<GetZkEVMStateMerkleProofResponse, ErrorResponse<StateManagerErrorType>>> {
    // For state recovery, we just need the endStateRootHash
    return getStateRootHash(blockInterval.endBlockNumber)
      .thenApply { stateRootHash ->
        Ok(
          GetZkEVMStateMerkleProofResponse(
            zkStateMerkleProof = ArrayNode(null),
            zkParentStateRootHash = ByteArray(32),
            zkEndStateRootHash = stateRootHash,
            zkStateManagerVersion = "fake-version"
          )
        )
      }
  }
}

class FakeStateManagerClientBasedOnBlobsRecords(
  val blobRecords: List<BlobRecord>
) : FakeStateManagerClient(
  blocksStateRootHashes = blobRecords
    .associate { it.endBlockNumber to it.blobCompressionProof!!.finalStateRootHash }.toMutableMap()
)

class FakeStateManagerClientReadFromL1(
  headBlockNumber: ULong,
  val logsSearcher: EthLogsSearcher,
  val contractAddress: String
) : FakeStateManagerClient(
  headBlockNumber = headBlockNumber
) {

  override fun getStateRootHash(blockNumber: ULong): SafeFuture<ByteArray> {
    return super
      .getStateRootHash(blockNumber)
      .exceptionallyCompose {
        logsSearcher
          .getLogs(
            fromBlock = BlockParameter.Tag.EARLIEST,
            toBlock = BlockParameter.Tag.FINALIZED,
            address = contractAddress,
            topics = listOf(
              DataFinalizedV3.topic,
              null,
              blockNumber.toHexStringUInt256()
            )
          ).thenApply { logs ->
            val logEvent = DataFinalizedV3.fromEthLog(logs.first())
            logEvent.event.finalStateRootHash
          }
      }
  }
}

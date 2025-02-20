package linea.staterecovery.test

import build.linea.clients.GetZkEVMStateMerkleProofResponse
import build.linea.clients.StateManagerClientV1
import build.linea.clients.StateManagerErrorType
import com.fasterxml.jackson.databind.node.ArrayNode
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import linea.EthLogsSearcher
import linea.domain.BlockInterval
import linea.domain.BlockParameter
import linea.kotlin.toHexStringUInt256
import linea.staterecovery.DataFinalizedV3
import net.consensys.linea.errors.ErrorResponse
import net.consensys.zkevm.domain.BlobRecord
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ConcurrentHashMap

open class FakeStateManagerClient(
  _blocksStateRootHashes: Map<ULong, ByteArray> = emptyMap(),
  var headBlockNumber: ULong = _blocksStateRootHashes.keys.maxOrNull() ?: 0UL
) : StateManagerClientV1 {
  open val blocksStateRootHashes: MutableMap<ULong, ByteArray> =
    ConcurrentHashMap<ULong, ByteArray>(_blocksStateRootHashes)

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
  _blocksStateRootHashes = blobRecords
    .associate { it.endBlockNumber to it.blobCompressionProof!!.finalStateRootHash }
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
            logs.firstOrNull()?.let { finalizationLog ->
              DataFinalizedV3.fromEthLog(finalizationLog).event.finalStateRootHash
            }
              ?: ByteArray(32) { 0 }
          }
      }
  }
}

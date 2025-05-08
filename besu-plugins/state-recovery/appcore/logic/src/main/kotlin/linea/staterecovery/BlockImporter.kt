package linea.staterecovery

import build.linea.clients.StateManagerClientV1
import io.vertx.core.Vertx
import linea.domain.BlockInterval
import net.consensys.linea.async.AsyncRetryer
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

data class ImportResult(
  val blockNumber: ULong,
  val zkStateRootHash: ByteArray
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ImportResult

    if (blockNumber != other.blockNumber) return false
    if (!zkStateRootHash.contentEquals(other.zkStateRootHash)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = blockNumber.hashCode()
    result = 31 * result + zkStateRootHash.contentHashCode()
    return result
  }
}

interface BlockImporterAndStateVerifier {
  fun importBlocks(blocks: List<BlockFromL1RecoveredData>): SafeFuture<ImportResult>
}

class BlockImporterAndStateVerifierV1(
  private val vertx: Vertx,
  private val elClient: ExecutionLayerClient,
  private val stateManagerClient: StateManagerClientV1,
  private val stateManagerImportTimeoutPerBlock: Duration
) : BlockImporterAndStateVerifier {
  override fun importBlocks(blocks: List<BlockFromL1RecoveredData>): SafeFuture<ImportResult> {
    val sortedBlocks = blocks.sortedBy { it.header.blockNumber }
    val lastBlockNumber = sortedBlocks.last().header.blockNumber
    return elClient
      .lineaEngineImportBlocksFromBlob(sortedBlocks)
      .thenCompose {
        getBlockStateRootHash(
          blockNumber = lastBlockNumber,
          timeout = stateManagerImportTimeoutPerBlock.times(blocks.size)
        )
      }
      .thenApply { stateRootHash ->
        ImportResult(
          blockNumber = lastBlockNumber,
          zkStateRootHash = stateRootHash
        )
      }
  }

  private fun getBlockStateRootHash(
    blockNumber: ULong,
    timeout: Duration
  ): SafeFuture<ByteArray> {
    return AsyncRetryer
      .retry(
        vertx,
        backoffDelay = 1.seconds,
        timeout = timeout,
        stopRetriesPredicate = { headBlockNumber -> headBlockNumber >= blockNumber },
        action = { stateManagerClient.rollupGetHeadBlockNumber() }
      )
      .thenCompose {
        stateManagerClient.rollupGetStateMerkleProof(BlockInterval(blockNumber, blockNumber))
      }.thenApply { proofResponse -> proofResponse.zkEndStateRootHash }
  }
}

package linea.clients

import linea.domain.BlockIntervalProofIndex
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.nio.ByteBuffer
import java.util.concurrent.CopyOnWriteArrayList
import kotlin.time.Instant

/**
 * In-memory [RollupProverClientV1]. Submitted requests are recorded in [requests];
 * [responseProvider] decides whether a proof is ready (null = not ready yet).
 */
class FakeRollupProverClient(
  var responseProvider: (BlockIntervalProofIndex) -> RollupProofResponseV1? = ::dummyRollupProofResponse,
) : RollupProverClientV1 {
  val requests: MutableList<RollupProofRequestV1> = CopyOnWriteArrayList()

  override fun getProofIndex(proofRequest: RollupProofRequestV1): BlockIntervalProofIndex {
    return BlockIntervalProofIndex(
      startBlockNumber = proofRequest.startBlockNumber,
      endBlockNumber = proofRequest.endBlockNumber,
      startBlockTimestamp = proofRequest.startBlockTimestamp,
      hash = ByteBuffer.allocate(32)
        .putLong(proofRequest.startBlockNumber.toLong())
        .putLong(proofRequest.endBlockNumber.toLong())
        .array(),
    )
  }

  override fun createProofRequest(proofRequest: RollupProofRequestV1): SafeFuture<BlockIntervalProofIndex> {
    requests.add(proofRequest)
    return SafeFuture.completedFuture(getProofIndex(proofRequest))
  }

  override fun findProofResponse(proofIndex: BlockIntervalProofIndex): SafeFuture<RollupProofResponseV1?> {
    return SafeFuture.completedFuture(responseProvider(proofIndex))
  }

  override fun isProofAlreadyDone(proofIndex: BlockIntervalProofIndex): SafeFuture<Boolean> {
    return findProofResponse(proofIndex).thenApply { it != null }
  }

  override fun requestProof(proofRequest: RollupProofRequestV1): SafeFuture<RollupProofResponseV1> {
    return createProofRequest(proofRequest).thenApply { requireNotNull(responseProvider(it)) }
  }

  override fun removeRequests(startBlockNumberGte: Long?): SafeFuture<Unit> {
    requests.removeIf { startBlockNumberGte == null || it.startBlockNumber.toLong() >= startBlockNumberGte }
    return SafeFuture.completedFuture(Unit)
  }
}

fun dummyRollupProofResponse(proofIndex: BlockIntervalProofIndex): RollupProofResponseV1 {
  val hash = ByteArray(32)
  return RollupProofResponseV1(
    startBlockNumber = proofIndex.startBlockNumber,
    endBlockNumber = proofIndex.endBlockNumber,
    proof = byteArrayOf(0x01),
    publicInputs = RollupProofPublicInputs(
      endBlockNumber = proofIndex.endBlockNumber,
      endBlockTimestamp = Instant.fromEpochSeconds(0),
      l2L1BridgeTransactionTree = hash,
      parentL1L2BridgeMessageNumber = 0UL,
      parentL1L2BridgeMessageRollingHash = hash,
      endL1L2BridgeMessageNumber = 0UL,
      endL1L2BridgeMessageRollingHash = hash,
      dynamicChainConfigHash = hash,
      parentFtxNumber = 0UL,
      parentFtxRollingHash = hash,
      endFtxNumber = 0UL,
      endFtxRollingHash = hash,
      filteredAddressesHash = hash,
      parentDataRollingHash = hash,
      endDataRollingHash = hash,
      parentBlockHash = hash,
      endBlockHash = hash,
      startOffset = 0,
      endOffset = 0,
      programVks = emptyList(),
    ),
    l2L1Roots = emptyList(),
    filteredAddresses = emptyList(),
    programVk = hash,
  )
}

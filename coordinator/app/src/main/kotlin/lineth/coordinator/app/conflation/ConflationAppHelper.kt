package lineth.coordinator.app.conflation

import io.vertx.core.Vertx
import linea.clients.L2ExecutionProverClientV1
import linea.clients.RollupAggregationProverClientV1
import linea.clients.RollupProverClientV1
import linea.domain.BlockWithTxHashes
import linea.domain.toBlockParameter
import linea.ethapi.EthApiClient
import linea.kotlin.toInstantFromEpochSeconds
import lineth.conflation.calculators.BlobTriggerCalculator
import lineth.conflation.calculators.BlobTriggerCalculatorByTargetBlockNumbers
import lineth.conflation.calculators.RollupTriggerCalculator
import lineth.conflation.calculators.RollupTriggerCalculatorByConflationCount
import lineth.conflation.calculators.RollupTriggerCalculatorByDataLimit
import lineth.conflation.calculators.RollupTriggerCalculatorByGas
import lineth.conflation.calculators.RollupTriggerCalculatorByTargetBlockNumbers
import lineth.conflation.calculators.RollupTriggerCalculatorByTxCount
import lineth.coordinator.config.v2.ConflationConfig
import lineth.coordinator.config.v2.ForcedTransactionsConfig
import lineth.coordinator.config.v2.L1SubmissionConfig
import lineth.persistence.AggregationsRepository
import lineth.persistence.BatchesRepository
import lineth.persistence.BlobsRepository
import lineth.persistence.RiscvRollupsDao
import net.consensys.linea.async.AsyncRetryer
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration
import kotlin.time.Instant

object ConflationAppHelper {
  /**
   * Forced transactions are a rollup-only feature in the coordinator today: ForcedTransactionsApp reads
   * the contract through the rollup client, which fails against a validium contract. They are therefore
   * disabled under VALIDIUM even when enabled in config. Both the ForcedTransactionsApp guard and the
   * forced-transactions DAO selection in CoordinatorApp go through this, so the two always agree.
   *
   * The validium V2 contract itself enforces forced-transaction inclusion at finalization, so storing
   * a forced transaction on such a deployment halts finalization until coordinator support is added.
   * Do not grant FORCED_TRANSACTION_SENDER_ROLE on a validium chain with this coordinator.
   */
  internal fun forcedTransactionsEnabled(
    forcedTransactions: ForcedTransactionsConfig?,
    dataAvailability: L1SubmissionConfig.DataAvailability,
  ): Boolean =
    forcedTransactions?.disabled == false &&
      dataAvailability != L1SubmissionConfig.DataAvailability.VALIDIUM

  /**
   * Returns the last block number inclusive upto which we have consecutive proven blobs or the last finalized block
   * number inclusive
   */
  internal fun resumeConflationFrom(
    aggregationsRepository: AggregationsRepository,
    lastFinalizedBlock: ULong,
  ): SafeFuture<ULong> {
    return aggregationsRepository
      .findConsecutiveProvenBlobs(lastFinalizedBlock.toLong() + 1)
      .thenApply { blobAndBatchCounters ->
        if (blobAndBatchCounters.isNotEmpty()) {
          blobAndBatchCounters.last().blobCounters.endBlockNumber
        } else {
          lastFinalizedBlock
        }
      }
  }

  private fun resumeAggregationFrom(
    aggregationsRepository: AggregationsRepository,
    lastFinalizedBlock: ULong,
  ): SafeFuture<ULong> {
    return aggregationsRepository
      .findHighestConsecutiveEndBlockNumber(lastFinalizedBlock.toLong() + 1)
      .thenApply { highestEndBlockNumber ->
        highestEndBlockNumber?.toULong() ?: lastFinalizedBlock
      }
  }

  fun getLastConflatedAndAggregatedBlocks(
    lastFinalizedBlock: ULong,
    aggregationsRepository: AggregationsRepository,
    l2EthClient: EthApiClient,
  ): SafeFuture<LastProcessedBlocks> {
    val lastConflatedBlock = resumeConflationFrom(
      aggregationsRepository,
      lastFinalizedBlock,
    ).thenCompose { lastProcessedBlockNumber ->
      l2EthClient.ethGetBlockByNumberTxHashes(
        lastProcessedBlockNumber.toBlockParameter(),
      )
    }
    val lastAggregatedBlock = resumeAggregationFrom(
      aggregationsRepository,
      lastFinalizedBlock,
    ).thenCompose { lastConsecutiveAggregatedBlockNumber ->
      l2EthClient.ethGetBlockByNumberTxHashes(
        lastConsecutiveAggregatedBlockNumber.toBlockParameter(),
      )
    }

    return SafeFuture.collectAll(lastConflatedBlock, lastAggregatedBlock)
      .thenApply { blocks ->
        LastProcessedBlocks(lastConflatedBlock = blocks.first(), lastAggregatedBlock = blocks.last())
      }
  }

  /**
   * Whether L1 has finalized every pre-RISC-V block, so only the RISC-V pipeline has work left: the last finalized
   * block is at or after the cutover, or it is the last pre-RISC-V block (the next block is at or after the cutover).
   */
  fun isRiscvCutoverFinalized(
    lastFinalizedBlock: ULong,
    lastFinalizedBlockTimestamp: Instant,
    riscvCutoverTimestamp: Instant,
    l2EthClient: EthApiClient,
  ): SafeFuture<Boolean> {
    if (lastFinalizedBlockTimestamp >= riscvCutoverTimestamp) return SafeFuture.completedFuture(true)
    return l2EthClient.ethFindBlockByNumberTxHashes((lastFinalizedBlock + 1UL).toBlockParameter())
      .thenApply { next -> next != null && next.timestamp.toInstantFromEpochSeconds() >= riscvCutoverTimestamp }
  }

  /** Blob cut-offs of the rollup stage, besides the blob deadline. */
  fun rollupBlobTriggers(config: ConflationConfig.Rollup): List<BlobTriggerCalculator> =
    buildList {
      if (config.blobTargetEndBlocks.isNotEmpty()) {
        add(
          BlobTriggerCalculatorByTargetBlockNumbers(config.blobTargetEndBlocks),
        )
      }
    }

  /** Rollup cut-offs of the rollup stage, besides the rollup deadline. */
  fun rollupTriggers(config: ConflationConfig.Rollup): List<RollupTriggerCalculator> =
    buildList {
      config.conflationsLimit?.let { add(RollupTriggerCalculatorByConflationCount(it.toInt())) }
      if (config.targetEndBlocks.isNotEmpty()) add(RollupTriggerCalculatorByTargetBlockNumbers(config.targetEndBlocks))
      config.dataLimit?.let { add(RollupTriggerCalculatorByDataLimit(it)) }
      config.gasLimit?.let { add(RollupTriggerCalculatorByGas(it)) }
      config.txCountLimit?.let { add(RollupTriggerCalculatorByTxCount(it)) }
    }

  /**
   * A rollup stage on a live pre-RISC-V to RISC-V transition requires ConflationAppV2 to pause at the cutover until L1
   * has finalized every pre-RISC-V block. RISC-V and pre-RISC-V rows share the batches table, and the pre-RISC-V resume
   * and cleanup would otherwise walk into RISC-V batches on a restart before that point.
   */
  fun validateRiscvRollupTransition(
    rollupConfigured: Boolean,
    riscvCutoverConfigured: Boolean,
    riscvCutoverCrossed: Boolean,
    waitTargetBlockL1Finalization: Boolean,
  ) {
    require(!rollupConfigured || riscvCutoverConfigured) {
      "conflation.rollup requires conflation.riscv-starting-block-timestamp-inclusive"
    }
    require(!rollupConfigured || riscvCutoverCrossed || waitTargetBlockL1Finalization) {
      "conflation.rollup before the RISC-V cutover requires " +
        "conflation.proof-aggregation.wait-target-block-l1-finalization=true"
    }
  }

  /**
   * Completes with the last L1 finalized block once L1 has finalized every pre-RISC-V block (see
   * [isRiscvCutoverFinalized]), polling [latestL1FinalizedBlock] every [pollingInterval].
   */
  fun awaitRiscvCutoverFinalized(
    vertx: Vertx,
    pollingInterval: Duration,
    riscvCutoverTimestamp: Instant,
    latestL1FinalizedBlock: () -> ULong,
    l2EthClient: EthApiClient,
  ): SafeFuture<BlockWithTxHashes> =
    AsyncRetryer.retry<BlockWithTxHashes?>(
      vertx = vertx,
      backoffDelay = pollingInterval,
      stopRetriesPredicate = { it != null },
    ) {
      l2EthClient.ethGetBlockByNumberTxHashes(latestL1FinalizedBlock().toBlockParameter()).thenCompose { finalized ->
        isRiscvCutoverFinalized(
          lastFinalizedBlock = finalized.number,
          lastFinalizedBlockTimestamp = finalized.timestamp.toInstantFromEpochSeconds(),
          riscvCutoverTimestamp = riscvCutoverTimestamp,
          l2EthClient = l2EthClient,
        ).thenApply { done -> finalized.takeIf { done } }
      }
    }.thenApply { it!! }

  /**
   * Where the RISC-V pipeline resumes after the cutover, all from the same last finalized block:
   * - conflation after the highest consecutive persisted batch from the last finalized block + 1, so already proven
   *   batches are kept;
   * - the rollup stage after the highest consecutive rollup row from the last finalized block + 1;
   * - aggregation at the last finalized block, as there is no RISC-V aggregation yet.
   * Each falls back to the last finalized block when nothing follows it.
   */
  fun getLastRiscvProcessedBlocks(
    lastFinalizedBlock: ULong,
    batchesRepository: BatchesRepository,
    rollupsDao: RiscvRollupsDao,
    l2EthClient: EthApiClient,
  ): SafeFuture<RiscvLastProcessedBlocks> {
    val lastConflatedBlock = batchesRepository
      .findHighestConsecutiveEndBlockNumberFromBlockNumber(lastFinalizedBlock.toLong() + 1)
      .thenCompose { highestEnd ->
        l2EthClient.ethGetBlockByNumberTxHashes((highestEnd?.toULong() ?: lastFinalizedBlock).toBlockParameter())
      }
    val lastAggregatedBlock = l2EthClient.ethGetBlockByNumberTxHashes(lastFinalizedBlock.toBlockParameter())
    val lastRollupEndBlockNumber = resumeRiscvRollupFrom(rollupsDao, lastFinalizedBlock)
    return SafeFuture.collectAll(lastConflatedBlock, lastAggregatedBlock)
      .thenCombine(lastRollupEndBlockNumber) { blocks, lastRollupEnd ->
        RiscvLastProcessedBlocks(
          lastConflatedBlock = blocks.first(),
          lastAggregatedBlock = blocks.last(),
          lastRollupEndBlockNumber = lastRollupEnd,
        )
      }
  }

  /**
   * End block of the last consecutive RISC-V rollup row from the last finalized block + 1, or the last finalized block
   * if there is none.
   */
  internal fun resumeRiscvRollupFrom(
    rollupsDao: RiscvRollupsDao,
    lastFinalizedBlock: ULong,
  ): SafeFuture<ULong> {
    return rollupsDao
      .findHighestConsecutiveEndBlockNumberFromBlockNumber(lastFinalizedBlock + 1UL)
      .thenApply { it ?: lastFinalizedBlock }
  }

  /**
   * Deletes what the RISC-V pipeline will rebuild: batches after [lastConflatedBlockNumber], rollup rows after
   * [lastRollupEndBlockNumber] and aggregations after [lastAggregatedBlockNumber].
   */
  fun cleanupRiscvDbDataAfterBlockNumbers(
    lastConflatedBlockNumber: ULong,
    lastRollupEndBlockNumber: ULong,
    lastAggregatedBlockNumber: ULong,
    batchesRepository: BatchesRepository,
    rollupsDao: RiscvRollupsDao,
    aggregationsRepository: AggregationsRepository,
  ): SafeFuture<*> {
    if (lastRollupEndBlockNumber > lastConflatedBlockNumber) {
      return SafeFuture.failedFuture<Unit>(
        IllegalStateException(
          "last rollup ends at block $lastRollupEndBlockNumber, after the last persisted batch " +
            "$lastConflatedBlockNumber: rollups are only built from persisted batches",
        ),
      )
    }
    return SafeFuture.allOf(
      batchesRepository.deleteBatchesAfterBlockNumber((lastConflatedBlockNumber + 1UL).toLong()),
      rollupsDao.deleteRollupsAfterBlockNumber(lastRollupEndBlockNumber + 1UL),
      aggregationsRepository.deleteAggregationsAfterBlockNumber((lastAggregatedBlockNumber + 1UL).toLong()),
    )
  }

  /** Removes the RISC-V prover request files past each stage's resume point. */
  fun cleanupRiscvProverRequests(
    l2ExecutionProverClient: L2ExecutionProverClientV1,
    rollupProverClient: RollupProverClientV1,
    rollupAggregationProverClient: RollupAggregationProverClientV1,
    lastConflatedBlockNumber: ULong,
    lastRollupEndBlockNumber: ULong,
    lastAggregatedBlockNumber: ULong,
  ): SafeFuture<*> {
    return SafeFuture.allOf(
      l2ExecutionProverClient.removeRequests((lastConflatedBlockNumber + 1UL).toLong()),
      rollupProverClient.removeRequests((lastRollupEndBlockNumber + 1UL).toLong()),
      rollupAggregationProverClient.removeRequests((lastAggregatedBlockNumber + 1UL).toLong()),
    )
  }

  fun cleanupDbDataAfterBlockNumbers(
    lastProcessedBlockNumber: ULong,
    lastConsecutiveAggregatedBlockNumber: ULong,
    batchesRepository: BatchesRepository,
    blobsRepository: BlobsRepository,
    aggregationsRepository: AggregationsRepository,
  ): SafeFuture<*> {
    val blockNumberInclusiveToDeleteFrom = lastProcessedBlockNumber + 1u
    val cleanupBatches = batchesRepository.deleteBatchesAfterBlockNumber(blockNumberInclusiveToDeleteFrom.toLong())
    val cleanupBlobs = blobsRepository.deleteBlobsAfterBlockNumber(blockNumberInclusiveToDeleteFrom)
    val cleanupAggregations =
      aggregationsRepository
        .deleteAggregationsAfterBlockNumber((lastConsecutiveAggregatedBlockNumber + 1u).toLong())

    return SafeFuture.allOf(cleanupBatches, cleanupBlobs, cleanupAggregations)
  }
}

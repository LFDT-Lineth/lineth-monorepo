/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import maru.consensus.PrevRandaoProvider
import maru.consensus.qbft.adapters.toBeaconBlockHeader
import maru.consensus.state.FinalizationProvider
import maru.database.BeaconChain
import maru.executionlayer.manager.ExecutionLayerManager
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlock
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlockCreator
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlockHeader
import org.hyperledger.besu.crypto.SECPSignature
import kotlin.time.Duration

/**
 * Responsible for QBFT block creation. As opposed to DelayedQbftBlockCreator, Eager one will send the FCU request to
 * the execution client to start the block building process and will wait for the required time until a block is built,
 * blocking the thread
 */
class EagerQbftBlockCreator(
  private val manager: ExecutionLayerManager,
  private val delegate: QbftBlockCreator,
  private val finalizationStateProvider: FinalizationProvider,
  private val prevRandaoProvider: PrevRandaoProvider<ULong>,
  private val feeRecipient: ByteArray,
  private val beaconChain: BeaconChain,
  private val config: Config,
) : QbftBlockCreator {
  private val log: Logger = LogManager.getLogger(this.javaClass)

  data class Config(
    val minBlockBuildTime: Duration,
  )

  override fun createBlock(
    headerTimeStampSeconds: Long,
    parentHeader: QbftBlockHeader,
  ): QbftBlockCreator.BlockCreationResult {
    val beaconBlockHeader = parentHeader.toBeaconBlockHeader()
    val parentBeaconBlock =
      beaconChain
        .getSealedBeaconBlock(beaconBlockHeader.hash)
        ?.beaconBlock
        ?: throw IllegalStateException("Parent block not found in the database")
    val finalizedState = finalizationStateProvider(parentBeaconBlock.beaconBlockBody)
    // If the parent beacon block is genesis, we use the latest EL block as head: at the QBFT->PoS
    // handover the beacon parent is genesis (timestamp 0) but the EL has already produced blocks
    // pre-merge, so we need the actual EL head. Otherwise we follow the executionPayload pinned
    // to the parent beacon block.
    val (elHeadHash, elHeadTimestampSeconds) =
      if (parentBeaconBlock.beaconBlockHeader.number == 0UL) {
        manager.getLatestBlockMetadata().get().let { it.blockHash to it.timestamp }
      } else {
        parentBeaconBlock.beaconBlockBody.executionPayload.let { it.blockHash to it.timestamp }
      }

    // Engine API requires payloadAttributes.timestamp > headBlockHeader.timestamp (see Besu's
    // AbstractEngineForkchoiceUpdated.isPayloadAttributeRelevantToNewHead). The QBFT scheduler
    // computes headerTimeStampSeconds via Math.round(clock.millis()/1000D) without clamping
    // against the EL parent, so at the merge handover (and on rounding edges in general) the
    // proposed timestamp can equal the EL head timestamp, which Besu rejects with
    // INVALID_PAYLOAD_ATTRIBUTES (or INVALID_WITHDRAWALS_PARAMS on V1). Clamping here makes the
    // timestamp strictly greater than the EL head, mirroring what Clique's DefaultBlockScheduler
    // does for non-merge consensus.
    val safeTimestampSeconds = maxOf(headerTimeStampSeconds, elHeadTimestampSeconds.toLong() + 1L)
    if (safeTimestampSeconds != headerTimeStampSeconds) {
      log.debug(
        "Clamped next block timestamp from {} to {} (EL head timestamp={})",
        headerTimeStampSeconds,
        safeTimestampSeconds,
        elHeadTimestampSeconds,
      )
    }

    val blockBuildingTriggerResult =
      manager
        .setHeadAndStartBlockBuilding(
          headHash = elHeadHash,
          safeHash = finalizedState.safeBlockHash,
          finalizedHash = finalizedState.finalizedBlockHash,
          nextBlockTimestamp = safeTimestampSeconds.toULong(),
          feeRecipient = feeRecipient,
          prevRandao = prevRandaoProvider.calculateNextPrevRandao(
            signee = parentBeaconBlock.beaconBlockBody.executionPayload.blockNumber
              .inc(),
            prevRandao = parentBeaconBlock.beaconBlockBody.executionPayload.prevRandao,
          ),
        ).get()
    log.debug(
      "Building new block, FCU result={}",
      blockBuildingTriggerResult,
    )
    val sleepTime = config.minBlockBuildTime.inWholeMilliseconds
    log.debug("Block building has started, sleeping for {} milliseconds", sleepTime)
    Thread.sleep(sleepTime)
    log.debug("Block building has finished, time to collect block building results")
    return delegate.createBlock(safeTimestampSeconds, parentHeader)
  }

  override fun createSealedBlock(
    block: QbftBlock,
    roundNumber: Int,
    commitSeals: Collection<SECPSignature>,
  ): QbftBlock = DelayedQbftBlockCreator.createSealedBlock(block, roundNumber, commitSeals)
}

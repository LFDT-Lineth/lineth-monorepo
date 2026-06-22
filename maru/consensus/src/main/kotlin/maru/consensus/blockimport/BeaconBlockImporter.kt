/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.blockimport

import maru.consensus.NewBlockHandler
import maru.consensus.NextBlockTimestampProvider
import maru.consensus.PrevRandaoProvider
import maru.consensus.state.FinalizationProvider
import maru.core.BeaconBlock
import maru.core.BeaconState
import maru.executionlayer.manager.ExecutionLayerManager
import maru.executionlayer.manager.ForkChoiceUpdatedResult
import maru.p2p.ValidationResult
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.hyperledger.besu.consensus.common.bft.ConsensusRoundIdentifier
import tech.pegasys.teku.infrastructure.async.SafeFuture

fun interface BeaconBlockImporter {
  fun importBlock(
    beaconState: BeaconState,
    beaconBlock: BeaconBlock,
  ): SafeFuture<*>
}

/**
 * A block importer that calls engine_newPayload before delegating to [delegate] for the
 * engine_forkchoiceUpdated (setHead) step. This is the "full" follower import path used for
 * EL nodes that have not yet validated the block themselves.
 *
 * Decorator pattern: wraps any [NewBlockHandler]<[ValidationResult]> and prepends the
 * newPayload step. The default [create] factory composes it with [SetHeadOnlyBlockImporter].
 */
class FollowerBeaconBlockImporter(
  private val executionLayerManager: ExecutionLayerManager,
  private val delegate: NewBlockHandler<ValidationResult>,
  private val importerName: String,
) : NewBlockHandler<ValidationResult> {
  companion object {
    fun create(
      executionLayerManager: ExecutionLayerManager,
      finalizationStateProvider: FinalizationProvider,
      importerName: String,
    ): NewBlockHandler<ValidationResult> =
      FollowerBeaconBlockImporter(
        executionLayerManager = executionLayerManager,
        delegate = SetHeadOnlyBlockImporter(
          executionLayerManager = executionLayerManager,
          finalizationStateProvider = finalizationStateProvider,
          importerName = importerName,
        ),
        importerName = importerName,
      )
  }

  private val log = LogManager.getLogger(this.javaClass)

  override fun handleNewBlock(beaconBlock: BeaconBlock): SafeFuture<ValidationResult> {
    val executionPayload = beaconBlock.beaconBlockBody.executionPayload
    return executionLayerManager
      .newPayload(executionPayload)
      .handleException { e ->
        log.error(
          "Error importing execution payload to {} for elBlockNumber={}",
          importerName,
          executionPayload.blockNumber,
          e,
        )
      }.thenCompose {
        delegate.handleNewBlock(beaconBlock)
      }
  }
}

/**
 * A block importer that only calls engine_forkchoiceUpdated (setHead) without calling engine_newPayload.
 * Used when engine_newPayload was already called during block validation (e.g., PROPOSAL validation
 * in the QBFT follower path), so calling it again during block import would be redundant.
 */
class SetHeadOnlyBlockImporter(
  private val executionLayerManager: ExecutionLayerManager,
  private val finalizationStateProvider: FinalizationProvider,
  private val importerName: String,
) : NewBlockHandler<ValidationResult> {
  private val log = LogManager.getLogger(this.javaClass)

  override fun handleNewBlock(beaconBlock: BeaconBlock): SafeFuture<ValidationResult> {
    val executionPayload = beaconBlock.beaconBlockBody.executionPayload
    val finalizationState = finalizationStateProvider(beaconBlock.beaconBlockBody)
    return executionLayerManager
      .setHead(
        headHash = executionPayload.blockHash,
        safeHash = finalizationState.safeBlockHash,
        finalizedHash = finalizationState.finalizedBlockHash,
      ).thenApply {
        log.debug("Set head for elBlockNumber={} on {}", executionPayload.blockNumber, importerName)
        ValidationResult.fromForkChoiceUpdatedResult(it)
      }
  }
}

class BlockBuildingBeaconBlockImporter(
  private val executionLayerManager: ExecutionLayerManager,
  private val finalizationStateProvider: FinalizationProvider,
  private val nextBlockTimestampProvider: NextBlockTimestampProvider,
  private val prevRandaoProvider: PrevRandaoProvider<ULong>,
  private val shouldBuildNextBlock: (BeaconState, ConsensusRoundIdentifier, ULong) -> Boolean,
  private val feeRecipient: ByteArray,
) : BeaconBlockImporter {
  private val log: Logger = LogManager.getLogger(this.javaClass)

  override fun importBlock(
    beaconState: BeaconState,
    beaconBlock: BeaconBlock,
  ): SafeFuture<ForkChoiceUpdatedResult> {
    val beaconBlockHeader = beaconBlock.beaconBlockHeader
    val finalizationState = finalizationStateProvider(beaconBlock.beaconBlockBody)
    val nextBlocksRoundIdentifier = ConsensusRoundIdentifier(beaconBlockHeader.number.toLong() + 1, 0)
    val nextBlockTimestamp =
      nextBlockTimestampProvider.nextTargetBlockUnixTimestamp(
        beaconState.beaconBlockHeader.timestamp,
      )
    return if (shouldBuildNextBlock(beaconState, nextBlocksRoundIdentifier, nextBlockTimestamp)) {
      log.info(
        "importing block and starting build next block: " +
          "elBlockNumber={} elBlockTimestamp={} elNextBlockTimestamp={} beaconBlockHeader={}",
        beaconBlock.beaconBlockBody.executionPayload.blockNumber,
        beaconBlock.beaconBlockBody.executionPayload.timestamp,
        nextBlockTimestamp,
        beaconBlockHeader,
      )
      executionLayerManager.setHeadAndStartBlockBuilding(
        headHash = beaconBlock.beaconBlockBody.executionPayload.blockHash,
        safeHash = finalizationState.safeBlockHash,
        finalizedHash = finalizationState.finalizedBlockHash,
        nextBlockTimestamp = nextBlockTimestamp,
        feeRecipient = feeRecipient,
        prevRandao = prevRandaoProvider.calculateNextPrevRandao(
          signee = beaconBlock.beaconBlockBody.executionPayload.blockNumber
            .inc(),
          prevRandao = beaconBlock.beaconBlockBody.executionPayload.prevRandao,
        ),
      )
    } else {
      log.info(
        "importing block: elBlockNumber={} clBlockNumber={} clBlockHeader={}",
        beaconBlock.beaconBlockBody.executionPayload.blockNumber,
        beaconBlockHeader.number,
        beaconBlockHeader,
      )
      executionLayerManager.setHead(
        headHash = beaconBlock.beaconBlockBody.executionPayload.blockHash,
        safeHash = finalizationState.safeBlockHash,
        finalizedHash = finalizationState.finalizedBlockHash,
      )
    }
  }
}

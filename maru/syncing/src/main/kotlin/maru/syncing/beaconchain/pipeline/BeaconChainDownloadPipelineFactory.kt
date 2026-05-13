/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.syncing.beaconchain.pipeline

import maru.consensus.blockimport.SealedBeaconBlockImporter
import maru.extensions.clampedAdd
import maru.p2p.PeerLookup
import maru.p2p.ValidationResult
import maru.p2p.messages.BeaconBlocksByRangeHandler.Companion.MAX_BLOCKS_PER_REQUEST
import org.hyperledger.besu.metrics.BesuMetricCategory
import org.hyperledger.besu.plugin.services.MetricsSystem
import org.hyperledger.besu.services.pipeline.Pipeline
import org.hyperledger.besu.services.pipeline.PipelineBuilder
import kotlin.time.Duration

data class BeaconChainPipeline(
  val pipeline: Pipeline<SyncTargetRange>,
  val target: () -> ULong,
)

class BeaconChainDownloadPipelineFactory(
  private val blockImporter: SealedBeaconBlockImporter<ValidationResult>,
  private val metricsSystem: MetricsSystem,
  private val peerLookup: PeerLookup,
  private val config: Config,
  private val syncTargetProvider: () -> ULong,
) {
  init {
    require(config.blocksBatchSize > 0u) { "Request size must be greater than 0" }
    require(config.blocksBatchSize <= MAX_BLOCKS_PER_REQUEST) {
      "Request size must not be greater than $MAX_BLOCKS_PER_REQUEST"
    }
    require(config.maxRetries > 0u) { "Maxi retries must be greater than 0" }
    require(config.blockRangeRequestTimeout.isPositive()) { "Block range request timeout must be positive" }
    require(config.blocksParallelism > 0u) { "Blocks download parallelism must be greater than 0" }
  }

  data class Config(
    val blockRangeRequestTimeout: Duration,
    val backoffDelay: Duration,
    val blocksBatchSize: UInt,
    val blocksParallelism: UInt,
    val maxRetries: UInt,
    val useUnconditionalRandomDownloadPeer: Boolean,
  )

  fun createPipeline(startBlock: ULong): BeaconChainPipeline {
    var latestEndBlock = startBlock

    val syncTargetRangeSequence =
      sequence {
        var currentStart = startBlock
        val rangeSize = config.blocksBatchSize.toULong()

        var syncTarget = syncTargetProvider()
        while (currentStart <= syncTarget) {
          val nextEnd = currentStart.clampedAdd(rangeSize) - 1uL
          val currentEnd = minOf(nextEnd, syncTarget)
          latestEndBlock = currentEnd
          yield(SyncTargetRange(currentStart, currentEnd))
          currentStart = currentEnd + 1uL
          syncTarget = syncTargetProvider()
        }
      }

    val downloadBlocksStep =
      DownloadBlocksStep(
        downloadPeerProvider = DownloadPeerProviderImpl(
          peerLookup = peerLookup,
          useUnconditionalRandomSelection = config.useUnconditionalRandomDownloadPeer,
        ),
        config = DownloadBlocksStep.Config(
          maxRetries = config.maxRetries,
          blockRangeRequestTimeout = config.blockRangeRequestTimeout,
          backoffDelay = config.backoffDelay,
        ),
      )
    val importBlocksStep = ImportBlocksStep(blockImporter)

    val pipeline =
      PipelineBuilder
        .createPipelineFrom(
          "blockNumbers",
          syncTargetRangeSequence.iterator(),
          config.blocksParallelism.toInt(),
          metricsSystem.createLabelledCounter(
            BesuMetricCategory.SYNCHRONIZER,
            "chain_download_pipeline_processed_total",
            "Number of entries process by each chain download pipeline stage",
            "step",
            "action",
          ),
          true,
          "importBlocks",
        ).thenProcessAsyncOrdered("downloadBlocks", downloadBlocksStep, config.blocksParallelism.toInt())
        .andFinishWith("importBlocks", importBlocksStep)

    return BeaconChainPipeline(
      pipeline = pipeline,
      target = {
        // If we started beyond the sync target, return the sync target (we're already synced to it)
        val currentSyncTarget = syncTargetProvider()
        if (startBlock > currentSyncTarget) currentSyncTarget else latestEndBlock
      },
    )
  }
}

/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.finalization

import linea.contract.l1.LineaRollupSmartContractClientReadOnly
import linea.domain.BlockData
import linea.domain.BlockParameter
import linea.domain.BlockParameter.Companion.toBlockParameter
import linea.ethapi.EthApiClient
import linea.kotlin.encodeHex
import linea.timer.PeriodicPollingService
import linea.timer.TimerFactory
import linea.timer.TimerSchedule
import maru.consensus.state.FinalizationProvider
import maru.consensus.state.FinalizationState
import maru.core.BeaconBlockBody
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicReference
import kotlin.time.Duration

class LineaFinalizationProvider(
  private val lineaContract: LineaRollupSmartContractClientReadOnly,
  private val l2EthApi: EthApiClient,
  private val pollingUpdateInterval: Duration,
  private val l1HighestBlock: BlockParameter = BlockParameter.Tag.FINALIZED,
  timerFactory: TimerFactory,
  private val log: Logger = LogManager.getLogger(LineaFinalizationProvider::class.java),
) : PeriodicPollingService(
  name = "l1-finalization-poller",
  timerFactory = timerFactory,
  timerSchedule = TimerSchedule.FIXED_RATE,
  pollingInterval = pollingUpdateInterval,
  log = log,
),
  FinalizationProvider {
  private data class BlockHeader(
    val blockNumber: ULong,
    val hash: ByteArray,
  ) {
    override fun equals(other: Any?): Boolean {
      if (this === other) return true
      if (javaClass != other?.javaClass) return false

      other as BlockHeader

      if (blockNumber != other.blockNumber) return false
      if (!hash.contentEquals(other.hash)) return false

      return true
    }

    override fun hashCode(): Int {
      var result = blockNumber.hashCode()
      result = 31 * result + hash.contentHashCode()
      return result
    }
  }

  private val lastFinalizedBlock: AtomicReference<BlockHeader>

  init {
    // initialize the last finalized block with the latest finalized block on L2
    l2EthApi
      .ethGetBlockByNumberTxHashes(BlockParameter.Tag.FINALIZED)
      .exceptionallyCompose { error ->
        log.error("failed to get FINALIZED block, will fallback to EARLIEST. error={}", error.message)
        l2EthApi.ethGetBlockByNumberTxHashes(BlockParameter.Tag.EARLIEST)
      }.get()
      .also { block ->
        lastFinalizedBlock = AtomicReference(
          BlockHeader(
            blockNumber = block.number,
            hash = block.hash,
          ),
        )
      }
  }

  override fun start(): SafeFuture<Unit> {
    log.debug("Starting LineaFinalizationProvider with polling interval: {}", pollingUpdateInterval)
    return super.start()
  }

  override fun action(): SafeFuture<Unit> =
    getFinalizationUpdate().thenApply { finalization ->
      lastFinalizedBlock.set(finalization)
    }

  private fun getFinalizationUpdate(): SafeFuture<BlockHeader> =
    lineaContract
      .finalizedL2BlockNumber(l1HighestBlock)
      .thenCompose { finalizedBlockNumber ->
        getHighestBlockAvailableUpToBlock(finalizedBlockNumber)
          .thenPeek { block ->
            log.debug(
              "finalization update: finalizedBlockOnL1={} prevCachedFinalizedBlock={} newCachedFinalizedBlock={} " +
                "prevFinalizedBlockHash={} newFinalizedBlockHash={} {}",
              finalizedBlockNumber,
              lastFinalizedBlock.get().blockNumber,
              block.number,
              lastFinalizedBlock.get().hash.encodeHex(),
              block.hash.encodeHex(),
              if (finalizedBlockNumber > block.number) {
                "(node is behind, using latest available block)"
              } else {
                ""
              },
            )
          }
      }.thenApply { block ->
        BlockHeader(
          blockNumber = block.number,
          hash = block.hash,
        )
      }

  private fun getHighestBlockAvailableUpToBlock(finalizedBlockNumber: ULong): SafeFuture<BlockData<ByteArray>> =
    l2EthApi
      .ethFindBlockByNumberTxHashes(finalizedBlockNumber.toBlockParameter())
      .thenCompose { block ->
        if (block == null) {
          // If this node is behind (syncing) won't have the block locally to retrieve its hash
          // until it catches up, we will default to the lastest available block while it catches up until
          // the finalized block
          l2EthApi
            .ethGetBlockByNumberTxHashes(BlockParameter.Tag.LATEST)
            .thenCompose { latestBlock ->
              if (latestBlock.number > finalizedBlockNumber) {
                // this that in meantime the code has caught up to the finalized block,
                // let's fetch it again
                l2EthApi
                  .ethGetBlockByNumberTxHashes(finalizedBlockNumber.toBlockParameter())
              } else {
                SafeFuture.completedFuture(latestBlock)
              }
            }
        } else {
          SafeFuture.completedFuture(block)
        }
      }

  override fun invoke(beaconBlock: BeaconBlockBody): FinalizationState =
    lastFinalizedBlock
      .get()
      .let { finalizedBlock ->
        log.debug(
          "finalization state: finalizedBlock={} beaconBlock={}",
          finalizedBlock.blockNumber,
          beaconBlock.executionPayload.blockNumber,
        )

        FinalizationState(
          safeBlockHash = finalizedBlock.hash,
          finalizedBlockHash = finalizedBlock.hash,
        )
      }
}

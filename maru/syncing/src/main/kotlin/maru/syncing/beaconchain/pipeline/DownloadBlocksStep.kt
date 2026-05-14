/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.syncing.beaconchain.pipeline

import maru.core.SealedBeaconBlock
import maru.p2p.MaruPeer
import maru.p2p.PeerLookup
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.hyperledger.besu.util.log.LogUtil
import tech.pegasys.teku.networking.eth2.rpc.core.RpcException
import tech.pegasys.teku.networking.p2p.peer.DisconnectReason
import tech.pegasys.teku.networking.p2p.reputation.ReputationAdjustment
import java.lang.Thread.sleep
import java.util.concurrent.CompletableFuture
import java.util.concurrent.TimeUnit
import java.util.concurrent.TimeoutException
import java.util.concurrent.atomic.AtomicBoolean
import java.util.function.Function
import kotlin.time.Duration

interface DownloadPeerProvider {
  fun getDownloadingPeer(downloadRangeEndBlockNumber: ULong): MaruPeer?
}

class DownloadPeerProviderImpl(
  private val peerLookup: PeerLookup,
  private val useUnconditionalRandomSelection: Boolean,
) : DownloadPeerProvider {
  override fun getDownloadingPeer(downloadRangeEndBlockNumber: ULong): MaruPeer? {
    val eligiblePeers =
      peerLookup.getPeers().let { peers ->
        if (useUnconditionalRandomSelection) {
          peers
        } else {
          peers.filter { peer ->
            // TODO: consider reducing the downloadRangeEndBlockNumber because peer status is only periodically updated.
            peer.getStatus()?.latestBlockNumber?.let { it >= downloadRangeEndBlockNumber } == true
          }
        }
      }
    return eligiblePeers.randomOrNull()
  }
}

class DownloadBlocksStep(
  private val downloadPeerProvider: DownloadPeerProvider,
  private val config: Config,
) : Function<SyncTargetRange, CompletableFuture<List<SealedBlockWithPeer>>> {
  class MaxRetriesReachedException(
    message: String,
  ) : Exception(message)

  data class Config(
    val maxRetries: UInt,
    val blockRangeRequestTimeout: Duration,
    val backoffDelay: Duration,
  )

  private val log: Logger = LogManager.getLogger(this.javaClass)
  private val shouldLog = AtomicBoolean(true)

  override fun apply(targetRange: SyncTargetRange): CompletableFuture<List<SealedBlockWithPeer>> =
    CompletableFuture.supplyAsync { downloadBlocks(targetRange) }

  private data class DownloadState(
    val startBlockNumber: ULong,
    val remaining: ULong,
    val retries: UInt,
    val downloadedBlocks: MutableList<SealedBlockWithPeer>,
  )

  private fun downloadBlocks(targetRange: SyncTargetRange): List<SealedBlockWithPeer> {
    val totalCount = targetRange.endBlock - targetRange.startBlock + 1uL
    var state =
      DownloadState(
        startBlockNumber = targetRange.startBlock,
        remaining = totalCount,
        retries = 0u,
        downloadedBlocks = mutableListOf(),
      )

    LogUtil.throttledLog(
      log::info,
      "Downloading blocks: start clBlockNumber=${state.startBlockNumber} count=$totalCount",
      shouldLog,
      30,
    )

    while (state.downloadedBlocks.size.toULong() < totalCount) {
      state = when (val peer = downloadPeerProvider.getDownloadingPeer(targetRange.endBlock)) {
        null -> {
          sleep(config.backoffDelay.inWholeMilliseconds)
          state.copy(retries = state.retries + 1u)
        }

        else -> downloadFromPeer(peer, state)
      }
      checkMaxRetries(state.retries)
    }

    return state.downloadedBlocks
  }

  private fun checkMaxRetries(retries: UInt) {
    if (retries >= config.maxRetries) {
      log.debug("Maximum retries reached.")
      throw MaxRetriesReachedException("Maximum retries reached.")
    }
  }

  private fun downloadFromPeer(
    peer: MaruPeer,
    state: DownloadState,
  ): DownloadState =
    try {
      val response =
        peer
          .sendBeaconBlocksByRange(state.startBlockNumber, state.remaining)
          .get(config.blockRangeRequestTimeout.inWholeMilliseconds, TimeUnit.MILLISECONDS)
      when {
        response.blocks.isEmpty() -> handleEmptyResponse(peer, state)
        response.blocks.size.toULong() > state.remaining -> {
          peer.disconnectCleanly(DisconnectReason.REMOTE_FAULT)
          log.debug(
            "Received more blocks than requested from peer: {}. Expected: {}, Received: {}",
            peer.id,
            state.remaining,
            response.blocks.size,
          )
          state.copy(retries = state.retries + 1u)
        }

        else -> handleSuccessfulDownload(peer, state, response.blocks)
      }
    } catch (e: Exception) {
      handleDownloadException(e, peer, state)
    }

  private fun handleEmptyResponse(
    peer: MaruPeer,
    state: DownloadState,
  ): DownloadState {
    peer.adjustReputation(ReputationAdjustment.SMALL_PENALTY)
    log.debug("No blocks received from peer: {}", peer.id)
    return state.copy(retries = state.retries + 1u)
  }

  private fun handleSuccessfulDownload(
    peer: MaruPeer,
    state: DownloadState,
    blocks: List<SealedBeaconBlock>,
  ): DownloadState {
    val numBlocks = blocks.size.toULong()

    blocks.forEach { block ->
      state.downloadedBlocks.add(SealedBlockWithPeer(block, peer))
    }

    // peers are rewarded for providing blocks in ImportBlockStep, when we know whether the blocks were valid

    return state.copy(
      startBlockNumber = state.startBlockNumber + numBlocks,
      remaining = state.remaining - numBlocks,
      retries = 0u, // Reset retries on successful download
    )
  }

  private fun handleDownloadException(
    e: Exception,
    peer: MaruPeer,
    state: DownloadState,
  ): DownloadState {
    when {
      e is TimeoutException || e.cause is TimeoutException -> {
        log.debug("Timed out while downloading blocks from peer: {}", peer.id)
        peer.adjustReputation(ReputationAdjustment.LARGE_PENALTY)
      }

      e.cause is RpcException -> {
        log.warn("RpcException while downloading blocks from peer: {}", peer.id, e.cause)
        peer.adjustReputation(ReputationAdjustment.SMALL_PENALTY)
      }

      else -> {
        log.debug("Failed to download blocks from peer: {}", peer.id, e)
        sleep(config.backoffDelay.inWholeMilliseconds)
      }
    }
    return state.copy(retries = state.retries + 1u)
  }
}

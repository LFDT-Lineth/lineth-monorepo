/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.test.extensions

import maru.config.P2PConfig
import maru.config.QbftConfig
import maru.config.SyncingConfig
import maru.config.SyncingConfig.SyncTargetSelection
import maru.test.cluster.NodeBuilder
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

object TestSyncingConfigs {
  val default: SyncingConfig =
    SyncingConfig(
      peerChainHeightPollingInterval = 1.seconds,
      syncTargetSelection = SyncTargetSelection.Highest,
      elSyncStatusRefreshInterval = 500.milliseconds,
      // desyncTolerance intentionally uses the Config default (5 blocks) for followers.
      download = SyncingConfig.Download(),
    )

  /**
   * Validators need desyncTolerance=0 to avoid a race condition in Besu's
   * QbftController.consumeMessage: it discards QBFT messages where
   * sequenceNumber <= blockchain.chainHeadBlockNumber. When the sync pipeline imports multiple
   * blocks at once (desyncTolerance > 0), BeaconChain advances progressively on the sync thread
   * while the QBFT event processor concurrently dequeues gossiped messages — messages correctly
   * queued as "future" get discarded as "old" because the chain head raced past them.
   * With desyncTolerance=0, sync imports 1 block at a time, keeping the race window minimal.
   */
  val validator: SyncingConfig = default.copy(desyncTolerance = 0UL)

  fun enumerating(): List<SyncingConfig> =
    listOf(
      default,
      default.copy(
        syncTargetSelection = SyncTargetSelection.MostFrequent(peerChainHeightGranularity = 10U),
        download = SyncingConfig.Download(useUnconditionalRandomDownloadPeer = true),
      ),
    )
}

fun NodeBuilder.withSyncingConfig(syncingConfig: SyncingConfig): NodeBuilder =
  maruConfig { it.copy(syncing = syncingConfig) }

/** Peers only with the configured static peers */
fun NodeBuilder.withStaticPeeringOnly(): NodeBuilder = maruConfig { it.copy(p2p = it.p2p!!.copy(discovery = null)) }

/** Faster peering timings than the defaults, as everything runs on localhost */
fun NodeBuilder.withFastPeering(
  banPeriod: Duration = 2.seconds,
  cooldownPeriod: Duration = 1.seconds,
): NodeBuilder =
  maruConfig { config ->
    val p2p = config.p2p!!
    config.copy(
      p2p = p2p.copy(
        reconnectDelay = 500.milliseconds,
        statusUpdate = P2PConfig.StatusUpdate(refreshInterval = 1.seconds),
        reputation = P2PConfig.Reputation(banPeriod = banPeriod, cooldownPeriod = cooldownPeriod),
        discovery = p2p.discovery?.copy(refreshInterval = 1.seconds, retryTimeout = 500.milliseconds),
      ),
    )
  }

/**
 * Faster QBFT timings than the defaults. Only for sequencers: a QBFT config makes a node a validator.
 * The fee recipient is set by the cluster from the node key.
 */
fun NodeBuilder.withFastQbft(
  minBlockBuildTime: Duration = 200.milliseconds,
  roundExpiry: Duration = 1.seconds,
): NodeBuilder =
  maruConfig {
    it.copy(
      qbft = (it.qbft ?: QbftConfig(feeRecipient = ByteArray(20)))
        .copy(minBlockBuildTime = minBlockBuildTime, roundExpiry = roundExpiry),
    )
  }

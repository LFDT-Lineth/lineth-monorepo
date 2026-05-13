/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.syncing

import linea.timer.PeriodicPollingService
import linea.timer.TimerFactory
import linea.timer.TimerSchedule
import maru.database.BeaconChain
import maru.p2p.PeersHeadBlockProvider
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration

/**
 * polls periodically peers chain head
 * and notify SyncTargetSelector when chain head of any peer changes
 *
 * it only notifies syncTargetUpdateHandler when there is actual update, not on every tick/calculation
 *
 * MaruPeerManager -> PeerChainTracker -> SyncController
 */
class PeerChainTracker(
  private val peersHeadsProvider: PeersHeadBlockProvider,
  private val beaconSyncTargetUpdateHandler: BeaconSyncTargetUpdateHandler,
  private val targetChainHeadCalculator: SyncTargetSelector,
  private val config: Config,
  private val timerFactory: TimerFactory,
  private val beaconChain: BeaconChain,
  private val log: Logger = LogManager.getLogger(PeerChainTracker::class.java),
) : PeriodicPollingService(
  name = "peer-chain-tracker",
  timerFactory = timerFactory,
  timerSchedule = TimerSchedule.FIXED_RATE,
  pollingInterval = config.pollingUpdateInterval,
  log = log,
) {
  data class Config(
    val pollingUpdateInterval: Duration,
  )

  private var lastNotifiedTarget: ULong? = null // 0 is an Ok magic number, since it represents Genesis
  private var isRunning = false

  /**
   * Updates the peer view and triggers sync target updates if needed
   */
  override fun action(): SafeFuture<Unit> {
    log.trace("Updating peer view")

    val peersHeads = peersHeadsProvider.getPeersHeads().values.toList()

    // Update the state and recalculate the sync target
    val newSyncTarget =
      if (peersHeads.isNotEmpty()) {
        targetChainHeadCalculator.selectBestSyncTarget(peersHeads)
      } else {
        // If there are no peers, we return the chain head of current node, because we don't know better
        beaconChain.getLatestBeaconState().beaconBlockHeader.number
      }
    log.trace("Selected best syncTarget={} lastNotifiedTarget={}", newSyncTarget, lastNotifiedTarget)
    if (newSyncTarget != lastNotifiedTarget) { // Only send an update if there's an actual target change
      beaconSyncTargetUpdateHandler.onBeaconChainSyncTargetUpdated(newSyncTarget)
      log.debug("Notified about the new syncTarget={}", newSyncTarget)
      lastNotifiedTarget = newSyncTarget
    }
    return SafeFuture.completedFuture(Unit)
  }

  override fun stop(): SafeFuture<Unit> =
    super.stop().thenApply {
      lastNotifiedTarget = null
    }
}

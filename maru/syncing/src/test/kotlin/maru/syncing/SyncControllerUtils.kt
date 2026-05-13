/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.syncing

import maru.core.ext.DataGenerators
import maru.database.InMemoryBeaconChain

/**
 * Fake implementation of CLSyncService for testing purposes.
 */
class FakeCLSyncService : CLSyncService {
  var lastSyncTarget: ULong? = null
  private val syncCompleteHandlers = mutableListOf<(ULong) -> Unit>()

  override fun setSyncTarget(syncTarget: ULong) {
    lastSyncTarget = syncTarget
  }

  override fun onSyncComplete(handler: (ULong) -> Unit) {
    syncCompleteHandlers.add(handler)
  }

  override fun getSyncDistance(): ULong = 0UL

  override fun getSyncTarget(): ULong = 0UL

  fun triggerSyncComplete(syncTarget: ULong) {
    syncCompleteHandlers.forEach { it(syncTarget) }
  }
}

fun createSyncController(
  blockNumber: ULong,
  clSyncService: CLSyncService = FakeCLSyncService(),
  desyncTolerance: ULong,
  elSyncEnabled: Boolean = true,
): BeaconSyncControllerImpl {
  val state = DataGenerators.randomBeaconState(blockNumber)
  val beaconChain = InMemoryBeaconChain(state)
  return BeaconSyncControllerImpl(
    beaconChain = beaconChain,
    clSyncService = clSyncService,
    desyncTolerance = desyncTolerance,
    elSyncEnabled = elSyncEnabled,
  )
}

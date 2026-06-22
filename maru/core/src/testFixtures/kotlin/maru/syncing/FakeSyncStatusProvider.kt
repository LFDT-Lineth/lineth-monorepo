/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.syncing

class FakeSyncStatusProvider(
  var clStatus: CLSyncStatus = CLSyncStatus.SYNCED,
  var elStatus: ELSyncStatus = ELSyncStatus.SYNCED,
  var beaconSyncDistanceValue: ULong = 0UL,
  var clSyncTarget: ULong = 0UL,
) : SyncStatusProvider {
  override fun getCLSyncStatus(): CLSyncStatus = clStatus

  override fun getElSyncStatus(): ELSyncStatus = elStatus

  override fun onClSyncStatusUpdate(handler: (newStatus: CLSyncStatus) -> Unit) {}

  override fun onElSyncStatusUpdate(handler: (newStatus: ELSyncStatus) -> Unit) {}

  override fun isBeaconChainSynced(): Boolean = true

  override fun isELSynced(): Boolean = true

  override fun onBeaconSyncComplete(handler: () -> Unit) {}

  override fun onFullSyncComplete(handler: () -> Unit) {}

  override fun getBeaconSyncDistance(): ULong = beaconSyncDistanceValue

  override fun getCLSyncTarget(): ULong = clSyncTarget
}

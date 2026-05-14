/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.syncing

enum class CLSyncStatus {
  SYNCING,
  SYNCED, // up to head - nearHeadBlocks
  ;

  override fun toString(): String = "CLSyncStatus.${super.toString()}"
}

enum class ELSyncStatus {
  SYNCING,
  SYNCED, // EL has latest SYNCED block from Beacon
  ;

  override fun toString(): String = "ELSyncStatus.${super.toString()}"
}

interface SyncStatusProvider {
  fun getCLSyncStatus(): CLSyncStatus

  fun getElSyncStatus(): ELSyncStatus

  fun onClSyncStatusUpdate(handler: (newStatus: CLSyncStatus) -> Unit)

  fun onElSyncStatusUpdate(handler: (newStatus: ELSyncStatus) -> Unit)

  fun isBeaconChainSynced(): Boolean

  fun isELSynced(): Boolean

  fun isNodeFullInSync(): Boolean = isELSynced() && isBeaconChainSynced()

  fun onBeaconSyncComplete(handler: () -> Unit)

  fun onFullSyncComplete(handler: () -> Unit)

  fun getBeaconSyncDistance(): ULong

  fun getCLSyncTarget(): ULong
}

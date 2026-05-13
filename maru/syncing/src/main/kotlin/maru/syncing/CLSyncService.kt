/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.syncing

/**
 * Service to synchronize the beacon chain with a specified target.
 * It allows setting a sync target and provides a callback mechanism to notify when the sync is complete.
 */
interface CLSyncService {
  /**
   * Sets the target for synchronization.
   * The service will synchronize up to this target block number.
   *
   * @param syncTarget The target block number to synchronize to.
   */
  fun setSyncTarget(syncTarget: ULong)

  /**
   * Notifies the handler when the <b>latest<b/> target is reached.
   * If the target is updated, onSyncComplete won't be called for previous targets
   */
  fun onSyncComplete(handler: (syncTarget: ULong) -> Unit)

  fun getSyncTarget(): ULong

  fun getSyncDistance(): ULong
}

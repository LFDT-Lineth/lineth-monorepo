/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.database

interface P2PState : AutoCloseable {
  fun getLocalNodeRecordSequenceNumber(): ULong

  fun newP2PStateUpdater(): Updater

  interface Updater : AutoCloseable {
    fun putDiscoverySequenceNumber(newSequenceNumber: ULong): Updater

    fun commit(): Unit

    fun rollback(): Unit
  }
}

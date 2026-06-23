/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.database

class InMemoryP2PState : P2PState {
  companion object {
    const val DISCOVERY_SEQUENCE_NUMBER_KEY = "DiscoverySequenceNumber"
  }

  private val configs = mutableMapOf<String, Any>()

  override fun getLocalNodeRecordSequenceNumber(): ULong = (configs[DISCOVERY_SEQUENCE_NUMBER_KEY] ?: 0uL) as ULong

  override fun newP2PStateUpdater(): P2PState.Updater = InMemoryP2PStateUpdater(this)

  override fun close() {
    // No-op for in-memory runtime configs
  }

  class InMemoryP2PStateUpdater(
    val inMemoryP2PState: InMemoryP2PState,
  ) : P2PState.Updater {
    private val configUpdates = mutableMapOf<String, Any>()

    override fun putDiscoverySequenceNumber(newSequenceNumber: ULong): P2PState.Updater {
      configUpdates[InMemoryP2PState.DISCOVERY_SEQUENCE_NUMBER_KEY] = newSequenceNumber
      return this
    }

    override fun commit() {
      configUpdates.forEach { (key, value) ->
        inMemoryP2PState.configs[key] = value
      }
      configUpdates.clear()
    }

    override fun rollback() {
      configUpdates.clear()
    }

    override fun close() {
      // No-op for in-memory updater
    }
  }
}

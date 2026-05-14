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
 * Responsible to keep track of peer's STATUS and select the head of the chain
 */
fun interface SyncTargetSelector {
  fun selectBestSyncTarget(peerHeads: List<ULong>): ULong
}

class MostFrequentHeadTargetSelector(
  // Resolution of the peer heights
  val granularity: UInt,
) : SyncTargetSelector {
  init {
    require(granularity >= 1U) { "Granularity should be >= 1!" }
  }

  /**
   * Rounds the block height according to the configured granularity
   */
  private fun roundHeight(height: ULong): ULong {
    if (granularity <= 1u) return height
    return (height / granularity.toULong()) * granularity.toULong()
  }

  override fun selectBestSyncTarget(peerHeads: List<ULong>): ULong {
    require(peerHeads.isNotEmpty()) { "Peer heads list cannot be empty" }

    val roundedPeerHeads = peerHeads.map(::roundHeight)
    val frequencyMap = roundedPeerHeads.groupingBy { it }.eachCount()
    val maxFrequency = frequencyMap.values.max()

    // Among all heads with max frequency, return the highest value
    return frequencyMap
      .filterValues { it == maxFrequency }
      .keys
      .max()
  }
}

class HighestHeadTargetSelector : SyncTargetSelector {
  override fun selectBestSyncTarget(peerHeads: List<ULong>): ULong {
    require(peerHeads.isNotEmpty()) { "Peer heads list cannot be empty" }

    return peerHeads.max()
  }
}

fun interface BeaconSyncTargetUpdateHandler {
  fun onBeaconChainSyncTargetUpdated(syncTargetBlockNumber: ULong)
}

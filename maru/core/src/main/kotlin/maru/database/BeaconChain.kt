/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.database

import maru.core.BeaconBlock
import maru.core.BeaconState
import maru.core.SealedBeaconBlock
import kotlin.math.min

interface BeaconChain : AutoCloseable {
  fun isInitialized(): Boolean

  fun getLatestBeaconState(): BeaconState

  fun getBeaconState(beaconBlockRoot: ByteArray): BeaconState?

  fun getBeaconState(beaconBlockNumber: ULong): BeaconState?

  fun getSealedBeaconBlock(beaconBlockRoot: ByteArray): SealedBeaconBlock?

  fun getSealedBeaconBlock(beaconBlockNumber: ULong): SealedBeaconBlock?

  /**
   * Returns a list of sealed beacon blocks inclusively starting from the given block number.
   * The list will contain at most `count` blocks, or fewer if there are not enough blocks available.
   *
   * @param startBlockNumber The block number to start from (inclusive).
   * @param count The maximum number of blocks to return.
   */
  fun getSealedBeaconBlocks(
    startBlockNumber: ULong,
    count: ULong,
  ): List<SealedBeaconBlock> =
    generateSequence(startBlockNumber) { it + 1UL }
      .take(min(count, getLatestBeaconState().beaconBlockHeader.number).toInt())
      .map { blockNumber ->
        getSealedBeaconBlock(blockNumber)
          ?: throw IllegalStateException("Missing sealed beacon block $blockNumber")
      }.toList()

  fun getLatestBeaconBlock(): BeaconBlock =
    getSealedBeaconBlock(getLatestBeaconState().beaconBlockHeader.number)
      ?.beaconBlock
      ?: throw IllegalStateException("Missing latest sealed beacon block")

  fun newBeaconChainUpdater(): Updater

  interface Updater : AutoCloseable {
    fun putBeaconState(beaconState: BeaconState): Updater

    fun putSealedBeaconBlock(sealedBeaconBlock: SealedBeaconBlock): Updater

    fun commit(): Unit

    fun rollback(): Unit
  }
}

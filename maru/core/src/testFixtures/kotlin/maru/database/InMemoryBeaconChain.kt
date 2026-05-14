/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.database

import maru.core.BeaconState
import maru.core.SealedBeaconBlock
import maru.core.ext.DataGenerators

// Wrapper class for ByteArray to use as a key in maps. This is necessary because ByteArray does not implement
// equals/hashCode correctly for content comparison
private class ByteArrayWrapper(
  val bytes: ByteArray,
) {
  override fun equals(other: Any?): Boolean = other is ByteArrayWrapper && bytes.contentEquals(other.bytes)

  override fun hashCode(): Int = bytes.contentHashCode()
}

class InMemoryBeaconChain(
  initialBeaconState: BeaconState,
  initialBeaconBlock: SealedBeaconBlock? = null,
) : BeaconChain {
  private val beaconStateByBlockRoot = mutableMapOf<ByteArrayWrapper, BeaconState>()
  private val beaconStateByBlockNumber = mutableMapOf<ULong, BeaconState>()
  private val sealedBeaconBlockByBlockRoot = mutableMapOf<ByteArrayWrapper, SealedBeaconBlock>()
  private val sealedBeaconBlockByBlockNumber = mutableMapOf<ULong, SealedBeaconBlock>()
  private var latestBeaconState: BeaconState = initialBeaconState

  companion object {
    fun fromGenesis(genesisTimestampSeconds: ULong = 0UL): InMemoryBeaconChain {
      val (beaconState, sealedBlock) = DataGenerators.genesisState(genesisTimestampSeconds)
      return InMemoryBeaconChain(beaconState, sealedBlock)
    }
  }

  init {
    newBeaconChainUpdater().run {
      putBeaconState(initialBeaconState)
      initialBeaconBlock?.let { putSealedBeaconBlock(it) }
      commit()
    }
  }

  override fun isInitialized(): Boolean = true

  override fun getLatestBeaconState(): BeaconState = latestBeaconState

  override fun getBeaconState(beaconBlockRoot: ByteArray): BeaconState? =
    beaconStateByBlockRoot[ByteArrayWrapper(beaconBlockRoot)]

  override fun getBeaconState(beaconBlockNumber: ULong): BeaconState? = beaconStateByBlockNumber[beaconBlockNumber]

  override fun getSealedBeaconBlock(beaconBlockRoot: ByteArray): SealedBeaconBlock? =
    sealedBeaconBlockByBlockRoot[ByteArrayWrapper(beaconBlockRoot)]

  override fun getSealedBeaconBlock(beaconBlockNumber: ULong): SealedBeaconBlock? =
    sealedBeaconBlockByBlockNumber[beaconBlockNumber]

  override fun newBeaconChainUpdater(): BeaconChain.Updater = InMemoryUpdater(this)

  override fun close() {
    // No-op for in-memory beacon chain
  }

  private class InMemoryUpdater(
    private val beaconChain: InMemoryBeaconChain,
  ) : BeaconChain.Updater {
    private val beaconStateByBlockRoot = mutableMapOf<ByteArrayWrapper, BeaconState>()
    private val beaconStateByBlockNumber = mutableMapOf<ULong, BeaconState>()
    private val sealedBeaconBlockByBlockRoot = mutableMapOf<ByteArrayWrapper, SealedBeaconBlock>()
    private val sealedBeaconBlockByBlockNumber = mutableMapOf<ULong, SealedBeaconBlock>()

    private var newBeaconState: BeaconState? = null

    override fun putBeaconState(beaconState: BeaconState): BeaconChain.Updater {
      beaconStateByBlockRoot[ByteArrayWrapper(beaconState.beaconBlockHeader.hash)] = beaconState
      beaconStateByBlockNumber[beaconState.beaconBlockHeader.number] = beaconState
      newBeaconState = beaconState
      return this
    }

    override fun putSealedBeaconBlock(sealedBeaconBlock: SealedBeaconBlock): BeaconChain.Updater {
      sealedBeaconBlockByBlockRoot[ByteArrayWrapper(sealedBeaconBlock.beaconBlock.beaconBlockHeader.hash)] =
        sealedBeaconBlock
      sealedBeaconBlockByBlockNumber[sealedBeaconBlock.beaconBlock.beaconBlockHeader.number] = sealedBeaconBlock
      return this
    }

    override fun commit() {
      beaconChain.beaconStateByBlockRoot.putAll(beaconStateByBlockRoot)
      beaconChain.beaconStateByBlockNumber.putAll(beaconStateByBlockNumber)
      beaconChain.sealedBeaconBlockByBlockRoot.putAll(sealedBeaconBlockByBlockRoot)
      beaconChain.sealedBeaconBlockByBlockNumber.putAll(sealedBeaconBlockByBlockNumber)
      if (newBeaconState != null) {
        beaconChain.latestBeaconState = newBeaconState!!
      }
    }

    override fun rollback() {
      beaconStateByBlockRoot.clear()
      beaconStateByBlockNumber.clear()
      sealedBeaconBlockByBlockRoot.clear()
      sealedBeaconBlockByBlockNumber.clear()
      newBeaconState = null
    }

    override fun close() {
      // No-op for in-memory updater
    }
  }
}

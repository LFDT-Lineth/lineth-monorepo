/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.core

import maru.core.ext.DataGenerators
import maru.database.InMemoryBeaconChain
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import kotlin.random.Random

class InMemoryBeaconChainTest {
  private lateinit var initialBeaconState: BeaconState
  private lateinit var inMemoryBeaconChain: InMemoryBeaconChain

  @BeforeEach
  fun setUp() {
    initialBeaconState = DataGenerators.randomBeaconState(2UL)
    inMemoryBeaconChain = InMemoryBeaconChain(initialBeaconState)
  }

  @Test
  fun `getLatestBeaconState returns initial state`() {
    val latestBeaconState = inMemoryBeaconChain.getLatestBeaconState()
    assertThat(latestBeaconState).isEqualTo(initialBeaconState)
  }

  @Test
  fun `getBeaconState returns null for unknown block root`() {
    val beaconState = inMemoryBeaconChain.getBeaconState(Random.nextBytes(32))
    assertThat(beaconState).isNull()
  }

  @Test
  fun `getSealedBeaconBlock returns null for unknown block root`() {
    val sealedBeaconBlock = inMemoryBeaconChain.getSealedBeaconBlock(Random.nextBytes(32))
    assertThat(sealedBeaconBlock).isNull()
  }

  @Test
  fun `getSealedBeaconBlock returns null for unknown block number`() {
    val sealedBeaconBlock = inMemoryBeaconChain.getSealedBeaconBlock(100uL)
    assertThat(sealedBeaconBlock).isNull()
  }

  @Test
  fun `newBeaconChainUpdater can put and commit beacon state`() {
    val newBeaconState = DataGenerators.randomBeaconState(2UL)
    val updater = inMemoryBeaconChain.newBeaconChainUpdater()
    updater.putBeaconState(newBeaconState).commit()

    val latestBeaconState = inMemoryBeaconChain.getLatestBeaconState()
    assertThat(latestBeaconState).isEqualTo(newBeaconState)

    val retrievedBeaconState = inMemoryBeaconChain.getBeaconState(newBeaconState.beaconBlockHeader.hash)
    assertThat(retrievedBeaconState).isEqualTo(newBeaconState)
  }

  @Test
  fun `newBeaconChainUpdater can put and commit sealed beacon block`() {
    val sealedBeaconBlock = DataGenerators.randomSealedBeaconBlock(3UL)
    val beaconBlockRoot = sealedBeaconBlock.beaconBlock.beaconBlockHeader.hash
    val updater = inMemoryBeaconChain.newBeaconChainUpdater()
    updater.putSealedBeaconBlock(sealedBeaconBlock).commit()

    val retrievedSealedBeaconBlockByBlockRoot = inMemoryBeaconChain.getSealedBeaconBlock(beaconBlockRoot)
    assertThat(retrievedSealedBeaconBlockByBlockRoot).isEqualTo(sealedBeaconBlock)

    val retrievedSealedBeaconBlockByBlockNumber =
      inMemoryBeaconChain
        .getSealedBeaconBlock(sealedBeaconBlock.beaconBlock.beaconBlockHeader.number)
    assertThat(retrievedSealedBeaconBlockByBlockNumber).isEqualTo(sealedBeaconBlock)
  }

  @Test
  fun `newBeaconChainUpdater can rollback changes`() {
    val newBeaconState = DataGenerators.randomBeaconState(4UL)
    val sealedBeaconBlock = DataGenerators.randomSealedBeaconBlock(5UL)
    val beaconBlockRoot = sealedBeaconBlock.beaconBlock.beaconBlockHeader.hash
    val updater = inMemoryBeaconChain.newBeaconChainUpdater()
    updater.putBeaconState(newBeaconState)
    updater.putSealedBeaconBlock(sealedBeaconBlock)
    updater.rollback()

    val latestBeaconState = inMemoryBeaconChain.getLatestBeaconState()
    assertThat(latestBeaconState).isEqualTo(initialBeaconState)

    val retrievedBeaconState = inMemoryBeaconChain.getBeaconState(newBeaconState.beaconBlockHeader.hash)
    assertThat(retrievedBeaconState).isNull()

    val retrievedSealedBeaconBlockByBlockRoot = inMemoryBeaconChain.getSealedBeaconBlock(beaconBlockRoot)
    assertThat(retrievedSealedBeaconBlockByBlockRoot).isNull()

    val retrievedSealedBeaconBlockByBlockNumber =
      inMemoryBeaconChain
        .getSealedBeaconBlock(sealedBeaconBlock.beaconBlock.beaconBlockHeader.number)
    assertThat(retrievedSealedBeaconBlockByBlockNumber).isNull()
  }

  @Test
  fun `uncommited changes are not visible by InMemoryBeaconChain`() {
    val newBeaconState = DataGenerators.randomBeaconState(6UL)
    val newBeaconBlock = DataGenerators.randomSealedBeaconBlock(7UL)
    val inflightBeaconBlockRoot = newBeaconBlock.beaconBlock.beaconBlockHeader.hash
    val updater = inMemoryBeaconChain.newBeaconChainUpdater()
    updater.putBeaconState(newBeaconState)
    updater.putSealedBeaconBlock(newBeaconBlock)

    val latestBeaconState = inMemoryBeaconChain.getLatestBeaconState()
    assertThat(latestBeaconState).isEqualTo(initialBeaconState)

    val retrievedBeaconState = inMemoryBeaconChain.getBeaconState(newBeaconState.beaconBlockHeader.hash)
    assertThat(retrievedBeaconState).isNull()

    val retrievedSealedBeaconBlockByBlockRoot = inMemoryBeaconChain.getSealedBeaconBlock(inflightBeaconBlockRoot)
    assertThat(retrievedSealedBeaconBlockByBlockRoot).isNull()

    val retrievedSealedBeaconBlockByBlockNumber =
      inMemoryBeaconChain
        .getSealedBeaconBlock(newBeaconBlock.beaconBlock.beaconBlockHeader.number)
    assertThat(retrievedSealedBeaconBlockByBlockNumber).isNull()
  }

  @Test
  fun `initial state can be found by hash`() {
    val initialBeaconStateByHash = inMemoryBeaconChain.getBeaconState(initialBeaconState.beaconBlockHeader.hash)
    assertThat(initialBeaconStateByHash).isEqualTo(initialBeaconState)
  }

  @Test
  fun `initial state can be found by number`() {
    val initialBeaconStateByNumber =
      inMemoryBeaconChain.getBeaconState(initialBeaconState.beaconBlockHeader.number)
    assertThat(initialBeaconStateByNumber).isEqualTo(initialBeaconState)
  }

  @Test
  fun `getSealedBeaconBlocks returns consecutive blocks`() {
    val testBlocks = (0uL..5uL).map { DataGenerators.randomSealedBeaconBlock(it) }

    val updater = inMemoryBeaconChain.newBeaconChainUpdater()
    testBlocks.forEach { block ->
      updater.putSealedBeaconBlock(block)
    }
    updater.putBeaconState(
      BeaconState(
        beaconBlockHeader = testBlocks.last().beaconBlock.beaconBlockHeader,
        validators = DataGenerators.randomValidators(),
      ),
    )
    updater.commit()

    val startBlockNumber = 2uL
    val count = 3uL
    val blocks = inMemoryBeaconChain.getSealedBeaconBlocks(startBlockNumber, count)
    assertThat(blocks).hasSize(3)
    assertThat(blocks).isEqualTo(testBlocks.subList(startBlockNumber.toInt(), (startBlockNumber + count).toInt()))
  }

  @Test
  fun `getSealedBeaconBlocks returns empty list when count is zero`() {
    val testBlock = DataGenerators.randomSealedBeaconBlock(1uL)

    val updater = inMemoryBeaconChain.newBeaconChainUpdater()
    updater.putSealedBeaconBlock(testBlock).commit()

    val blocks = inMemoryBeaconChain.getSealedBeaconBlocks(startBlockNumber = 1uL, count = 0uL)
    assertThat(blocks).isEmpty()
  }

  @Test
  fun `getSealedBeaconBlocks stops at gap in sequence`() {
    val block1 = DataGenerators.randomSealedBeaconBlock(1uL)
    val block2 = DataGenerators.randomSealedBeaconBlock(2uL)
    val block4 = DataGenerators.randomSealedBeaconBlock(4uL)

    val updater = inMemoryBeaconChain.newBeaconChainUpdater()
    updater
      .putSealedBeaconBlock(block1)
      .putSealedBeaconBlock(block2)
      .putSealedBeaconBlock(block4)
      .putBeaconState(
        BeaconState(
          beaconBlockHeader = block4.beaconBlock.beaconBlockHeader,
          validators = DataGenerators.randomValidators(),
        ),
      ).commit()

    assertThatThrownBy {
      inMemoryBeaconChain.getSealedBeaconBlocks(startBlockNumber = 1uL, count = 5uL)
    }.isInstanceOf(IllegalStateException::class.java)
      .hasMessage("Missing sealed beacon block 3")
  }

  @Test
  fun `getSealedBeaconBlocks returns available blocks when count exceeds available`() {
    val testBlocks = (1uL..3uL).map { DataGenerators.randomSealedBeaconBlock(it) }

    val updater = inMemoryBeaconChain.newBeaconChainUpdater()
    testBlocks.forEach { block ->
      updater.putSealedBeaconBlock(block)
    }
    updater.putBeaconState(
      BeaconState(
        beaconBlockHeader = testBlocks.last().beaconBlock.beaconBlockHeader,
        validators = DataGenerators.randomValidators(),
      ),
    )
    updater.commit()

    val blocks = inMemoryBeaconChain.getSealedBeaconBlocks(startBlockNumber = 1uL, count = 10uL)
    assertThat(blocks).hasSize(3)
    assertThat(blocks).isEqualTo(testBlocks)
  }

  @Test
  fun `getBeaconState finds state by block root with different byte arrays`() {
    val newBeaconState = DataGenerators.randomBeaconState(3UL)
    val updater = inMemoryBeaconChain.newBeaconChainUpdater()
    updater.putBeaconState(newBeaconState).commit()
    val blockRootCopy = newBeaconState.beaconBlockHeader.hash.copyOf() // new instance, same content
    val found = inMemoryBeaconChain.getBeaconState(blockRootCopy)
    assertThat(found).isEqualTo(newBeaconState)
  }

  @Test
  fun `getSealedBeaconBlock finds block by block root with different byte arrays`() {
    val newBlock = DataGenerators.randomSealedBeaconBlock(4UL)
    val updater = inMemoryBeaconChain.newBeaconChainUpdater()
    updater.putSealedBeaconBlock(newBlock).commit()
    val blockRootCopy =
      newBlock.beaconBlock.beaconBlockHeader.hash
        .copyOf() // new instance, same content
    val found = inMemoryBeaconChain.getSealedBeaconBlock(blockRootCopy)
    assertThat(found).isEqualTo(newBlock)
  }
}

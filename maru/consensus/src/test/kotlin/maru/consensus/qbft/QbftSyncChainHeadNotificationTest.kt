/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import maru.consensus.qbft.adapters.QbftBlockchainAdapter
import maru.core.SealedBeaconBlock
import maru.core.ext.DataGenerators
import maru.database.InMemoryBeaconChain
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.consensus.common.bft.BftEventQueue
import org.hyperledger.besu.consensus.qbft.core.types.QbftNewChainHead
import org.junit.jupiter.api.Test
import java.util.concurrent.TimeUnit

/**
 * Verifies that the sync-completion → QbftNewChainHead wiring works with real objects.
 *
 * When the CL sync pipeline imports blocks from other validators, a QbftNewChainHead event
 * must be enqueued so QBFT advances past the externally-imported blocks.
 * This test replicates the exact wiring that [QbftValidatorFactory.create] sets up
 * when a [maru.syncing.SyncStatusProvider] is supplied.
 */
class QbftSyncChainHeadNotificationTest {
  @Test
  fun `sync completion enqueues QbftNewChainHead with current chain head`() {
    val beaconChain = inMemoryBeaconChainFromGenesis()
    val blockChain = QbftBlockchainAdapter(beaconChain)
    val bftEventQueue = BftEventQueue(100).also { it.start() }

    // This is the exact callback that QbftValidatorFactory registers on syncStatusProvider
    val onSyncComplete: () -> Unit = {
      bftEventQueue.add(QbftNewChainHead(blockChain.chainHeadHeader))
    }

    // Simulate the sync pipeline advancing the chain to block 5
    var currentState = beaconChain.getLatestBeaconState()
    for (blockNumber in 1UL..5UL) {
      val beaconBlock = DataGenerators.randomBeaconBlock(
        blockNumber,
      )
      val sealedBlock = SealedBeaconBlock(beaconBlock, emptySet())
      currentState = currentState.copy(beaconBlockHeader = beaconBlock.beaconBlockHeader)
      beaconChain.newBeaconChainUpdater().run {
        putBeaconState(currentState)
        putSealedBeaconBlock(sealedBlock)
        commit()
      }
    }

    // Fire the sync-complete callback (as the sync controller would)
    onSyncComplete()

    val event = bftEventQueue.poll(1, TimeUnit.SECONDS)
    assertThat(event).isInstanceOf(QbftNewChainHead::class.java)
    assertThat((event as QbftNewChainHead).newChainHeadHeader().number).isEqualTo(5L)
  }

  private fun inMemoryBeaconChainFromGenesis(): InMemoryBeaconChain {
    val (beaconState, sealedBlock) = DataGenerators.genesisState()
    return InMemoryBeaconChain(beaconState, sealedBlock)
  }
}

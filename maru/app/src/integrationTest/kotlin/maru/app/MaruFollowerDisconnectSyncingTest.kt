/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import maru.config.SyncingConfig
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.tests.acceptance.dsl.blockchain.Amount
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.MethodSource
import testutils.Checks.getBlockNumber
import testutils.maru.MaruFactory
import testutils.maru.awaitTillMaruHasPeers
import kotlin.time.Duration.Companion.seconds

/**
 * Follower syncing-after-peer-disconnect scenario. Kept in its own class (see [MaruFollowerTestBase])
 * so it runs in a parallel Gradle fork rather than serially with the other syncing scenarios.
 */
class MaruFollowerDisconnectSyncingTest : MaruFollowerTestBase() {
  companion object {
    @JvmStatic
    fun enumeratingSyncingConfigs(): List<SyncingConfig> = MaruFactory.enumeratingSyncingConfigs()
  }

  @ParameterizedTest
  @MethodSource("enumeratingSyncingConfigs")
  fun `Maru follower is able to complete syncing after disconnect peers`(syncingConfig: SyncingConfig) {
    setupMaruHelper(syncingConfig)

    val residueBlocks = 3 // residue of modulo peerChainHeightGranularity i.e. 10
    val blocksToProduce = 20 // a block number dividable by 10
    repeat(blocksToProduce) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    // This is here mainly to wait until block propagation is complete
    checkValidatorAndFollowerBlocks(blocksToProduce)

    val followerP2PNetwork = followerStack.maruApp.p2pNetwork
    val peers = followerP2PNetwork.getPeers()
    peers.forEach {
      followerP2PNetwork.dropPeer(it)
    }

    repeat(blocksToProduce + residueBlocks) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }
    checkNetworkStacksBlocksProduced(2 * blocksToProduce + residueBlocks, validatorStack)
    checkNetworkStacksBlocksProduced(blocksToProduce, followerStack)
    // ensure that the head of follower is at blocksToProduce
    assertThat(followerStack.besuNode.getBlockNumber()).isEqualTo(blocksToProduce)

    peers.forEach {
      followerP2PNetwork.addPeer("${it.address}/p2p/${it.nodeId}")
    }
    followerStack.maruApp.awaitTillMaruHasPeers(1u, pollingInterval = 1.seconds)
    when (syncingConfig.syncTargetSelection) {
      is SyncingConfig.SyncTargetSelection.Highest ->
        checkValidatorAndFollowerBlocks(
          blocksToProduce = 2 * blocksToProduce + residueBlocks,
          timeout = 60.seconds,
        )

      is SyncingConfig.SyncTargetSelection.MostFrequent -> {
        checkValidatorAndFollowerBlocks(2 * blocksToProduce)
        // ensure that the head of follower is at 2 * blocksToProduce
        assertThat(followerStack.besuNode.getBlockNumber()).isEqualTo(2 * blocksToProduce)
      }
    }
  }
}

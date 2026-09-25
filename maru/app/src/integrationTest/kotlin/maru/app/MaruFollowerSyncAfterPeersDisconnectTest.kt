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
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.MethodSource
import testutils.Checks.getBlockNumber
import testutils.ValidatorFollowerNetwork
import testutils.maru.MaruFactory
import testutils.maru.awaitTillMaruHasPeers
import kotlin.time.Duration.Companion.seconds

class MaruFollowerSyncAfterPeersDisconnectTest {
  companion object {
    @JvmStatic
    fun enumeratingSyncingConfigs(): List<SyncingConfig> = MaruFactory.enumeratingSyncingConfigs()
  }

  private lateinit var network: ValidatorFollowerNetwork

  @BeforeEach
  fun setUp() {
    network = ValidatorFollowerNetwork()
    network.startBesuNodes()
  }

  @AfterEach
  fun tearDown() {
    network.close()
  }

  @ParameterizedTest
  @MethodSource("enumeratingSyncingConfigs")
  fun `Maru follower is able to complete syncing after disconnect peers`(syncingConfig: SyncingConfig) {
    network.startMaruNodes(syncingConfig)

    val residueBlocks = 3 // residue of modulo peerChainHeightGranularity i.e. 10
    val blocksToProduce = 20 // a block number dividable by 10
    network.produceBlocks(blocksToProduce)

    // This is here mainly to wait until block propagation is complete
    network.checkValidatorAndFollowerBlocks(blocksToProduce)

    val followerP2PNetwork = network.followerStack.maruApp.p2pNetwork
    val peers = followerP2PNetwork.getPeers()
    peers.forEach {
      followerP2PNetwork.dropPeer(it)
    }

    network.produceBlocks(blocksToProduce + residueBlocks)
    network.checkNetworkStacksBlocksProduced(2 * blocksToProduce + residueBlocks, network.validatorStack)
    network.checkNetworkStacksBlocksProduced(blocksToProduce, network.followerStack)
    // ensure that the head of follower is at blocksToProduce
    assertThat(network.followerStack.besuNode.getBlockNumber()).isEqualTo(blocksToProduce)

    peers.forEach {
      followerP2PNetwork.addPeer("${it.address}/p2p/${it.nodeId}")
    }
    network.followerStack.maruApp.awaitTillMaruHasPeers(1u, pollingInterval = 1.seconds)
    when (syncingConfig.syncTargetSelection) {
      is SyncingConfig.SyncTargetSelection.Highest ->
        network.checkValidatorAndFollowerBlocks(
          blocksToProduce = 2 * blocksToProduce + residueBlocks,
          timeout = 60.seconds,
        )

      is SyncingConfig.SyncTargetSelection.MostFrequent -> {
        network.checkValidatorAndFollowerBlocks(2 * blocksToProduce)
        // ensure that the head of follower is at 2 * blocksToProduce
        assertThat(network.followerStack.besuNode.getBlockNumber()).isEqualTo(2 * blocksToProduce)
      }
    }
  }
}

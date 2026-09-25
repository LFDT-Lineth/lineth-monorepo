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

class MaruFollowerSyncAfterRestartTest {
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
  fun `Maru follower is able to complete syncing after restarted`(syncingConfig: SyncingConfig) {
    network.startMaruNodes(syncingConfig)

    val residueBlocks = 3 // residue of modulo peerChainHeightGranularity i.e. 10
    val blocksToProduceWithoutResidue = 10 // a block number dividable by 10

    network.produceBlocks(blocksToProduceWithoutResidue)

    // This is here mainly to wait until block propagation is complete
    network.checkValidatorAndFollowerBlocks(blocksToProduceWithoutResidue)

    network.stopFollowerMaru()

    network.produceBlocks(blocksToProduceWithoutResidue + residueBlocks)
    network.checkNetworkStacksBlocksProduced(2 * blocksToProduceWithoutResidue + residueBlocks, network.validatorStack)

    network.startFollowerMaru(syncingConfig)
    network.followerStack.maruApp.awaitTillMaruHasPeers(1u, pollingInterval = 1.seconds)

    when (syncingConfig.syncTargetSelection) {
      is SyncingConfig.SyncTargetSelection.Highest ->
        network.checkValidatorAndFollowerBlocks(
          2 * blocksToProduceWithoutResidue + residueBlocks,
        )

      is SyncingConfig.SyncTargetSelection.MostFrequent -> {
        network.checkValidatorAndFollowerBlocks(2 * blocksToProduceWithoutResidue)
        // ensure that the head of follower is 2 * blocksToProduceWithoutResidue
        assertThat(network.followerStack.besuNode.getBlockNumber()).isEqualTo(2 * blocksToProduceWithoutResidue)
      }
    }
  }
}

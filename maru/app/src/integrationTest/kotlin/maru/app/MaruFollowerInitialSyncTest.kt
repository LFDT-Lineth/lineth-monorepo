/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import maru.config.SyncingConfig
import maru.test.extensions.TestSyncingConfigs
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.MethodSource
import testutils.Checks.checkAllNodesHaveSameBlocks
import testutils.Checks.getBlockNumber
import testutils.ValidatorFollowerNetwork
import testutils.maru.awaitTillMaruHasPeers
import kotlin.time.Duration.Companion.seconds

class MaruFollowerInitialSyncTest {
  companion object {
    @JvmStatic
    fun enumeratingSyncingConfigs(): List<SyncingConfig> = TestSyncingConfigs.enumerating()
  }

  private lateinit var network: ValidatorFollowerNetwork

  @BeforeEach
  fun setUp() {
    network = ValidatorFollowerNetwork()
  }

  @AfterEach
  fun tearDown() {
    network.close()
  }

  @ParameterizedTest
  @MethodSource("enumeratingSyncingConfigs")
  fun `Maru follower is able to complete initial syncing`(syncingConfig: SyncingConfig) {
    network.start(syncingConfig)

    network.stopFollowerMaru()

    val residueBlocks = 3 // residue of modulo peerChainHeightGranularity i.e. 10
    val blocksToProduceWithoutResidue = 20 // a block number dividable by 10
    val blocksTotal = residueBlocks + blocksToProduceWithoutResidue

    network.produceBlocks(blocksTotal)

    // This is here mainly to wait until block propagation is complete
    checkAllNodesHaveSameBlocks(blocksTotal, network.validatorBesu)

    network.startFollowerMaru()
    network.followerMaru.awaitTillMaruHasPeers(1u, pollingInterval = 1.seconds)

    when (syncingConfig.syncTargetSelection) {
      is SyncingConfig.SyncTargetSelection.Highest ->
        network.checkValidatorAndFollowerBlocks(blocksTotal)

      is SyncingConfig.SyncTargetSelection.MostFrequent -> {
        network.checkValidatorAndFollowerBlocks(blocksToProduceWithoutResidue)
        // ensure that the head of follower is blocksToProduceWithoutResidue
        assertThat(network.followerBesu.getBlockNumber()).isEqualTo(blocksToProduceWithoutResidue)
      }
    }
  }
}

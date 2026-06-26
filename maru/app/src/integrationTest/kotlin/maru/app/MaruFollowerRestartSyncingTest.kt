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
 * Follower syncing-after-restart scenario. Kept in its own class (see [MaruFollowerTestBase]) so it
 * runs in a parallel Gradle fork rather than serially with the other syncing scenarios.
 */
class MaruFollowerRestartSyncingTest : MaruFollowerTestBase() {
  companion object {
    @JvmStatic
    fun enumeratingSyncingConfigs(): List<SyncingConfig> = MaruFactory.enumeratingSyncingConfigs()
  }

  @ParameterizedTest
  @MethodSource("enumeratingSyncingConfigs")
  fun `Maru follower is able to complete syncing after restarted`(syncingConfig: SyncingConfig) {
    setupMaruHelper(syncingConfig)

    val residueBlocks = 3 // residue of modulo peerChainHeightGranularity i.e. 10
    val blocksToProduceWithoutResidue = 10 // a block number dividable by 10

    repeat(blocksToProduceWithoutResidue) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    // This is here mainly to wait until block propagation is complete
    checkValidatorAndFollowerBlocks(blocksToProduceWithoutResidue)

    followerStack.maruApp.stop().get()
    followerStack.maruApp.close()

    repeat(blocksToProduceWithoutResidue + residueBlocks) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }
    checkNetworkStacksBlocksProduced(2 * blocksToProduceWithoutResidue + residueBlocks, validatorStack)

    followerStack.setMaruApp(
      maruFactory.buildTestMaruFollowerWithP2pPeering(
        ethereumJsonRpcUrl = followerStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = followerStack.besuNode.engineRpcUrl().get(),
        dataDir = followerStack.tmpDir,
        validatorPortForStaticPeering = validatorStack.p2pPort,
        syncingConfig = syncingConfig,
      ),
    )
    followerStack.maruApp.start().get()
    followerStack.maruApp.awaitTillMaruHasPeers(1u, pollingInterval = 1.seconds)

    when (syncingConfig.syncTargetSelection) {
      is SyncingConfig.SyncTargetSelection.Highest ->
        checkValidatorAndFollowerBlocks(
          2 * blocksToProduceWithoutResidue + residueBlocks,
        )

      is SyncingConfig.SyncTargetSelection.MostFrequent -> {
        checkValidatorAndFollowerBlocks(2 * blocksToProduceWithoutResidue)
        // ensure that the head of follower is 2 * blocksToProduceWithoutResidue
        assertThat(followerStack.besuNode.getBlockNumber()).isEqualTo(2 * blocksToProduceWithoutResidue)
      }
    }
  }
}

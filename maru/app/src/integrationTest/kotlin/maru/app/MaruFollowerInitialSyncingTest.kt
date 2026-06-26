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
 * Follower initial-syncing scenario. Kept in its own class (see [MaruFollowerTestBase]) so it runs in
 * a parallel Gradle fork rather than serially with the other syncing scenarios.
 */
class MaruFollowerInitialSyncingTest : MaruFollowerTestBase() {
  companion object {
    @JvmStatic
    fun enumeratingSyncingConfigs(): List<SyncingConfig> = MaruFactory.enumeratingSyncingConfigs()
  }

  @ParameterizedTest
  @MethodSource("enumeratingSyncingConfigs")
  fun `Maru follower is able to complete initial syncing`(syncingConfig: SyncingConfig) {
    setupMaruHelper(syncingConfig)

    followerStack.maruApp.stop().get()
    followerStack.maruApp.close()

    val residueBlocks = 3 // residue of modulo peerChainHeightGranularity i.e. 10
    val blocksToProduceWithoutResidue = 20 // a block number dividable by 10
    val blocksTotal = residueBlocks + blocksToProduceWithoutResidue

    repeat(blocksTotal) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    // This is here mainly to wait until block propagation is complete
    checkNetworkStacksBlocksProduced(blocksTotal, validatorStack)

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
        checkValidatorAndFollowerBlocks(blocksTotal)

      is SyncingConfig.SyncTargetSelection.MostFrequent -> {
        checkValidatorAndFollowerBlocks(blocksToProduceWithoutResidue)
        // ensure that the head of follower is blocksToProduceWithoutResidue
        assertThat(followerStack.besuNode.getBlockNumber()).isEqualTo(blocksToProduceWithoutResidue)
      }
    }
  }
}

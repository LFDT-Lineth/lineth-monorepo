/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import maru.config.QbftConfig
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.tests.acceptance.dsl.blockchain.Amount
import org.hyperledger.besu.tests.acceptance.dsl.condition.net.NetConditions
import org.hyperledger.besu.tests.acceptance.dsl.node.ThreadBesuNodeRunner
import org.hyperledger.besu.tests.acceptance.dsl.node.cluster.Cluster
import org.hyperledger.besu.tests.acceptance.dsl.node.cluster.ClusterConfigurationBuilder
import org.hyperledger.besu.tests.acceptance.dsl.transaction.net.NetTransactions
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import testutils.Checks.getMinedBlocks
import testutils.RecordingEngineProxy
import testutils.SingleNodeNetworkStack
import testutils.besu.BesuTransactionsHelper
import testutils.maru.MaruFactory
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

class MaruLongRunningTransactionTest {
  private lateinit var cluster: Cluster
  private lateinit var networkParticipantStack: SingleNodeNetworkStack
  private lateinit var transactionsHelper: BesuTransactionsHelper
  private val log = LogManager.getLogger(this.javaClass)
  private val maruFactory = MaruFactory()
  private lateinit var proxy: RecordingEngineProxy

  private val expectedMinBuildTime = 1700L

  @BeforeEach
  fun setUp() {
    transactionsHelper = BesuTransactionsHelper()
    cluster = Cluster(
      ClusterConfigurationBuilder().build(),
      NetConditions(NetTransactions()),
      ThreadBesuNodeRunner(),
    )

    networkParticipantStack = SingleNodeNetworkStack(
      cluster = cluster,
    ) { ethereumJsonRpcBaseUrl, engineRpcUrl, tmpDir ->
      proxy = RecordingEngineProxy(engineRpcUrl)
      proxy.start()

      maruFactory.buildTestMaruValidatorWithoutP2pPeering(
        ethereumJsonRpcUrl = ethereumJsonRpcBaseUrl,
        engineApiRpc = proxy.url(),
        dataDir = tmpDir,
        allowEmptyBlocks = false,
        syncingConfig = MaruFactory.defaultSyncingConfig.copy(
          elSyncStatusRefreshInterval = 1000.seconds,
        ),
        qbftOptions = QbftConfig(
          feeRecipient = maruFactory.qbftValidator.address.reversedArray(),
          minBlockBuildTime = expectedMinBuildTime.milliseconds,
          roundExpiry = 2.seconds,
        ),
      )
    }
    networkParticipantStack.maruApp.start().get()
  }

  @AfterEach
  fun tearDown() {
    networkParticipantStack.maruApp.stop().get()
    networkParticipantStack.maruApp.close()
    proxy.stop()
    cluster.close()
  }

  @Test
  fun `Maru creates a block in the next round when empty blocks are rejected`() {
    mineBlockWithTransaction(expectedBlockNumber = 1)

    // In Round 0, Maru creates an empty block (no pending transactions) which gets rejected.
    // With the pre-build design, the round-1 proposer pre-builds during round 0 expiry (~roundExpiry
    // time), giving transactions time to arrive. The transaction sent here is included in round 1
    // (or a later round), verifying that allowEmptyBlocks=false is respected.
    mineBlockWithTransaction(expectedBlockNumber = 2)
  }

  private fun mineBlockWithTransaction(expectedBlockNumber: Int) {
    transactionsHelper.run {
      networkParticipantStack.besuNode.sendTransactionAndAssertExecution(
        logger = log,
        recipient = createAccount("another account"),
        amount = Amount.ether(100),
      )
    }
    assertThat(networkParticipantStack.besuNode.getMinedBlocks(expectedBlockNumber)).hasSize(expectedBlockNumber)
  }
}

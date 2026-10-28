/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import linea.testing.besu.BesuTransactionsHelper
import maru.test.cluster.MaruCluster
import maru.test.cluster.NodeRole
import maru.test.cluster.configureLoggers
import maru.test.extensions.TestSyncingConfigs
import maru.test.extensions.withFastQbft
import maru.test.extensions.withSyncingConfig
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.tests.acceptance.dsl.blockchain.Amount
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import testutils.Checks.getMinedBlocks
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

class MaruLongRunningTransactionTest {
  private lateinit var cluster: MaruCluster
  private lateinit var transactionsHelper: BesuTransactionsHelper
  private val log = LogManager.getLogger(this.javaClass)

  private val expectedMinBuildTime = 1700L

  @BeforeEach
  fun setUp() {
    configureLoggers(
      rootLevel = Level.WARN,
      logLevels = listOf("maru" to Level.INFO),
    )
    transactionsHelper = BesuTransactionsHelper()

    // Single validator without P2P peering
    cluster = MaruCluster()
      .addNode(NodeRole.Sequencer, withBesuEl = true) { nodeBuilder ->
        nodeBuilder
          .withSyncingConfig(TestSyncingConfigs.default.copy(elSyncStatusRefreshInterval = 1000.seconds))
          .withFastQbft(minBlockBuildTime = expectedMinBuildTime.milliseconds, roundExpiry = 2.seconds)
          .maruConfig { it.copy(allowEmptyBlocks = false, p2p = null) }
      }.start()
  }

  @AfterEach
  fun tearDown() {
    if (::cluster.isInitialized) {
      cluster.stop()
    }
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
      cluster.besuNode("sequencer").sendTransactionAndAssertExecution(
        logger = log,
        recipient = createAccount("another account"),
        amount = Amount.ether(100),
      )
    }
    assertThat(cluster.besuNode("sequencer").getMinedBlocks(expectedBlockNumber)).hasSize(expectedBlockNumber)
  }
}

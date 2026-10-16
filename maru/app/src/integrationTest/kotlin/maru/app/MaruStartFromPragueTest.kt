/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import linea.testing.besu.BesuTransactionsHelper
import linea.testing.besu.latestBlock
import maru.test.cluster.MaruCluster
import maru.test.cluster.NodeRole
import maru.test.cluster.configureLoggers
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.kotlin.await
import org.hyperledger.besu.tests.acceptance.dsl.blockchain.Amount
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

class MaruStartFromPragueTest {
  private lateinit var cluster: MaruCluster
  private lateinit var transactionsHelper: BesuTransactionsHelper
  private val log = LogManager.getLogger(this.javaClass)

  @BeforeEach
  fun setUp() {
    configureLoggers(
      rootLevel = Level.WARN,
      logLevels = listOf("maru" to Level.INFO),
    )
    transactionsHelper = BesuTransactionsHelper()
  }

  @AfterEach
  fun tearDown() {
    if (::cluster.isInitialized) {
      cluster.stop()
    }
  }

  @Test
  fun `should start QBFT consensus from prague fork allowing empty blocks`() {
    cluster = MaruCluster()
      .addNode(NodeRole.Sequencer, withBesuEl = true) { nodeBuilder ->
        nodeBuilder.maruConfig { it.copy(allowEmptyBlocks = true) }
      }.start()
    val besuNode = cluster.besuNode("sequencer")

    await
      .atMost(10.seconds.toJavaDuration())
      .ignoreExceptions()
      .until {
        besuNode.latestBlock().number.toLong() > 5
      }
  }

  @Test
  fun `should start QBFT consensus from prague fork not allowing empty blocks`() {
    cluster = MaruCluster()
      .addNode(NodeRole.Sequencer, withBesuEl = true) { nodeBuilder ->
        nodeBuilder.maruConfig { it.copy(allowEmptyBlocks = false) }
      }.start()
    val besuNode = cluster.besuNode("sequencer")

    val totalBlocksToProduce = 5
    repeat(totalBlocksToProduce) {
      transactionsHelper.run {
        besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("account"),
          amount = Amount.ether(1),
        )
      }
    }

    await
      .ignoreExceptions()
      .atMost(10.seconds.toJavaDuration())
      .until {
        besuNode.latestBlock().number.toLong() == totalBlocksToProduce.toLong()
      }

    Thread.sleep(3000) // Wait for the node to process the blocks
    // assert no blocks were produced without transactions
    assertThat(besuNode.latestBlock().number.toLong()).isEqualTo(totalBlocksToProduce.toLong())
  }
}

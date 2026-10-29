/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import linea.testing.besu.BesuTransactionsHelper
import maru.core.Seal
import maru.p2p.NoOpP2PNetwork
import maru.p2p.ValidationResult
import maru.test.cluster.MaruCluster
import maru.test.cluster.NodeRole
import maru.test.cluster.assertSameCanonicalElChain
import maru.test.cluster.configureLoggers
import maru.test.extensions.TestSyncingConfigs
import maru.test.extensions.withFastQbft
import maru.test.extensions.withSyncingConfig
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.kotlin.await
import org.hyperledger.besu.tests.acceptance.dsl.blockchain.Amount
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import testutils.Checks.getMinedBlocks
import testutils.InjectableSealedBlocksFakeNetwork
import testutils.SpyingP2PNetwork
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

class MaruFollowerNegativeTest {
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
  fun `Maru follower doesn't import blocks without proper signature`() {
    // Neither node peers over P2P: the validator's blocks are captured by the spying network
    // and injected into the follower through the fake network
    val spyingP2PNetwork = SpyingP2PNetwork(NoOpP2PNetwork)
    val followerP2PNetwork = InjectableSealedBlocksFakeNetwork()
    cluster = MaruCluster()
      .addNode(NodeRole.Sequencer, withBesuEl = true) { nodeBuilder ->
        nodeBuilder
          .withP2pNetwork(spyingP2PNetwork)
          .withSyncingConfig(TestSyncingConfigs.validator)
          .withFastQbft()
          .maruConfig { it.copy(allowEmptyBlocks = false, p2p = null) }
      }.addNode(NodeRole.Follower, withBesuEl = true) { nodeBuilder ->
        nodeBuilder
          .payloadValidation(true)
          .withP2pNetwork(followerP2PNetwork)
          .withSyncingConfig(TestSyncingConfigs.default)
          .maruConfig { it.copy(allowEmptyBlocks = false, p2p = null) }
      }.start()
    cluster.assertSameCanonicalElChain(chainHeight = 0UL)

    val validatorBesuNode = cluster.besuNode("sequencer")
    val blocksToProduce = 1
    repeat(blocksToProduce) {
      transactionsHelper.run {
        validatorBesuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    await
      .pollDelay(100.milliseconds.toJavaDuration())
      .timeout(5.seconds.toJavaDuration())
      .untilAsserted {
        val blocksProducedByQbftValidator = validatorBesuNode.getMinedBlocks(blocksToProduce)
        assertThat(blocksProducedByQbftValidator).hasSize(1)
      }

    val originalSealedBlock = spyingP2PNetwork.emittedBlockMessages.first()
    val originalSeal = originalSealedBlock.commitSeals.toList().first()
    val alternatedSignature = originalSeal.signature.reversed().toByteArray()
    val blockWithWrongSeal = originalSealedBlock.copy(commitSeals = setOf(Seal(alternatedSignature)))
    val result = followerP2PNetwork.injectSealedBlock(blockWithWrongSeal).get()

    assertThat(result).isInstanceOf(ValidationResult.Companion.Invalid::class.java)
  }
}

/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import linea.kotlin.decodeHex
import linea.testing.besu.BesuFactory
import linea.testing.besu.BesuTransactionsHelper
import linea.testing.besu.ethGetBlockByNumber
import linea.testing.besu.startWithRetry
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.tests.acceptance.dsl.blockchain.Amount
import org.hyperledger.besu.tests.acceptance.dsl.condition.net.NetConditions
import org.hyperledger.besu.tests.acceptance.dsl.node.BesuNode
import org.hyperledger.besu.tests.acceptance.dsl.node.ThreadBesuNodeRunner
import org.hyperledger.besu.tests.acceptance.dsl.node.cluster.Cluster
import org.hyperledger.besu.tests.acceptance.dsl.node.cluster.ClusterConfigurationBuilder
import org.hyperledger.besu.tests.acceptance.dsl.transaction.net.NetTransactions
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import org.web3j.protocol.core.methods.response.EthBlock
import testutils.Checks.checkAllNodesHaveSameBlocks
import testutils.Checks.getMinedBlocks
import testutils.Checks.verifyBlockTime
import testutils.maru.MaruFactory
import testutils.maru.awaitTillMaruHasPeers
import java.io.File
import kotlin.time.Duration.Companion.seconds

class MaruConsensusSwitchTest {
  companion object {
    private const val VANILLA_EXTRA_DATA_LENGTH = 32
  }

  private lateinit var cluster: Cluster
  private lateinit var validatorBesuNode: BesuNode
  private lateinit var validatorMaruNode: MaruApp
  private lateinit var followerBesuNode: BesuNode
  private lateinit var followerMaruNode: MaruApp
  private lateinit var transactionsHelper: BesuTransactionsHelper
  private val log = LogManager.getLogger(this.javaClass)

  @TempDir
  private lateinit var validatorMaruTmpDir: File

  @TempDir
  private lateinit var followerMaruTmpDir: File
  private val net = NetConditions(NetTransactions())

  @BeforeEach
  fun setUp() {
    transactionsHelper = BesuTransactionsHelper()
    // We'll set the switchTimestamp in the test method
    cluster = Cluster(
      ClusterConfigurationBuilder().build(),
      net,
      ThreadBesuNodeRunner(),
    )
  }

  @AfterEach
  fun tearDown() {
    followerMaruNode.stop().get()
    validatorMaruNode.stop().get()
    followerMaruNode.close()
    validatorMaruNode.close()
    cluster.close()
  }

  private fun verifyConsensusSwitch(
    besuNode: BesuNode,
    expectedBlocksBeforeSwitch: Int,
    totalBlocksToProduce: Int,
  ) {
    val blockProducedByQbft = besuNode.ethGetBlockByNumber(1UL)
    assertThat(blockProducedByQbft.extraData.decodeHex().size).isGreaterThan(VANILLA_EXTRA_DATA_LENGTH)

    val blocks = besuNode.getMinedBlocks(totalBlocksToProduce)
    assertThat(blocks.size).isGreaterThanOrEqualTo(expectedBlocksBeforeSwitch + 1)

    val parisSwitchBlockIndex =
      blocks.findSwitchBlock()
        ?: error("Could not find a PoS block (difficulty=0) in the mined block list")
    assertThat(parisSwitchBlockIndex).isEqualTo(expectedBlocksBeforeSwitch)

    blocks.subList(0, parisSwitchBlockIndex).verifyBlockTime()
    blocks.subList(parisSwitchBlockIndex, blocks.size).verifyBlockTime()
  }

  @Test
  fun `follower node correctly switches from QBFT to POS after peering with Sequencer validator`() {
    val stackStartupMargin = 40UL
    val expectedBlocksBeforeSwitch = 5
    var currentTimestamp = (System.currentTimeMillis() / 1000).toULong()
    val shanghaiTimestamp = currentTimestamp + stackStartupMargin + expectedBlocksBeforeSwitch.toULong()
    val cancunTimestamp = shanghaiTimestamp + 10u
    val pragueTimestamp = cancunTimestamp + 10u

    // Send one tx per ~1s block from now until shortly past pragueTimestamp so the chain actually
    // traverses QBFT -> Paris -> Shanghai -> Cancun -> Prague during the test run.
    val postPragueBuffer = 10UL
    val totalBlocksToProduce = (pragueTimestamp + postPragueBuffer - currentTimestamp).toInt()

    // QBFT genesis TD = 1 and each QBFT block adds difficulty 1, so block #N has TD = N + 1.
    // Setting TTD = expectedBlocksBeforeSwitch + 1 makes that block the terminal PoW block,
    // and the next block is the first PoS block driven by Maru.
    val ttd = expectedBlocksBeforeSwitch.toULong() + 1UL
    log.info(
      "Setting Prague switch timestamp to $pragueTimestamp, shanghai switch to $shanghaiTimestamp, Cancun switch to " +
        "$cancunTimestamp, current timestamp: $currentTimestamp",
    )

    validatorBesuNode = BesuFactory.buildSwitchableBesuQbft(
      shanghaiTimestamp = shanghaiTimestamp,
      cancunTimestamp = cancunTimestamp,
      pragueTimestamp = pragueTimestamp,
      ttd = ttd,
      validator = true,
    )
    followerBesuNode = BesuFactory.buildSwitchableBesuQbft(
      shanghaiTimestamp = shanghaiTimestamp,
      cancunTimestamp = cancunTimestamp,
      pragueTimestamp = pragueTimestamp,
      ttd = ttd,
      validator = false,
    )
    cluster.startWithRetry(validatorBesuNode, followerBesuNode)

    // Create a new Maru node with consensus switch configuration
    val validatorEthereumJsonRpcBaseUrl = validatorBesuNode.jsonRpcBaseUrl().get()
    val validatorEngineRpcUrl = validatorBesuNode.engineRpcUrl().get()

    val maruFactory =
      MaruFactory(
        pragueTimestamp = pragueTimestamp,
        cancunTimestamp = cancunTimestamp,
        shanghaiTimestamp = shanghaiTimestamp,
        ttd = ttd,
      )
    validatorMaruNode = maruFactory.buildSwitchableTestMaruValidatorWithP2pPeering(
      ethereumJsonRpcUrl = validatorEthereumJsonRpcBaseUrl,
      engineApiRpc = validatorEngineRpcUrl,
      dataDir = validatorMaruTmpDir.toPath(),
    )
    validatorMaruNode.start().get()

    val followerEthereumJsonRpcBaseUrl = followerBesuNode.jsonRpcBaseUrl().get()
    val followerEngineRpcUrl = followerBesuNode.engineRpcUrl().get()

    followerMaruNode = maruFactory.buildTestMaruFollowerWithConsensusSwitch(
      ethereumJsonRpcUrl = followerEthereumJsonRpcBaseUrl,
      engineApiRpc = followerEngineRpcUrl,
      dataDir = followerMaruTmpDir.toPath(),
      validatorPortForStaticPeering = validatorMaruNode.p2pPort(),
    )
    followerMaruNode.start().get()

    followerMaruNode.awaitTillMaruHasPeers(1u)
    validatorMaruNode.awaitTillMaruHasPeers(1u)
    log.info("Sending transactions")
    repeat(totalBlocksToProduce) {
      transactionsHelper.run {
        validatorBesuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("pre-switch account"),
          amount = Amount.ether(100),
        )
      }
    }

    currentTimestamp = (System.currentTimeMillis() / 1000).toULong()
    log.info("Current timestamp: $currentTimestamp, prague switch timestamp: $pragueTimestamp")
    assertThat(currentTimestamp).isGreaterThan(pragueTimestamp)

    // Wait for both nodes to have all blocks before verifying contents.
    // The follower may still be syncing when the validator has already committed all blocks.
    checkAllNodesHaveSameBlocks(totalBlocksToProduce, validatorBesuNode, followerBesuNode, timeout = 180.seconds)

    verifyConsensusSwitch(
      besuNode = validatorBesuNode,
      expectedBlocksBeforeSwitch = expectedBlocksBeforeSwitch,
      totalBlocksToProduce = totalBlocksToProduce,
    )
    verifyConsensusSwitch(
      besuNode = followerBesuNode,
      expectedBlocksBeforeSwitch = expectedBlocksBeforeSwitch,
      totalBlocksToProduce = totalBlocksToProduce,
    )
  }

  private fun List<EthBlock.Block>.findSwitchBlock(): Int? =
    this
      .indexOfFirst {
        it.difficulty.toInt() == 0
      }.takeIf { it != -1 }
}

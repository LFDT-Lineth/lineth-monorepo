/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import linea.kotlin.decodeHex
import linea.testing.besu.BesuTransactionsHelper
import linea.testing.besu.ethGetBlockByNumber
import maru.consensus.ChainFork
import maru.consensus.ClFork
import maru.consensus.ElFork
import maru.test.cluster.MaruCluster
import maru.test.cluster.NodeRole
import maru.test.cluster.assertSameCanonicalElChain
import maru.test.cluster.configureLoggers
import maru.test.extensions.getBlockByTag
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.awaitility.kotlin.await
import org.hyperledger.besu.tests.acceptance.dsl.blockchain.Amount
import org.hyperledger.besu.tests.acceptance.dsl.node.BesuNode
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.web3j.protocol.core.methods.response.EthBlock
import testutils.Checks.getMinedBlocks
import testutils.Checks.verifyBlockTime
import kotlin.time.Clock
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant
import kotlin.time.toJavaDuration

class MaruConsensusSwitchTest {
  companion object {
    private const val VANILLA_EXTRA_DATA_LENGTH = 32
  }

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

  private fun verifyConsensusSwitch(
    besuNode: BesuNode,
    expectedBlocksBeforeSwitch: Int,
    totalBlocksToProduce: Int,
    shanghaiTimestamp: Instant,
  ) {
    val blockProducedByQbft = besuNode.ethGetBlockByNumber(1UL)
    assertThat(blockProducedByQbft.extraData.decodeHex().size).isGreaterThan(VANILLA_EXTRA_DATA_LENGTH)

    val blocks = besuNode.getMinedBlocks(totalBlocksToProduce)
    assertThat(blocks.size).isGreaterThanOrEqualTo(expectedBlocksBeforeSwitch + 1)

    val parisSwitchBlockIndex =
      blocks.findSwitchBlock()
        ?: error("Could not find a PoS block (difficulty=0) in the mined block list")
    assertThat(parisSwitchBlockIndex).isEqualTo(expectedBlocksBeforeSwitch)
    // Besu itself stops mining at TTD, so the first PoS block is always right after the terminal block. Assert it
    // was produced before the Shanghai timestamp fork, i.e. by the DifficultyAwareQbft -> post-TTD transition and
    // not by the timestamp-based switch to the Shanghai QBFT fork after a stall.
    val firstPosBlock = blocks[parisSwitchBlockIndex]
    assertThat(Instant.fromEpochSeconds(firstPosBlock.timestamp.toLong()))
      .withFailMessage {
        "First PoS block number=${firstPosBlock.number} timestamp=${firstPosBlock.timestamp} is not before " +
          "shanghaiTimestamp=$shanghaiTimestamp, so it was not produced by the post-TTD transition"
      }.isLessThan(shanghaiTimestamp)

    blocks.subList(0, parisSwitchBlockIndex).verifyBlockTime()
    blocks.subList(parisSwitchBlockIndex, blocks.size).verifyBlockTime()
  }

  @Test
  fun `follower node correctly switches from QBFT to POS after peering with Sequencer validator`() {
    val clusterStartupMargin = 20.seconds
    val expectedBlocksBeforeSwitch = 5
    var currentTimestamp = Clock.System.now()
    val shanghaiTimestamp = currentTimestamp + clusterStartupMargin + expectedBlocksBeforeSwitch.seconds
    val cancunTimestamp = shanghaiTimestamp + 5.seconds
    val pragueTimestamp = cancunTimestamp + 5.seconds

    // QBFT genesis TD = 1 and each QBFT block adds difficulty 1, so block #N has TD = N + 1.
    // Setting TTD = expectedBlocksBeforeSwitch + 1 makes that block the terminal PoW block,
    // and the next block is the first PoS block driven by Maru.
    val ttd = expectedBlocksBeforeSwitch.toULong() + 1UL
    log.info(
      "Setting Prague switch timestamp to $pragueTimestamp, shanghai switch to $shanghaiTimestamp, Cancun switch to " +
        "$cancunTimestamp, current timestamp: $currentTimestamp",
    )

    cluster = MaruCluster(
      blockTimeSeconds = 1u,
      terminalTotalDifficulty = ttd,
      chainForks = mapOf(
        Instant.fromEpochSeconds(0) to ChainFork(ClFork.QBFT_PHASE0, ElFork.Paris),
        shanghaiTimestamp to ChainFork(ClFork.QBFT_PHASE0, ElFork.Shanghai),
        cancunTimestamp to ChainFork(ClFork.QBFT_PHASE0, ElFork.Cancun),
        pragueTimestamp to ChainFork(ClFork.QBFT_PHASE0, ElFork.Prague),
      ),

    ).addNode(NodeRole.Sequencer, withBesuEl = true) { nodeBuilder ->
      nodeBuilder.maruConfig { it.copy(allowEmptyBlocks = false) }
    }.addNode(NodeRole.Follower, withBesuEl = true) { nodeBuilder ->
      nodeBuilder
        .staticPeers(listOf("sequencer"))
        .maruConfig { it.copy(syncing = it.syncing.copy(desyncTolerance = 10UL)) }
    }.start()

    val validatorBesuNode = cluster.besuNode("sequencer")
    val followerBesuNode = cluster.besuNode("follower")
    val validatorMaruNode = cluster.maruNode("sequencer")
    val followerMaruNode = cluster.maruNode("follower")

    await
      .timeout(60.seconds.toJavaDuration())
      .pollInterval(500.milliseconds.toJavaDuration())
      .untilAsserted {
        assertThat(validatorMaruNode.p2pNetwork.peerCount).isGreaterThanOrEqualTo(1)
        assertThat(followerMaruNode.p2pNetwork.peerCount).isGreaterThanOrEqualTo(1)
      }
    log.info("Sending transactions")
    do {
      transactionsHelper.run {
        validatorBesuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("pre-switch account"),
          amount = Amount.ether(1),
        )
      }
    } while (Clock.System.now() < pragueTimestamp + 2.seconds)

    await()
      .atMost(3.seconds.toJavaDuration())
      .pollInterval(100.milliseconds.toJavaDuration())
      .untilAsserted {
        assertThat(Instant.fromEpochSeconds(validatorBesuNode.getBlockByTag("latest").timestamp.toLong()))
          .isGreaterThanOrEqualTo(pragueTimestamp)
      }

    val sequencerHead = validatorBesuNode.getBlockByTag("latest").number.toInt()
    // Wait for both nodes to have all blocks before verifying contents.
    // The follower may still be syncing when the validator has already committed all blocks.
    cluster.assertSameCanonicalElChain(sequencerHead.toULong(), timeout = 100.seconds)

    verifyConsensusSwitch(
      besuNode = validatorBesuNode,
      expectedBlocksBeforeSwitch = expectedBlocksBeforeSwitch,
      totalBlocksToProduce = sequencerHead,
      shanghaiTimestamp = shanghaiTimestamp,
    )
    verifyConsensusSwitch(
      besuNode = followerBesuNode,
      expectedBlocksBeforeSwitch = expectedBlocksBeforeSwitch,
      totalBlocksToProduce = sequencerHead,
      shanghaiTimestamp = shanghaiTimestamp,
    )
  }

  private fun List<EthBlock.Block>.findSwitchBlock(): Int? =
    this
      .indexOfFirst {
        it.difficulty.toInt() == 0
      }.takeIf { it != -1 }
}

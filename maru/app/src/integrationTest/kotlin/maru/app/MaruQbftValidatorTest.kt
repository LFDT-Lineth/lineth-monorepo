/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import com.github.michaelbull.result.Ok
import linea.kotlin.decodeHex
import linea.testing.besu.BesuTransactionsHelper
import maru.consensus.StaticValidatorProvider
import maru.consensus.qbft.toAddress
import maru.consensus.validation.QuorumOfSealsVerifier
import maru.consensus.validation.SCEP256SealVerifier
import maru.core.Validator
import maru.crypto.PrivateKeyGenerator
import maru.p2p.NoOpP2PNetwork
import maru.test.cluster.MaruCluster
import maru.test.cluster.NodeRole
import maru.test.cluster.configureLoggers
import maru.test.extensions.TestSyncingConfigs
import maru.test.extensions.withFastQbft
import maru.test.extensions.withSyncingConfig
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.kotlin.await
import org.hyperledger.besu.consensus.qbft.core.messagewrappers.Commit
import org.hyperledger.besu.consensus.qbft.core.messagewrappers.Prepare
import org.hyperledger.besu.consensus.qbft.core.messagewrappers.Proposal
import org.hyperledger.besu.consensus.qbft.core.messagewrappers.RoundChange
import org.hyperledger.besu.tests.acceptance.dsl.blockchain.Amount
import org.hyperledger.besu.tests.acceptance.dsl.node.BesuNode
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import testutils.Checks.getMinedBlocks
import testutils.Checks.verifyBlockTime
import testutils.Checks.verifyBlockTimeWithAGapOn
import testutils.SpyingP2PNetwork

class MaruQbftValidatorTest {
  private lateinit var cluster: MaruCluster
  private lateinit var transactionsHelper: BesuTransactionsHelper
  private val log = LogManager.getLogger(this.javaClass)
  private lateinit var spyingP2pNetwork: SpyingP2PNetwork
  private val validatorKeyData = PrivateKeyGenerator.generatePrivateKey()
  private val validator = Validator(validatorKeyData.address)

  private val besuNode: BesuNode
    get() = cluster.besuNode("sequencer")

  @BeforeEach
  fun setUp() {
    configureLoggers(
      rootLevel = Level.WARN,
      logLevels = listOf("maru" to Level.INFO),
    )
    transactionsHelper = BesuTransactionsHelper()

    // Single validator without P2P peering: QBFT messages are captured by the spying network
    spyingP2pNetwork = SpyingP2PNetwork(NoOpP2PNetwork)
    cluster = MaruCluster()
      .addNode(NodeRole.Sequencer, withBesuEl = true) { nodeBuilder ->
        nodeBuilder
          .withNodeKey(validatorKeyData.prefixedPrivateKey)
          .withP2pNetwork(spyingP2pNetwork)
          .withSyncingConfig(TestSyncingConfigs.validator)
          .withFastQbft()
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
  fun `Maru is producing blocks with expected block time and emits messages`() {
    val blocksToProduce = 10
    repeat(blocksToProduce) {
      transactionsHelper.run {
        besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    val blocks = besuNode.getMinedBlocks(blocksToProduce)
    blocks.verifyBlockTime()
    blocks.forEach {
      assertThat(it.miner.decodeHex()).isEqualTo(
        cluster
          .maruNode("sequencer")
          .config.qbft!!
          .feeRecipient,
      )
    }

    // Need to wait because otherwise not all of the messages might be emitted at the time of a block being mined
    await.untilAsserted {
      validateRoundChange(maxBlockNumber = blocksToProduce.toLong())
    }

    for (blockNumber in 1L..blocksToProduce) {
      assertThat(
        anyPrepareWithBlockNumber(blockNumber),
      ).withFailMessage { "Didn't find any prepare messages for blockNumber=$blockNumber" }
        .isTrue
      assertThat(
        anyCommitWithBlockNumber(blockNumber),
      ).withFailMessage { "Didn't find any commit messages for blockNumber=$blockNumber" }
        .isTrue
      assertThat(
        anyProposalWithBlockNumber(blockNumber),
      ).withFailMessage { "Didn't find any proposal messages for blockNumber=$blockNumber" }
        .isTrue
    }
    allMessagesAreSignedByTheExpectedSigner()
    allBlocksAreSignedByTheExpectedSigner()
  }

  private fun anyPrepareWithBlockNumber(blockNumber: Long): Boolean =
    spyingP2pNetwork.emittedQbftMessages.any {
      it is Prepare && it.roundIdentifier.sequenceNumber == blockNumber
    }

  private fun anyProposalWithBlockNumber(blockNumber: Long): Boolean =
    spyingP2pNetwork.emittedQbftMessages.any {
      it is Proposal && it.roundIdentifier.sequenceNumber == blockNumber
    }

  private fun anyCommitWithBlockNumber(blockNumber: Long): Boolean =
    spyingP2pNetwork.emittedQbftMessages.any {
      it is Commit && it.roundIdentifier.sequenceNumber == blockNumber
    }

  private fun allBlocksAreSignedByTheExpectedSigner() {
    val validatorProvider = StaticValidatorProvider(setOf(validator))
    val sealVerifier = SCEP256SealVerifier()
    val sealsVerifier = QuorumOfSealsVerifier(validatorProvider = validatorProvider, sealVerifier)
    spyingP2pNetwork.emittedBlockMessages.forEach { sealedBeaconBlock ->
      val verificationResult =
        sealsVerifier
          .verifySeals(
            sealedBeaconBlock.commitSeals,
            sealedBeaconBlock.beaconBlock.beaconBlockHeader,
          ).get()
      assertThat(verificationResult).isEqualTo(Ok(Unit))
    }
  }

  // All blocks except the first must be produced within 0th round. Absence of transactions will trigger RoundChange
  // events post test
  private fun validateRoundChange(maxBlockNumber: Long) {
    val roundChangeMessages =
      spyingP2pNetwork.emittedQbftMessages.filter {
        it is RoundChange
      }
    assertThat(roundChangeMessages).isNotEmpty()
    roundChangeMessages.forEach { roundChange ->
      assertThat(
        roundChange.roundIdentifier.sequenceNumber == 1L ||
          roundChange.roundIdentifier.sequenceNumber > maxBlockNumber,
      ).withFailMessage { "Unexpected RoundChange! $roundChange" }
        .isTrue
    }
  }

  private fun allMessagesAreSignedByTheExpectedSigner() {
    spyingP2pNetwork.emittedQbftMessages.forEach {
      assertThat(it.author)
        .withFailMessage { "Unexpected signer address for message=$it author=${it.author}" }
        .isEqualTo(validator.toAddress())
    }
  }

  @Test
  fun `Maru works if Besu stops mid flight`() {
    val blocksToProduce = 5
    repeat(blocksToProduce) {
      transactionsHelper.run {
        besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }
    cluster.besuCluster.stopNode(besuNode.name)
    Thread.sleep(3000)
    startBesuWithRetry(label = "sequencer")

    repeat(blocksToProduce) {
      transactionsHelper.run {
        besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    val blocks = besuNode.getMinedBlocks(blocksToProduce * 2)
    blocks.verifyBlockTimeWithAGapOn(blocksToProduce)
  }

  @Test
  fun `Maru works after restart`() {
    val blocksToProduce = 5
    repeat(blocksToProduce) {
      transactionsHelper.run {
        besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }
    // Stops and starts the very same Maru instance
    val maruApp = cluster.maruNode("sequencer")
    maruApp.stop().get()
    Thread.sleep(3000)
    maruApp.start().get()
    repeat(blocksToProduce) {
      transactionsHelper.run {
        besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    val blocks = besuNode.getMinedBlocks(blocksToProduce * 2)
    blocks.verifyBlockTimeWithAGapOn(blocksToProduce)
  }

  @Test
  fun `Maru works after recreation`() {
    val blocksToProduce = 5
    repeat(blocksToProduce) {
      transactionsHelper.run {
        besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }
    cluster.stopMaruNode("sequencer")

    Thread.sleep(3000)
    // The difference from the previous test is that BeaconChain is instantiated with the Maru instance and it's not
    // affected by start and stop. The new Maru instance reuses the node key, data directory and Besu node
    cluster.startMaruNode("sequencer")
    repeat(blocksToProduce) {
      transactionsHelper.run {
        besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    val blocks = besuNode.getMinedBlocks(blocksToProduce * 2)

    blocks.verifyBlockTimeWithAGapOn(blocksToProduce)
  }

  // Starts again a Besu node previously stopped in the Besu cluster, retrying as its ports may not be released yet
  private fun startBesuWithRetry(
    label: String,
    maxAttempts: Int = 10,
  ) {
    val besuName = cluster.besuNode(label).name
    val elNode = cluster.elNode(label)!!
    repeat(maxAttempts) { attempt ->
      try {
        elNode.start()
        return
      } catch (e: IllegalStateException) {
        if (attempt == maxAttempts - 1) throw e
        log.warn("Besu start attempt={} failed, retrying", attempt, e)
        cluster.besuCluster.stopNode(besuName)
        Thread.sleep(1000)
      }
    }
  }
}

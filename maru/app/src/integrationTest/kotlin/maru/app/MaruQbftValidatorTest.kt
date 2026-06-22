/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import com.github.michaelbull.result.Ok
import linea.kotlin.decodeHex
import linea.testing.besu.BesuTransactionsHelper
import linea.testing.besu.startWithRetry
import maru.consensus.StaticValidatorProvider
import maru.consensus.qbft.toAddress
import maru.consensus.validation.QuorumOfSealsVerifier
import maru.consensus.validation.SCEP256SealVerifier
import maru.core.Validator
import maru.p2p.NoOpP2PNetwork
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.kotlin.await
import org.hyperledger.besu.consensus.qbft.core.messagewrappers.Commit
import org.hyperledger.besu.consensus.qbft.core.messagewrappers.Prepare
import org.hyperledger.besu.consensus.qbft.core.messagewrappers.Proposal
import org.hyperledger.besu.consensus.qbft.core.messagewrappers.RoundChange
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
import testutils.Checks.verifyBlockTime
import testutils.Checks.verifyBlockTimeWithAGapOn
import testutils.SingleNodeNetworkStack
import testutils.SpyingP2PNetwork
import testutils.maru.MaruFactory

class MaruQbftValidatorTest {
  private lateinit var cluster: Cluster
  private lateinit var networkParticipantStack: SingleNodeNetworkStack
  private lateinit var transactionsHelper: BesuTransactionsHelper
  private val log = LogManager.getLogger(this.javaClass)
  private lateinit var spyingP2pNetwork: SpyingP2PNetwork
  private val maruFactory = MaruFactory()

  @BeforeEach
  fun setUp() {
    transactionsHelper = BesuTransactionsHelper()
    cluster = Cluster(
      ClusterConfigurationBuilder().build(),
      NetConditions(NetTransactions()),
      ThreadBesuNodeRunner(),
    )

    spyingP2pNetwork = SpyingP2PNetwork(NoOpP2PNetwork)
    networkParticipantStack = SingleNodeNetworkStack(
      cluster = cluster,
    ) { ethereumJsonRpcBaseUrl, engineRpcUrl, tmpDir ->
      maruFactory.buildTestMaruValidatorWithoutP2pPeering(
        ethereumJsonRpcUrl = ethereumJsonRpcBaseUrl,
        engineApiRpc = engineRpcUrl,
        dataDir = tmpDir,
        overridingP2PNetwork = spyingP2pNetwork,
      )
    }
    networkParticipantStack.maruApp.start().get()
  }

  @AfterEach
  fun tearDown() {
    networkParticipantStack.maruApp.stop().get()
    networkParticipantStack.maruApp.close()
    cluster.close()
  }

  @Test
  fun `Maru is producing blocks with expected block time and emits messages`() {
    val blocksToProduce = 10
    repeat(blocksToProduce) {
      transactionsHelper.run {
        networkParticipantStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    val blocks = networkParticipantStack.besuNode.getMinedBlocks(blocksToProduce)
    blocks.verifyBlockTime()
    blocks.forEach {
      assertThat(it.miner.decodeHex()).isEqualTo(
        networkParticipantStack.maruApp.config.qbft!!
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
    val validatorProvider = StaticValidatorProvider(setOf(Validator(maruFactory.validatorAddress.decodeHex())))
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
        .isEqualTo(maruFactory.qbftValidator.toAddress())
    }
  }

  @Test
  fun `Maru works if Besu stops mid flight`() {
    val blocksToProduce = 5
    repeat(blocksToProduce) {
      transactionsHelper.run {
        networkParticipantStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }
    cluster.stop()
    Thread.sleep(3000)
    cluster.startWithRetry(networkParticipantStack.besuNode)

    repeat(blocksToProduce) {
      transactionsHelper.run {
        networkParticipantStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    val blocks = networkParticipantStack.besuNode.getMinedBlocks(blocksToProduce * 2)
    blocks.verifyBlockTimeWithAGapOn(blocksToProduce)
  }

  @Test
  fun `Maru works after restart`() {
    val blocksToProduce = 5
    repeat(blocksToProduce) {
      transactionsHelper.run {
        networkParticipantStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }
    networkParticipantStack.maruApp.stop().get()
    Thread.sleep(3000)
    networkParticipantStack.maruApp.start().get()
    repeat(blocksToProduce) {
      transactionsHelper.run {
        networkParticipantStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    val blocks = networkParticipantStack.besuNode.getMinedBlocks(blocksToProduce * 2)
    blocks.verifyBlockTimeWithAGapOn(blocksToProduce)
  }

  @Test
  fun `Maru works after recreation`() {
    val blocksToProduce = 5
    repeat(blocksToProduce) {
      transactionsHelper.run {
        networkParticipantStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }
    networkParticipantStack.maruApp.stop().get()
    networkParticipantStack.maruApp.close()

    Thread.sleep(3000)
    networkParticipantStack.maruApp =
      maruFactory.buildTestMaruValidatorWithoutP2pPeering(
        ethereumJsonRpcUrl = networkParticipantStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = networkParticipantStack.besuNode.engineRpcUrl().get(),
        dataDir = networkParticipantStack.tmpDir,
      )
    // The difference from the previous test is that BeaconChain is instantiated with the Maru instance and it's not
    // affected by start and stop
    networkParticipantStack.maruApp.start().get()
    repeat(blocksToProduce) {
      transactionsHelper.run {
        networkParticipantStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    val blocks = networkParticipantStack.besuNode.getMinedBlocks(blocksToProduce * 2)

    blocks.verifyBlockTimeWithAGapOn(blocksToProduce)
  }
}

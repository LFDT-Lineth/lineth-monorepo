/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import io.libp2p.etc.types.fromHex
import linea.kotlin.encodeHex
import linea.testing.besu.BesuFactory
import maru.consensus.qbft.ProposerSelectorImpl
import maru.core.SealedBeaconBlock
import maru.core.Validator
import maru.crypto.SecpCrypto
import maru.database.BeaconChain
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.kotlin.await
import org.hyperledger.besu.consensus.common.bft.ConsensusRoundIdentifier
import org.hyperledger.besu.tests.acceptance.dsl.condition.net.NetConditions
import org.hyperledger.besu.tests.acceptance.dsl.node.ThreadBesuNodeRunner
import org.hyperledger.besu.tests.acceptance.dsl.node.cluster.Cluster
import org.hyperledger.besu.tests.acceptance.dsl.node.cluster.ClusterConfigurationBuilder
import org.hyperledger.besu.tests.acceptance.dsl.transaction.net.NetTransactions
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import testutils.PeeringNodeNetworkStack
import testutils.maru.MaruFactory
import testutils.maru.awaitTillMaruHasPeers
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

class MaruMultiValidatorTest {
  companion object {
    /** Number of consecutive round-0 blocks required to declare convergence / stable production. */
    private const val STABLE_BLOCKS = 5

    private val multiValidatorSyncingConfig = MaruFactory.defaultValidatorSyncingConfig
  }

  private val key0 = "080212201dd171cec7e2995408b5513004e8207fe88d6820aeff0d82463b3e41df251aae".fromHex()
  private val key1 = "0802122100abb81ba53518eb0a206dfe80f2a973182e5d66c98cd31d00bf7471fcd5514157".fromHex()
  private val key2 = "080212202fec0750fe3edc7e8272d4814a36b632921fc5e835d20a2de874471e8ad9ad0b".fromHex()
  private val key3 = "080212207a19c01ce2246b94b48ed778d9bfb3b76eaabe8193c468d6751f4e4d1adf98a8".fromHex()

  private lateinit var cluster: Cluster
  private lateinit var stack0: PeeringNodeNetworkStack
  private lateinit var stack1: PeeringNodeNetworkStack
  private lateinit var stack2: PeeringNodeNetworkStack
  private lateinit var stack3: PeeringNodeNetworkStack

  private val log = LogManager.getLogger(this.javaClass)

  private val maruFactory0 = MaruFactory(validatorPrivateKey = key0)
  private val maruFactory1 = MaruFactory(validatorPrivateKey = key1)
  private val maruFactory2 = MaruFactory(validatorPrivateKey = key2)
  private val maruFactory3 = MaruFactory(validatorPrivateKey = key3)

  private val initialValidators: Set<Validator> by lazy {
    setOf(
      SecpCrypto.privateKeyToValidator(SecpCrypto.privateKeyBytesWithoutPrefix(key0)),
      SecpCrypto.privateKeyToValidator(SecpCrypto.privateKeyBytesWithoutPrefix(key1)),
      SecpCrypto.privateKeyToValidator(SecpCrypto.privateKeyBytesWithoutPrefix(key2)),
      SecpCrypto.privateKeyToValidator(SecpCrypto.privateKeyBytesWithoutPrefix(key3)),
    )
  }

  @BeforeEach
  fun setUp() {
    cluster = Cluster(
      ClusterConfigurationBuilder().build(),
      NetConditions(NetTransactions()),
      ThreadBesuNodeRunner(),
    )
    val besuBuilder = { BesuFactory.buildTestBesu(validator = false) }
    stack0 = PeeringNodeNetworkStack(besuBuilder)
    stack1 = PeeringNodeNetworkStack(besuBuilder)
    stack2 = PeeringNodeNetworkStack(besuBuilder)
    stack3 = PeeringNodeNetworkStack(besuBuilder)
    PeeringNodeNetworkStack.startBesuNodes(cluster, stack0, stack1, stack2, stack3)
  }

  @AfterEach
  fun tearDown() {
    runCatching { stack3.maruApp.stop().get() }
    runCatching { stack2.maruApp.stop().get() }
    runCatching { stack1.maruApp.stop().get() }
    runCatching { stack0.maruApp.stop().get() }
    runCatching { stack3.maruApp.close() }
    runCatching { stack2.maruApp.close() }
    runCatching { stack1.maruApp.close() }
    runCatching { stack0.maruApp.close() }
    cluster.close()
  }

  // -- Helper methods ---------------------------------------------------------

  private fun startAllValidators() {
    val app0 =
      maruFactory0.buildTestMaruValidatorWithP2pPeering(
        ethereumJsonRpcUrl = stack0.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = stack0.besuNode.engineRpcUrl().get(),
        dataDir = stack0.tmpDir,
        syncingConfig = multiValidatorSyncingConfig,
        allowEmptyBlocks = true,
        initialValidators = initialValidators,
      )
    stack0.setMaruApp(app0)
    app0.start().get()

    val app1 =
      maruFactory1.buildTestMaruValidatorWithP2pPeering(
        ethereumJsonRpcUrl = stack1.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = stack1.besuNode.engineRpcUrl().get(),
        dataDir = stack1.tmpDir,
        syncingConfig = multiValidatorSyncingConfig,
        allowEmptyBlocks = true,
        initialValidators = initialValidators,
      )
    stack1.setMaruApp(app1)
    app1.start().get()

    val app2 =
      maruFactory2.buildTestMaruValidatorWithP2pPeering(
        ethereumJsonRpcUrl = stack2.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = stack2.besuNode.engineRpcUrl().get(),
        dataDir = stack2.tmpDir,
        syncingConfig = multiValidatorSyncingConfig,
        allowEmptyBlocks = true,
        initialValidators = initialValidators,
      )
    stack2.setMaruApp(app2)
    app2.start().get()

    val app3 =
      maruFactory3.buildTestMaruValidatorWithP2pPeering(
        ethereumJsonRpcUrl = stack3.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = stack3.besuNode.engineRpcUrl().get(),
        dataDir = stack3.tmpDir,
        syncingConfig = multiValidatorSyncingConfig,
        allowEmptyBlocks = true,
        initialValidators = initialValidators,
      )
    stack3.setMaruApp(app3)
    app3.start().get()

    // Wire full mesh: 6 bidirectional connections for 4 nodes
    fun peerAddr(app: MaruApp) = "/ip4/127.0.0.1/tcp/${app.p2pPort()}/p2p/${app.p2pNetwork.nodeId}"
    app1.p2pNetwork.addPeer(peerAddr(app0))
    app2.p2pNetwork.addPeer(peerAddr(app0))
    app2.p2pNetwork.addPeer(peerAddr(app1))
    app3.p2pNetwork.addPeer(peerAddr(app0))
    app3.p2pNetwork.addPeer(peerAddr(app1))
    app3.p2pNetwork.addPeer(peerAddr(app2))

    // Wait for full mesh -- each validator should see exactly 3 peers
    app0.awaitTillMaruHasPeers(3u, pollingInterval = 500.milliseconds)
    log.info(
      "Validator 0 has 3 peers (height=${stack0.maruApp.beaconChain.getLatestBeaconState().beaconBlockHeader.number})",
    )
    app1.awaitTillMaruHasPeers(3u, pollingInterval = 500.milliseconds)
    log.info(
      "Validator 1 has 3 peers (height=${stack1.maruApp.beaconChain.getLatestBeaconState().beaconBlockHeader.number})",
    )
    app2.awaitTillMaruHasPeers(3u, pollingInterval = 500.milliseconds)
    log.info(
      "Validator 2 has 3 peers (height=${stack2.maruApp.beaconChain.getLatestBeaconState().beaconBlockHeader.number})",
    )
    app3.awaitTillMaruHasPeers(3u, pollingInterval = 500.milliseconds)
    log.info(
      "Validator 3 has 3 peers (height=${stack3.maruApp.beaconChain.getLatestBeaconState().beaconBlockHeader.number})",
    )
    log.info("All 4 validators peered in full mesh")
  }

  private fun waitForBlockHeight(
    vararg beaconChains: BeaconChain,
    targetHeight: ULong,
    timeout: Duration = 240.seconds,
  ) {
    await
      .timeout(timeout.toJavaDuration())
      .pollInterval(500.milliseconds.toJavaDuration())
      .untilAsserted {
        beaconChains.forEachIndexed { idx, chain ->
          assertThat(chain.getLatestBeaconState().beaconBlockHeader.number)
            .withFailMessage { "Validator $idx has not reached block $targetHeight yet" }
            .isGreaterThanOrEqualTo(targetHeight)
        }
      }
  }

  /**
   * Polls [beaconChain] until [requiredConsecutive] consecutive round-0 blocks have been committed.
   * Returns the block number of the last block in the first qualifying run.
   *
   * This is the proper way to detect QBFT convergence: during startup, validators run independently
   * before the P2P mesh is wired, causing round skips. Checking a fixed block number is unreliable;
   * instead we wait for a stable run of round-0 blocks.
   */
  private fun waitForConsecutiveRound0Blocks(
    beaconChain: BeaconChain,
    requiredConsecutive: Int = 5,
    timeout: Duration = 240.seconds,
  ): ULong {
    var consecutiveCount = 0
    var lastStableBlock = 0uL
    var lastPolled = 0uL

    await
      .timeout(timeout.toJavaDuration())
      .pollInterval(500.milliseconds.toJavaDuration())
      .until {
        val latestHeight = beaconChain.getLatestBeaconState().beaconBlockHeader.number
        log.info(
          "waitForConsecutiveRound0Blocks: polled height=$latestHeight, " +
            "lastPolled=$lastPolled, consecutiveCount=$consecutiveCount",
        )
        for (blockNum in (lastPolled + 1uL)..latestHeight) {
          val block = beaconChain.getSealedBeaconBlock(blockNum) ?: break
          val round = block.beaconBlock.beaconBlockHeader.round
          if (round == 0u) {
            consecutiveCount++
            lastStableBlock = blockNum
            log.info("Block $blockNum: round=0 (consecutive=$consecutiveCount)")
          } else {
            log.info("Block $blockNum has round=$round — resetting consecutive count (was $consecutiveCount)")
            consecutiveCount = 0
            lastStableBlock = 0uL
          }
          lastPolled = blockNum
        }
        consecutiveCount >= requiredConsecutive
      }

    return lastStableBlock
  }

  private fun currentBlockHeight(stack: PeeringNodeNetworkStack): ULong =
    stack.maruApp.beaconChain
      .getLatestBeaconState()
      .beaconBlockHeader.number

  private fun stopValidator(stack: PeeringNodeNetworkStack) {
    stack.maruApp.stop().get()
    stack.maruApp.close()
  }

  private fun restartValidator(
    stack: PeeringNodeNetworkStack,
    factory: MaruFactory,
    peersToConnect: List<MaruApp>,
  ) {
    val app =
      factory.buildTestMaruValidatorWithP2pPeering(
        ethereumJsonRpcUrl = stack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = stack.besuNode.engineRpcUrl().get(),
        dataDir = stack.tmpDir,
        syncingConfig = multiValidatorSyncingConfig,
        allowEmptyBlocks = true,
        initialValidators = initialValidators,
      )
    stack.setMaruApp(app)
    app.start().get()

    fun peerAddr(peer: MaruApp) = "/ip4/127.0.0.1/tcp/${peer.p2pPort()}/p2p/${peer.p2pNetwork.nodeId}"
    peersToConnect.forEach { peer ->
      app.p2pNetwork.addPeer(peerAddr(peer))
    }
    app.awaitTillMaruHasPeers(peersToConnect.size.toUInt(), pollingInterval = 500.milliseconds)
  }

  private fun <T, M> checkAllValidatorBlocksAreTheSame(
    validatorBlocks: List<() -> List<T>>,
    blocksToMetadata: (List<T>) -> List<M>,
  ) {
    val allMetadata = validatorBlocks.map { blocksToMetadata(it()) }
    for (i in 1 until allMetadata.size) {
      assertThat(allMetadata[i])
        .withFailMessage { "Validator $i blocks differ from validator 0" }
        .isEqualTo(allMetadata[0])
    }
  }

  private fun checkBlockProposersMatchExpectedProposers(
    beaconChain: BeaconChain,
    startBlock: ULong,
    endBlock: ULong,
  ) {
    val count = endBlock - startBlock + 1uL
    val blocks = beaconChain.getSealedBeaconBlocks(startBlock, count)
    val proposerSelector = ProposerSelectorImpl

    blocks.forEach { block ->
      val beaconBlockHeader = block.beaconBlock.beaconBlockHeader
      val roundIdentifier =
        ConsensusRoundIdentifier(beaconBlockHeader.number.toLong(), beaconBlockHeader.round.toInt())
      val parentBeaconState = beaconChain.getBeaconState(beaconBlockHeader.number - 1uL)
      val expectedProposer = proposerSelector.getProposerForBlock(parentBeaconState!!, roundIdentifier).get()

      assertThat(beaconBlockHeader.proposer)
        .withFailMessage {
          "Block ${beaconBlockHeader.number} should be proposed by ${expectedProposer.address.encodeHex()} " +
            "but was proposed by ${beaconBlockHeader.proposer.address.encodeHex()}"
        }.isEqualTo(expectedProposer)
    }
  }

  private fun clBlocksToMetadata(blocks: List<SealedBeaconBlock>): List<Pair<ULong, String>> =
    blocks.map {
      it.beaconBlock.beaconBlockHeader.number to
        it.beaconBlock.beaconBlockHeader.hash
          .encodeHex()
    }

  // -- Test scenarios ---------------------------------------------------------

  @Test
  fun `validators converge to stable block production without round skips`() {
    startAllValidators()

    // Wait until 5 consecutive round-0 blocks are observed. During startup, validators run QBFT
    // independently before the P2P mesh is wired, causing round skips on early blocks
    val stableHeight =
      waitForConsecutiveRound0Blocks(
        stack0.maruApp.beaconChain,
        requiredConsecutive = STABLE_BLOCKS,
        timeout = 240.seconds,
      )
    log.info("QBFT convergence achieved at block $stableHeight")

    // Verify STABLE_BLOCKS more blocks after convergence are also round-0.
    // Wait for ALL validators to reach the target so blocks can safely be read from all.
    waitForBlockHeight(
      stack0.maruApp.beaconChain,
      stack1.maruApp.beaconChain,
      stack2.maruApp.beaconChain,
      stack3.maruApp.beaconChain,
      targetHeight = stableHeight + STABLE_BLOCKS.toULong(),
      timeout = 90.seconds,
    )
    val verifyStart = stableHeight - (STABLE_BLOCKS - 1).toULong()
    val verifyCount = (STABLE_BLOCKS * 2).toULong()
    val verifyBlocks = stack0.maruApp.beaconChain.getSealedBeaconBlocks(verifyStart, verifyCount)
    verifyBlocks.forEach { block ->
      val header = block.beaconBlock.beaconBlockHeader
      assertThat(header.round)
        .withFailMessage { "Block ${header.number} has round ${header.round}, expected 0" }
        .isEqualTo(0u)
    }

    // Verify blocks consistent across all 4 validators
    checkAllValidatorBlocksAreTheSame(
      validatorBlocks = listOf(
        { stack0.maruApp.beaconChain.getSealedBeaconBlocks(verifyStart, verifyCount) },
        { stack1.maruApp.beaconChain.getSealedBeaconBlocks(verifyStart, verifyCount) },
        { stack2.maruApp.beaconChain.getSealedBeaconBlocks(verifyStart, verifyCount) },
        { stack3.maruApp.beaconChain.getSealedBeaconBlocks(verifyStart, verifyCount) },
      ),
      blocksToMetadata = ::clBlocksToMetadata,
    )

    // Verify proposers match expected
    checkBlockProposersMatchExpectedProposers(
      beaconChain = stack0.maruApp.beaconChain,
      startBlock = verifyStart,
      endBlock = stableHeight + STABLE_BLOCKS.toULong(),
    )
  }

  @Test
  fun `block production continues with 1 node offline`() {
    startAllValidators()

    // Wait for convergence before stopping a node
    waitForConsecutiveRound0Blocks(
      stack0.maruApp.beaconChain,
      requiredConsecutive = STABLE_BLOCKS,
      timeout = 240.seconds,
    )

    // Stop validator 3
    log.info("Stopping validator 3")
    stopValidator(stack3)

    // Record current height and wait for STABLE_BLOCKS more blocks
    val heightAfterStop = currentBlockHeight(stack0)
    log.info("Height after stopping validator 3: $heightAfterStop")
    // Wait for all 3 remaining validators so getSealedBeaconBlocks doesn't throw on any of them.
    waitForBlockHeight(
      stack0.maruApp.beaconChain,
      stack1.maruApp.beaconChain,
      stack2.maruApp.beaconChain,
      targetHeight = heightAfterStop + STABLE_BLOCKS.toULong(),
      timeout = 240.seconds,
    )

    // Verify blocks consistent across the 3 remaining validators
    val verifyStart = heightAfterStop + 1uL
    val count = STABLE_BLOCKS.toULong()
    checkAllValidatorBlocksAreTheSame(
      validatorBlocks = listOf(
        { stack0.maruApp.beaconChain.getSealedBeaconBlocks(verifyStart, count) },
        { stack1.maruApp.beaconChain.getSealedBeaconBlocks(verifyStart, count) },
        { stack2.maruApp.beaconChain.getSealedBeaconBlocks(verifyStart, count) },
      ),
      blocksToMetadata = ::clBlocksToMetadata,
    )
    log.info("Block production continued successfully with 3 of 4 validators")
  }

  @Test
  fun `block production recovers after 2 nodes offline and 1 returns`() {
    startAllValidators()

    // Wait for convergence before stopping nodes
    waitForConsecutiveRound0Blocks(
      stack0.maruApp.beaconChain,
      requiredConsecutive = STABLE_BLOCKS,
      timeout = 240.seconds,
    )

    // Stop validators 2 and 3 -- only 2 of 4 remain, below quorum (need 3)
    log.info("Stopping validators 2 and 3")
    stopValidator(stack2)
    stopValidator(stack3)

    val heightAfterStop = currentBlockHeight(stack0)
    log.info("Height after stopping 2 validators: $heightAfterStop")

    // Wait 5 seconds and verify no new blocks were produced
    Thread.sleep(5000)
    val heightAfterWait = currentBlockHeight(stack0)
    assertThat(heightAfterWait)
      .withFailMessage {
        "Expected no new blocks (height $heightAfterStop) but got height $heightAfterWait"
      }.isEqualTo(heightAfterStop)
    log.info("Confirmed: no blocks produced without quorum")

    // Restart validator 2 -- quorum restored (3 of 4)
    log.info("Restarting validator 2")
    restartValidator(stack2, maruFactory2, listOf(stack0.maruApp, stack1.maruApp))

    // Wait for all 3 active validators before reading their blocks.
    waitForBlockHeight(
      stack0.maruApp.beaconChain,
      stack1.maruApp.beaconChain,
      stack2.maruApp.beaconChain,
      targetHeight = heightAfterStop + STABLE_BLOCKS.toULong(),
      timeout = 90.seconds,
    )

    // Verify consistency across the 3 active validators
    val verifyStart = heightAfterStop + 1uL
    val count = STABLE_BLOCKS.toULong()
    checkAllValidatorBlocksAreTheSame(
      validatorBlocks = listOf(
        { stack0.maruApp.beaconChain.getSealedBeaconBlocks(verifyStart, count) },
        { stack1.maruApp.beaconChain.getSealedBeaconBlocks(verifyStart, count) },
        { stack2.maruApp.beaconChain.getSealedBeaconBlocks(verifyStart, count) },
      ),
      blocksToMetadata = ::clBlocksToMetadata,
    )
    log.info("Block production recovered after quorum was restored")
  }

  @Test
  fun `block production resumes after all 4 nodes restart`() {
    startAllValidators()

    // Wait for STABLE_BLOCKS blocks before recording the checkpoint (stack0 only; all are synced).
    waitForBlockHeight(stack0.maruApp.beaconChain, targetHeight = STABLE_BLOCKS.toULong(), timeout = 90.seconds)
    val heightBeforeRestart = currentBlockHeight(stack0)
    log.info("Height before full restart: $heightBeforeRestart")

    // Stop all 4 validators
    log.info("Stopping all 4 validators")
    stopValidator(stack0)
    stopValidator(stack1)
    stopValidator(stack2)
    stopValidator(stack3)

    Thread.sleep(2000)

    // Restart all 4 and re-establish full mesh
    log.info("Restarting all 4 validators")
    startAllValidators()

    // Wait for ALL 4 validators to reach the target before reading their blocks.
    waitForBlockHeight(
      stack0.maruApp.beaconChain,
      stack1.maruApp.beaconChain,
      stack2.maruApp.beaconChain,
      stack3.maruApp.beaconChain,
      targetHeight = heightBeforeRestart + STABLE_BLOCKS.toULong(),
      timeout = 90.seconds,
    )

    // Verify consistency across all 4 validators
    val verifyStart = heightBeforeRestart + 1uL
    val count = STABLE_BLOCKS.toULong()
    checkAllValidatorBlocksAreTheSame(
      validatorBlocks = listOf(
        { stack0.maruApp.beaconChain.getSealedBeaconBlocks(verifyStart, count) },
        { stack1.maruApp.beaconChain.getSealedBeaconBlocks(verifyStart, count) },
        { stack2.maruApp.beaconChain.getSealedBeaconBlocks(verifyStart, count) },
        { stack3.maruApp.beaconChain.getSealedBeaconBlocks(verifyStart, count) },
      ),
      blocksToMetadata = ::clBlocksToMetadata,
    )
    log.info("Block production resumed successfully after full restart")
  }
}

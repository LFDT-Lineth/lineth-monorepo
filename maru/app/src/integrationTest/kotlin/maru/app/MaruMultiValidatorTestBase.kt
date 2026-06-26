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
import testutils.PeeringNodeNetworkStack
import testutils.maru.MaruFactory
import testutils.maru.awaitTillMaruHasPeers
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

/**
 * Shared 4-validator QBFT fixture + helpers. The scenarios are split across concrete subclasses
 * (convergence/liveness vs. restart-recovery) so they run in parallel Gradle forks instead of
 * serially in one ~9-min fork. Each scenario still builds its own independent cluster in
 * [setUp]/[startAllValidators], so the split is purely fork-level parallelism, not shared state.
 *
 * Deliberately kept to a 2-way split rather than one-class-per-scenario: each scenario runs four
 * in-process validators on 1 s slot timers, and too many heavy consensus forks in parallel can
 * starve those timers under CFS throttling and push convergence toward the 240 s timeout.
 */
abstract class MaruMultiValidatorTestBase {
  companion object {
    /** Number of consecutive round-0 blocks required to declare convergence / stable production. */
    const val STABLE_BLOCKS = 5
  }

  private val multiValidatorSyncingConfig = MaruFactory.defaultValidatorSyncingConfig

  private val key0 = "080212201dd171cec7e2995408b5513004e8207fe88d6820aeff0d82463b3e41df251aae".fromHex()
  private val key1 = "0802122100abb81ba53518eb0a206dfe80f2a973182e5d66c98cd31d00bf7471fcd5514157".fromHex()
  private val key2 = "080212202fec0750fe3edc7e8272d4814a36b632921fc5e835d20a2de874471e8ad9ad0b".fromHex()
  private val key3 = "080212207a19c01ce2246b94b48ed778d9bfb3b76eaabe8193c468d6751f4e4d1adf98a8".fromHex()

  protected lateinit var cluster: Cluster
  protected lateinit var stack0: PeeringNodeNetworkStack
  protected lateinit var stack1: PeeringNodeNetworkStack
  protected lateinit var stack2: PeeringNodeNetworkStack
  protected lateinit var stack3: PeeringNodeNetworkStack

  protected val log = LogManager.getLogger(this.javaClass)

  protected val maruFactory0 = MaruFactory(validatorPrivateKey = key0)
  protected val maruFactory1 = MaruFactory(validatorPrivateKey = key1)
  protected val maruFactory2 = MaruFactory(validatorPrivateKey = key2)
  protected val maruFactory3 = MaruFactory(validatorPrivateKey = key3)

  protected val initialValidators: Set<Validator> by lazy {
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

  protected fun startAllValidators() {
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

  protected fun waitForBlockHeight(
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
  protected fun waitForConsecutiveRound0Blocks(
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

  protected fun currentBlockHeight(stack: PeeringNodeNetworkStack): ULong =
    stack.maruApp.beaconChain
      .getLatestBeaconState()
      .beaconBlockHeader.number

  protected fun stopValidator(stack: PeeringNodeNetworkStack) {
    stack.maruApp.stop().get()
    stack.maruApp.close()
  }

  protected fun restartValidator(
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

  protected fun <T, M> checkAllValidatorBlocksAreTheSame(
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

  protected fun checkBlockProposersMatchExpectedProposers(
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

  protected fun clBlocksToMetadata(blocks: List<SealedBeaconBlock>): List<Pair<ULong, String>> =
    blocks.map {
      it.beaconBlock.beaconBlockHeader.number to
        it.beaconBlock.beaconBlockHeader.hash
          .encodeHex()
    }
}

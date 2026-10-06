/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package testutils

import io.libp2p.etc.types.fromHex
import linea.kotlin.encodeHex
import linea.testing.besu.BesuFactory
import maru.app.MaruApp
import maru.consensus.ClFork
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
import testutils.maru.MaruFactory
import testutils.maru.awaitTillMaruHasPeers
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

/**
 * Four Maru QBFT validators, each backed by its own Besu node, wired in a full P2P mesh.
 *
 * Shared by the MaruMultiValidator*Test classes, which are split by scenario so that Gradle runs them in parallel
 * forks.
 */
class MultiValidatorNetwork {
  companion object {
    private val multiValidatorSyncingConfig = MaruFactory.defaultValidatorSyncingConfig

    private val validatorKeys =
      listOf(
        "080212201dd171cec7e2995408b5513004e8207fe88d6820aeff0d82463b3e41df251aae".fromHex(),
        "0802122100abb81ba53518eb0a206dfe80f2a973182e5d66c98cd31d00bf7471fcd5514157".fromHex(),
        "080212202fec0750fe3edc7e8272d4814a36b632921fc5e835d20a2de874471e8ad9ad0b".fromHex(),
        "080212207a19c01ce2246b94b48ed778d9bfb3b76eaabe8193c468d6751f4e4d1adf98a8".fromHex(),
      )

    private fun peerAddr(app: MaruApp) = "/ip4/127.0.0.1/tcp/${app.p2pPort()}/p2p/${app.p2pNetwork.nodeId}"
  }

  private val log = LogManager.getLogger(this.javaClass)

  private val cluster =
    Cluster(
      ClusterConfigurationBuilder().build(),
      NetConditions(NetTransactions()),
      ThreadBesuNodeRunner(),
    )

  val stacks: List<PeeringNodeNetworkStack> =
    List(validatorKeys.size) { PeeringNodeNetworkStack { BesuFactory.buildTestBesu(validator = false) } }

  private val defaultFactories = factoriesWithClFork(ClFork.QBFT_PHASE0)

  private val initialValidators: Set<Validator> by lazy {
    validatorKeys
      .map { SecpCrypto.privateKeyToValidator(SecpCrypto.privateKeyBytesWithoutPrefix(it)) }
      .toSet()
  }

  fun factoriesWithClFork(clFork: ClFork): List<MaruFactory> =
    validatorKeys.map { MaruFactory(validatorPrivateKey = it, clFork = clFork) }

  fun beaconChain(validatorIndex: Int): BeaconChain = stacks[validatorIndex].maruApp.beaconChain

  fun startBesuNodes() {
    PeeringNodeNetworkStack.startBesuNodes(cluster, *stacks.toTypedArray())
  }

  fun close() {
    stacks.reversed().forEach { runCatching { it.maruApp.stop().get() } }
    stacks.reversed().forEach { runCatching { it.maruApp.close() } }
    cluster.close()
  }

  fun startAllValidators(factories: List<MaruFactory> = defaultFactories) {
    val apps =
      stacks.zip(factories).map { (stack, factory) ->
        val app = buildValidator(stack, factory)
        stack.setMaruApp(app)
        app.start().get()
        app
      }

    // Wire full mesh: every validator dials all validators started before it
    apps.forEachIndexed { index, app ->
      apps.take(index).forEach { peer -> app.p2pNetwork.addPeer(peerAddr(peer)) }
    }

    // Wait for full mesh -- each validator should see all the others
    val expectedPeers = (apps.size - 1).toUInt()
    apps.forEachIndexed { index, app ->
      app.awaitTillMaruHasPeers(expectedPeers, pollingInterval = 500.milliseconds)
      log.info("Validator $index has $expectedPeers peers (height=${currentBlockHeight(index)})")
    }
    log.info("All ${apps.size} validators peered in full mesh")
  }

  fun stopValidator(validatorIndex: Int) {
    val stack = stacks[validatorIndex]
    stack.maruApp.stop().get()
    stack.maruApp.close()
  }

  fun restartValidator(
    validatorIndex: Int,
    peerIndexes: List<Int>,
  ) {
    val stack = stacks[validatorIndex]
    val app = buildValidator(stack, defaultFactories[validatorIndex])
    stack.setMaruApp(app)
    app.start().get()

    peerIndexes.forEach { peerIndex ->
      app.p2pNetwork.addPeer(peerAddr(stacks[peerIndex].maruApp))
    }
    app.awaitTillMaruHasPeers(peerIndexes.size.toUInt(), pollingInterval = 500.milliseconds)
  }

  fun currentBlockHeight(validatorIndex: Int): ULong =
    beaconChain(validatorIndex)
      .getLatestBeaconState()
      .beaconBlockHeader.number

  fun waitForBlockHeight(
    validatorIndexes: IntRange,
    targetHeight: ULong,
    timeout: Duration = 240.seconds,
  ) {
    await
      .timeout(timeout.toJavaDuration())
      .pollInterval(500.milliseconds.toJavaDuration())
      .untilAsserted {
        validatorIndexes.forEach { idx ->
          assertThat(currentBlockHeight(idx))
            .withFailMessage { "Validator $idx has not reached block $targetHeight yet" }
            .isGreaterThanOrEqualTo(targetHeight)
        }
      }
  }

  /**
   * Polls validator [validatorIndex] until [requiredConsecutive] consecutive round-0 blocks have been committed.
   * Returns the block number of the last block in the first qualifying run.
   *
   * This is the proper way to detect QBFT convergence: during startup, validators run independently
   * before the P2P mesh is wired, causing round skips. Checking a fixed block number is unreliable;
   * instead we wait for a stable run of round-0 blocks.
   */
  fun waitForConsecutiveRound0Blocks(
    validatorIndex: Int,
    requiredConsecutive: Int,
    timeout: Duration = 240.seconds,
  ): ULong {
    val beaconChain = beaconChain(validatorIndex)
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

  fun checkValidatorsHaveSameBlocks(
    validatorIndexes: IntRange,
    startBlock: ULong,
    count: ULong,
  ) {
    val allMetadata =
      validatorIndexes.map { idx -> clBlocksToMetadata(beaconChain(idx).getSealedBeaconBlocks(startBlock, count)) }
    for (i in 1 until allMetadata.size) {
      assertThat(allMetadata[i])
        .withFailMessage { "Validator ${validatorIndexes.elementAt(i)} blocks differ from validator 0" }
        .isEqualTo(allMetadata[0])
    }
  }

  fun checkBlockProposersMatchExpectedProposers(
    validatorIndex: Int,
    startBlock: ULong,
    endBlock: ULong,
  ) {
    val beaconChain = beaconChain(validatorIndex)
    val count = endBlock - startBlock + 1uL
    val blocks = beaconChain.getSealedBeaconBlocks(startBlock, count)

    blocks.forEach { block ->
      val beaconBlockHeader = block.beaconBlock.beaconBlockHeader
      val roundIdentifier =
        ConsensusRoundIdentifier(beaconBlockHeader.number.toLong(), beaconBlockHeader.round.toInt())
      val parentBeaconState = beaconChain.getBeaconState(beaconBlockHeader.number - 1uL)
      val expectedProposer = ProposerSelectorImpl.getProposerForBlock(parentBeaconState!!, roundIdentifier).get()

      assertThat(beaconBlockHeader.proposer)
        .withFailMessage {
          "Block ${beaconBlockHeader.number} should be proposed by ${expectedProposer.address.encodeHex()} " +
            "but was proposed by ${beaconBlockHeader.proposer.address.encodeHex()}"
        }.isEqualTo(expectedProposer)
    }
  }

  private fun buildValidator(
    stack: PeeringNodeNetworkStack,
    factory: MaruFactory,
  ): MaruApp =
    factory.buildTestMaruValidatorWithP2pPeering(
      ethereumJsonRpcUrl = stack.besuNode.jsonRpcBaseUrl().get(),
      engineApiRpc = stack.besuNode.engineRpcUrl().get(),
      dataDir = stack.tmpDir,
      syncingConfig = multiValidatorSyncingConfig,
      allowEmptyBlocks = true,
      initialValidators = initialValidators,
    )

  private fun clBlocksToMetadata(blocks: List<SealedBeaconBlock>): List<Pair<ULong, String>> =
    blocks.map {
      it.beaconBlock.beaconBlockHeader.number to
        it.beaconBlock.beaconBlockHeader.beaconBlockIdHash
          .encodeHex()
    }
}

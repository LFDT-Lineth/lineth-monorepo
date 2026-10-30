/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package testutils

import io.libp2p.etc.types.fromHex
import linea.kotlin.encodeHex
import maru.app.MaruApp
import maru.consensus.ChainFork
import maru.consensus.ClFork
import maru.consensus.ElFork
import maru.consensus.qbft.ProposerSelectorImpl
import maru.core.SealedBeaconBlock
import maru.database.BeaconChain
import maru.test.cluster.MaruCluster
import maru.test.cluster.NodeBuilder
import maru.test.cluster.NodeRole
import maru.test.extensions.TestSyncingConfigs
import maru.test.extensions.withFastPeering
import maru.test.extensions.withFastQbft
import maru.test.extensions.withStaticPeeringOnly
import maru.test.extensions.withSyncingConfig
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.kotlin.await
import org.hyperledger.besu.consensus.common.bft.ConsensusRoundIdentifier
import testutils.maru.awaitTillMaruHasPeers
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant
import kotlin.time.toJavaDuration

/**
 * Four Maru QBFT validators, each backed by its own Besu node, wired in a full P2P mesh.
 *
 * Shared by the MaruMultiValidator*Test classes, which are split by scenario so that Gradle runs them in parallel
 * forks.
 */
class MultiValidatorNetwork {
  companion object {
    private const val CHAIN_ID = 1337u
    private const val BLOCK_TIME_SECONDS = 1u

    private val validatorKeys =
      listOf(
        "080212201dd171cec7e2995408b5513004e8207fe88d6820aeff0d82463b3e41df251aae".fromHex(),
        "0802122100abb81ba53518eb0a206dfe80f2a973182e5d66c98cd31d00bf7471fcd5514157".fromHex(),
        "080212202fec0750fe3edc7e8272d4814a36b632921fc5e835d20a2de874471e8ad9ad0b".fromHex(),
        "080212207a19c01ce2246b94b48ed778d9bfb3b76eaabe8193c468d6751f4e4d1adf98a8".fromHex(),
      )

    private fun label(validatorIndex: Int) = "validator-$validatorIndex"
  }

  private val log = LogManager.getLogger(this.javaClass)

  private lateinit var cluster: MaruCluster

  fun beaconChain(validatorIndex: Int): BeaconChain = maruApp(validatorIndex).beaconChain

  fun close() {
    if (::cluster.isInitialized) {
      cluster.stop()
    }
  }

  /**
   * Starts all validators with a genesis fork schedule on [clFork]. Every validator statically peers with all
   * validators started before it, which wires a full mesh.
   */
  fun startAllValidators(clFork: ClFork = ClFork.QBFT_PHASE0) {
    val chainForks = mapOf(Instant.fromEpochSeconds(0) to ChainFork(clFork, ElFork.Prague))
    cluster = MaruCluster(chainId = CHAIN_ID, blockTimeSeconds = BLOCK_TIME_SECONDS, chainForks = chainForks)
    validatorKeys.indices.forEach { index ->
      cluster.addNode(label(index), addBesu = true) { nodeBuilder ->
        configureValidator(nodeBuilder, index)
      }
    }
    cluster.start()
    awaitFullMesh()
  }

  /** Restarts all validators stopped with [stopValidator], reusing their data directories and Besu nodes */
  fun restartAllValidators() {
    validatorKeys.indices.forEach { cluster.startMaruNode(label(it)) }
    awaitFullMesh()
  }

  fun stopValidator(validatorIndex: Int) {
    cluster.stopMaruNode(label(validatorIndex))
  }

  fun restartValidator(
    validatorIndex: Int,
    peerIndexes: List<Int>,
  ) {
    val app = cluster.startMaruNode(label(validatorIndex))

    peerIndexes.forEach { peerIndex ->
      app.p2pNetwork.addPeer(cluster.nodeAddr(label(peerIndex)))
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

  private fun maruApp(validatorIndex: Int): MaruApp = cluster.maruNode(label(validatorIndex))

  private fun configureValidator(
    nodeBuilder: NodeBuilder,
    validatorIndex: Int,
  ) {
    nodeBuilder
      .withRole(NodeRole.Sequencer)
      .withNodeKey(validatorKeys[validatorIndex])
      // otherwise the 4 Besu nodes would share the default signer key, hence the same enode
      .besuValidator(false)
      .staticPeers((0 until validatorIndex).map(::label))
      .withSyncingConfig(TestSyncingConfigs.validator)
      .withFastPeering()
      .withFastQbft()
      // discovery is not needed for a full mesh
      .withStaticPeeringOnly()
  }

  private fun awaitFullMesh() {
    // Wait for full mesh -- each validator should see all the others
    val expectedPeers = (validatorKeys.size - 1).toUInt()
    validatorKeys.indices.forEach { index ->
      maruApp(index).awaitTillMaruHasPeers(expectedPeers, pollingInterval = 500.milliseconds)
      log.info("Validator $index has $expectedPeers peers (height=${currentBlockHeight(index)})")
    }
    log.info("All ${validatorKeys.size} validators peered in full mesh")
  }

  private fun clBlocksToMetadata(blocks: List<SealedBeaconBlock>): List<Pair<ULong, String>> =
    blocks.map {
      it.beaconBlock.beaconBlockHeader.number to
        it.beaconBlock.beaconBlockHeader.beaconBlockIdHash
          .encodeHex()
    }
}

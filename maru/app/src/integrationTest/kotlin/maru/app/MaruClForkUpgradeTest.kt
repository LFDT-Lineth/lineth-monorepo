/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import linea.kotlin.encodeHex
import linea.testing.besu.BesuTransactionsHelper
import maru.consensus.ChainFork
import maru.consensus.ClFork
import maru.consensus.ElFork
import maru.database.BeaconChain
import maru.serialization.rlp.HashUtil
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
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant
import kotlin.time.toJavaDuration

/**
 * Proves a live QBFT_PHASE0 network -- with a validator and a peered follower -- can upgrade to QBFT_PHASE1
 * at a scheduled timestamp without losing agreement on the chain, and that the chain-identity hashing
 * algorithm actually switches from round-inclusive to round-independent at the boundary.
 */
class MaruClForkUpgradeTest {
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

  private fun waitForBlockHeight(
    beaconChain: BeaconChain,
    targetHeight: ULong,
    timeout: kotlin.time.Duration = 90.seconds,
  ) {
    await
      .timeout(timeout.toJavaDuration())
      .pollInterval(500.milliseconds.toJavaDuration())
      .untilAsserted {
        assertThat(beaconChain.getLatestBeaconState().beaconBlockHeader.number)
          .isGreaterThanOrEqualTo(targetHeight)
      }
  }

  @Test
  fun `network upgrades live from QBFT_PHASE0 to QBFT_PHASE1 without losing chain agreement`() {
    val clusterStartupMargin = 40UL
    val blocksBeforeSwitch = 5UL
    val currentTimestamp = (System.currentTimeMillis() / 1000).toULong()
    val phase1ActivationTimestamp = currentTimestamp + clusterStartupMargin + blocksBeforeSwitch
    val postSwitchBuffer = 8UL

    cluster = MaruCluster(
      chainForks = mapOf(
        Instant.fromEpochSeconds(0) to ChainFork(ClFork.QBFT_PHASE0, ElFork.Prague),
        Instant.fromEpochSeconds(phase1ActivationTimestamp.toLong()) to ChainFork(ClFork.QBFT_PHASE1, ElFork.Prague),
      ),
    ).addNode(NodeRole.Sequencer, withBesuEl = true)
      .addNode(NodeRole.Follower, withBesuEl = true) {
        it.staticPeers(listOf("sequencer"))
      }.start()

    val validatorMaruApp = cluster.maruNode("sequencer")
    val followerMaruApp = cluster.maruNode("follower")

    await
      .timeout(60.seconds.toJavaDuration())
      .pollInterval(500.milliseconds.toJavaDuration())
      .untilAsserted {
        assertThat(validatorMaruApp.p2pNetwork.peerCount).isGreaterThanOrEqualTo(1)
        assertThat(followerMaruApp.p2pNetwork.peerCount).isGreaterThanOrEqualTo(1)
      }
    log.info("Validator and follower peered, PHASE1 activates at $phase1ActivationTimestamp")

    // Drive one block roughly per second from now until well past the activation timestamp, so the chain
    // organically crosses the QBFT_PHASE0 -> QBFT_PHASE1 boundary while the test is running.
    val totalBlocksToProduce = (phase1ActivationTimestamp + postSwitchBuffer - currentTimestamp).toInt()
    repeat(totalBlocksToProduce) {
      transactionsHelper.run {
        cluster.besuNode("sequencer").sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("phase-upgrade account"),
          amount = Amount.ether(100),
        )
      }
    }

    waitForBlockHeight(validatorMaruApp.beaconChain, targetHeight = totalBlocksToProduce.toULong())
    waitForBlockHeight(followerMaruApp.beaconChain, targetHeight = totalBlocksToProduce.toULong())

    val validatorBlocks =
      validatorMaruApp.beaconChain.getSealedBeaconBlocks(1UL, totalBlocksToProduce.toULong())
    val followerBlocks =
      followerMaruApp.beaconChain.getSealedBeaconBlocks(1UL, totalBlocksToProduce.toULong())

    // The network never forked: validator and follower agree on every block's chain identity, both
    // before and after the live upgrade.
    assertThat(followerBlocks.map { it.beaconBlock.beaconBlockHeader.beaconBlockIdHash.encodeHex() })
      .withFailMessage { "Follower diverged from validator across the PHASE0 -> PHASE1 upgrade" }
      .isEqualTo(validatorBlocks.map { it.beaconBlock.beaconBlockHeader.beaconBlockIdHash.encodeHex() })

    val allHeaders = validatorBlocks.map { it.beaconBlock.beaconBlockHeader }
    val preSwitchHeaders = allHeaders.filter { it.timestamp < phase1ActivationTimestamp }
    val postSwitchHeaders = allHeaders.filter { it.timestamp >= phase1ActivationTimestamp }

    assertThat(preSwitchHeaders)
      .withFailMessage { "Expected at least one block before the PHASE1 activation" }
      .isNotEmpty()
    assertThat(postSwitchHeaders)
      .withFailMessage { "Expected at least one block after the PHASE1 activation" }
      .isNotEmpty()

    // The hashing algorithm itself switched at the boundary: round-inclusive identity before, round
    // -independent identity from PHASE1 onward. This is the actual live proof of the upgrade, on top of
    // the plain block-agreement check above.
    preSwitchHeaders.forEach { header ->
      assertThat(header.beaconBlockIdHash)
        .withFailMessage { "Block ${header.number} before the switch should use the round-inclusive identity hash" }
        .isEqualTo(HashUtil.headerHash(header))
    }
    postSwitchHeaders.forEach { header ->
      assertThat(header.beaconBlockIdHash)
        .withFailMessage { "Block ${header.number} after the switch should use the round-independent identity hash" }
        .isEqualTo(HashUtil.roundIndependentHeaderHash(header))
    }

    log.info(
      "Confirmed live QBFT_PHASE0 -> QBFT_PHASE1 upgrade: ${preSwitchHeaders.size} PHASE0 blocks, " +
        "${postSwitchHeaders.size} PHASE1 blocks, validator and follower agree throughout",
    )
  }
}

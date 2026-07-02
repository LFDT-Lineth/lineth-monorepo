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
import linea.testing.besu.BesuFactory
import linea.testing.besu.BesuTransactionsHelper
import maru.database.BeaconChain
import maru.serialization.rlp.HashUtil
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.kotlin.await
import org.hyperledger.besu.tests.acceptance.dsl.blockchain.Amount
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
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

/**
 * Proves a live QBFT_PHASE0 network -- with a validator and a peered follower -- can upgrade to QBFT_PHASE1
 * at a scheduled timestamp without losing agreement on the chain, and that the chain-identity hashing
 * algorithm actually switches from round-inclusive to round-independent at the boundary.
 */
class MaruClForkUpgradeTest {
  private lateinit var cluster: Cluster
  private lateinit var validatorStack: PeeringNodeNetworkStack
  private lateinit var followerStack: PeeringNodeNetworkStack
  private lateinit var transactionsHelper: BesuTransactionsHelper
  private val log = LogManager.getLogger(this.javaClass)

  @BeforeEach
  fun setUp() {
    transactionsHelper = BesuTransactionsHelper()
    cluster = Cluster(
      ClusterConfigurationBuilder().build(),
      NetConditions(NetTransactions()),
      ThreadBesuNodeRunner(),
    )
    validatorStack = PeeringNodeNetworkStack()
    followerStack = PeeringNodeNetworkStack(besuBuilder = { BesuFactory.buildTestBesu(validator = false) })
    PeeringNodeNetworkStack.startBesuNodes(cluster, validatorStack, followerStack)
  }

  @AfterEach
  fun tearDown() {
    runCatching { followerStack.maruApp.stop().get() }
    runCatching { validatorStack.maruApp.stop().get() }
    runCatching { followerStack.maruApp.close() }
    runCatching { validatorStack.maruApp.close() }
    cluster.close()
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
    val stackStartupMargin = 40UL
    val blocksBeforeSwitch = 5UL
    val currentTimestamp = (System.currentTimeMillis() / 1000).toULong()
    val phase1ActivationTimestamp = currentTimestamp + stackStartupMargin + blocksBeforeSwitch
    val postSwitchBuffer = 8UL

    val maruFactory = MaruFactory(qbftPhase1ActivationTimestamp = phase1ActivationTimestamp)

    val validatorMaruApp =
      maruFactory.buildTestMaruValidatorWithP2pPeering(
        ethereumJsonRpcUrl = validatorStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = validatorStack.besuNode.engineRpcUrl().get(),
        dataDir = validatorStack.tmpDir,
      )
    validatorStack.setMaruApp(validatorMaruApp)
    validatorMaruApp.start().get()

    val followerMaruApp =
      maruFactory.buildTestMaruFollowerWithP2pPeering(
        ethereumJsonRpcUrl = followerStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = followerStack.besuNode.engineRpcUrl().get(),
        dataDir = followerStack.tmpDir,
        validatorPortForStaticPeering = validatorStack.p2pPort,
      )
    followerStack.setMaruApp(followerMaruApp)
    followerMaruApp.start().get()

    followerStack.maruApp.awaitTillMaruHasPeers(1u)
    validatorStack.maruApp.awaitTillMaruHasPeers(1u)
    log.info("Validator and follower peered, PHASE1 activates at $phase1ActivationTimestamp")

    // Drive one block roughly per second from now until well past the activation timestamp, so the chain
    // organically crosses the QBFT_PHASE0 -> QBFT_PHASE1 boundary while the test is running.
    val totalBlocksToProduce = (phase1ActivationTimestamp + postSwitchBuffer - currentTimestamp).toInt()
    repeat(totalBlocksToProduce) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("phase-upgrade account"),
          amount = Amount.ether(100),
        )
      }
    }

    waitForBlockHeight(validatorStack.maruApp.beaconChain, targetHeight = totalBlocksToProduce.toULong())
    waitForBlockHeight(followerStack.maruApp.beaconChain, targetHeight = totalBlocksToProduce.toULong())

    val validatorBlocks =
      validatorStack.maruApp.beaconChain.getSealedBeaconBlocks(1UL, totalBlocksToProduce.toULong())
    val followerBlocks =
      followerStack.maruApp.beaconChain.getSealedBeaconBlocks(1UL, totalBlocksToProduce.toULong())

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

/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package testutils

import linea.testing.besu.BesuTransactionsHelper
import maru.app.MaruApp
import maru.config.SyncingConfig
import maru.test.cluster.MaruCluster
import maru.test.cluster.NodeBuilder
import maru.test.cluster.NodeRole
import maru.test.cluster.assertSameCanonicalElChain
import maru.test.extensions.TestSyncingConfigs
import maru.test.extensions.withFastPeering
import maru.test.extensions.withFastQbft
import maru.test.extensions.withStaticPeeringOnly
import maru.test.extensions.withSyncingConfig
import org.apache.logging.log4j.LogManager
import org.hyperledger.besu.tests.acceptance.dsl.blockchain.Amount
import org.hyperledger.besu.tests.acceptance.dsl.node.BesuNode
import testutils.Checks.checkAllNodesHaveSameBlocks
import testutils.maru.awaitTillMaruHasPeers
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

/**
 * One Maru validator and one Maru follower, each backed by its own Besu node, peered over P2P.
 *
 * Shared by the MaruFollower*Test classes, which are split by scenario so that Gradle runs them in parallel forks.
 */
class ValidatorFollowerNetwork {
  companion object {
    const val VALIDATOR = "validator"
    const val FOLLOWER = "follower"
  }

  private val log = LogManager.getLogger(this.javaClass)
  private val transactionsHelper = BesuTransactionsHelper()
  private val cluster = MaruCluster()

  val validatorBesu: BesuNode
    get() = cluster.besuNode(VALIDATOR)
  val followerBesu: BesuNode
    get() = cluster.besuNode(FOLLOWER)
  val validatorMaru: MaruApp
    get() = cluster.maruNode(VALIDATOR)
  val followerMaru: MaruApp
    get() = cluster.maruNode(FOLLOWER)

  fun close() {
    cluster.stop()
  }

  /** Starts the validator and follower, each with its own Besu, and waits until they are peered */
  fun start(
    syncingConfig: SyncingConfig = TestSyncingConfigs.default,
    payloadValidationEnabled: Boolean = true,
    validatorP2pPort: UInt = 0u,
  ) {
    cluster
      .addNode(VALIDATOR, addBesu = true) { nodeBuilder ->
        nodeBuilder
          .withRole(NodeRole.Sequencer)
          .withNetworkDefaults(syncingConfig)
          .withFastQbft()
          .maruConfig { it.copy(p2p = it.p2p!!.copy(port = validatorP2pPort)) }
      }.addNode(FOLLOWER, addBesu = true) { nodeBuilder ->
        nodeBuilder
          .withRole(NodeRole.Follower)
          .withNetworkDefaults(syncingConfig)
          .staticPeers(listOf(VALIDATOR))
          .payloadValidation(payloadValidationEnabled)
      }.start()

    log.info("Nodes are peered")
    followerMaru.awaitTillMaruHasPeers(1u)
    validatorMaru.awaitTillMaruHasPeers(1u)
    cluster.assertSameCanonicalElChain(chainHeight = 0UL)
  }

  fun stopFollowerMaru() {
    cluster.stopMaruNode(FOLLOWER)
  }

  fun startFollowerMaru() {
    cluster.startMaruNode(FOLLOWER)
  }

  /** Restarts the validator Maru with the validator syncing config, keeping its node key, data directory and P2P port */
  fun restartValidatorMaru() {
    cluster.restartMaruNode(VALIDATOR) { it.copy(syncing = TestSyncingConfigs.validator) }
  }

  fun stopBesuNodes() {
    cluster.besuCluster.stop()
  }

  /** Starts again the Besu nodes stopped with [stopBesuNodes], retrying as their ports may not be released yet */
  fun startBesuNodes() {
    val besuNodes = listOf(followerBesu, validatorBesu)
    val maxAttempts = 10
    var lastException: IllegalStateException? = null
    repeat(maxAttempts) { attempt ->
      try {
        besuNodes.forEach(cluster.besuCluster::addNode)
        cluster.besuCluster.start(awaitPeerDiscovery = false)
        return
      } catch (e: IllegalStateException) {
        lastException = e
        cluster.besuCluster.stop()
        if (attempt < maxAttempts - 1) {
          Thread.sleep(1000)
        }
      }
    }
    throw lastException ?: IllegalStateException("Failed to start Besu nodes after $maxAttempts attempts")
  }

  fun produceBlocks(blockCount: Int) {
    repeat(blockCount) {
      transactionsHelper.run {
        validatorBesu.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }
  }

  fun checkValidatorAndFollowerBlocks(
    blocksToProduce: Int,
    timeout: Duration = 30.seconds,
  ) {
    checkAllNodesHaveSameBlocks(
      expectedBlockCount = blocksToProduce,
      validatorBesu,
      followerBesu,
      timeout = timeout,
    )
  }

  private fun NodeBuilder.withNetworkDefaults(syncingConfig: SyncingConfig): NodeBuilder =
    withSyncingConfig(syncingConfig)
      .withFastPeering()
      .withStaticPeeringOnly()
      // blocks are produced only when there are transactions, as these tests count blocks
      .maruConfig { it.copy(allowEmptyBlocks = false) }
}

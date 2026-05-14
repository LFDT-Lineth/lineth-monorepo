/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import maru.config.SyncingConfig
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.tests.acceptance.dsl.blockchain.Amount
import org.hyperledger.besu.tests.acceptance.dsl.condition.net.NetConditions
import org.hyperledger.besu.tests.acceptance.dsl.node.ThreadBesuNodeRunner
import org.hyperledger.besu.tests.acceptance.dsl.node.cluster.Cluster
import org.hyperledger.besu.tests.acceptance.dsl.node.cluster.ClusterConfigurationBuilder
import org.hyperledger.besu.tests.acceptance.dsl.transaction.net.NetTransactions
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.MethodSource
import testutils.Checks.checkAllNodesHaveSameBlocks
import testutils.Checks.getBlockNumber
import testutils.PeeringNodeNetworkStack
import testutils.besu.BesuFactory
import testutils.besu.BesuTransactionsHelper
import testutils.besu.ethGetBlockByNumber
import testutils.besu.startWithRetry
import testutils.maru.MaruFactory
import testutils.maru.awaitTillMaruHasPeers
import kotlin.collections.map
import kotlin.time.Duration.Companion.seconds

class MaruFollowerTest {
  companion object {
    @JvmStatic
    fun enumeratingSyncingConfigs(): List<SyncingConfig> = MaruFactory.enumeratingSyncingConfigs()
  }

  private lateinit var cluster: Cluster
  private lateinit var validatorStack: PeeringNodeNetworkStack
  private lateinit var followerStack: PeeringNodeNetworkStack
  private lateinit var transactionsHelper: BesuTransactionsHelper
  private val log = LogManager.getLogger(this.javaClass)
  private val maruFactory = MaruFactory()

  private fun setupMaruHelper(
    syncingConfig: SyncingConfig = MaruFactory.defaultSyncingConfig,
    payloadValidationEnabled: Boolean = true,
  ) {
    // Create and start validator Maru app first
    val validatorMaruApp =
      maruFactory.buildTestMaruValidatorWithP2pPeering(
        ethereumJsonRpcUrl = validatorStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = validatorStack.besuNode.engineRpcUrl().get(),
        dataDir = validatorStack.tmpDir,
        syncingConfig = syncingConfig,
      )
    validatorStack.setMaruApp(validatorMaruApp)
    validatorStack.maruApp.start().get()

    // Get the validator's p2p port after it's started
    val validatorP2pPort = validatorStack.p2pPort

    // Create follower Maru app with the validator's p2p port for static peering
    val followerMaruApp =
      maruFactory.buildTestMaruFollowerWithP2pPeering(
        ethereumJsonRpcUrl = followerStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = followerStack.besuNode.engineRpcUrl().get(),
        dataDir = followerStack.tmpDir,
        validatorPortForStaticPeering = validatorP2pPort,
        syncingConfig = syncingConfig,
        enablePayloadValidation = payloadValidationEnabled,
      )
    followerStack.setMaruApp(followerMaruApp)
    followerStack.maruApp.start().get()

    log.info("Nodes are peered")
    followerStack.maruApp.awaitTillMaruHasPeers(1u)
    validatorStack.maruApp.awaitTillMaruHasPeers(1u)
    val validatorGenesis = validatorStack.besuNode.ethGetBlockByNumber("earliest", false)
    val followerGenesis = followerStack.besuNode.ethGetBlockByNumber("earliest", false)

    assertThat(validatorGenesis).isEqualTo(followerGenesis)
  }

  @BeforeEach
  fun setUp() {
    transactionsHelper = BesuTransactionsHelper()
    cluster = Cluster(
      ClusterConfigurationBuilder().build(),
      NetConditions(NetTransactions()),
      ThreadBesuNodeRunner(),
    )

    validatorStack = PeeringNodeNetworkStack()

    followerStack = PeeringNodeNetworkStack(
      besuBuilder = { BesuFactory.buildTestBesu(validator = false) },
    )

    // Start all Besu nodes together for proper peering
    PeeringNodeNetworkStack.startBesuNodes(cluster, validatorStack, followerStack)
  }

  @AfterEach
  fun tearDown() {
    followerStack.maruApp.stop().get()
    validatorStack.maruApp.stop().get()
    followerStack.maruApp.close()
    validatorStack.maruApp.close()
    cluster.close()
  }

  @Test
  fun `Maru follower is able to import blocks`() {
    setupMaruHelper()

    val blocksToProduce = 5
    repeat(blocksToProduce) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    checkValidatorAndFollowerBlocks(blocksToProduce)
  }

  @Test
  fun `Maru follower is able to import blocks with payload validation disabled`() {
    setupMaruHelper(payloadValidationEnabled = false)

    val blocksToProduce = 5
    repeat(blocksToProduce) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    checkValidatorAndFollowerBlocks(blocksToProduce)
  }

  @Test
  fun `Maru follower is able to import blocks after going down`() {
    setupMaruHelper()

    val blocksToProduce = 5
    repeat(blocksToProduce) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    // This is here mainly to wait until block propagation is complete
    checkValidatorAndFollowerBlocks(blocksToProduce)

    followerStack.maruApp.stop().get()
    followerStack.maruApp.close()
    followerStack.setMaruApp(
      maruFactory.buildTestMaruFollowerWithP2pPeering(
        ethereumJsonRpcUrl = followerStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = followerStack.besuNode.engineRpcUrl().get(),
        dataDir = followerStack.tmpDir,
        validatorPortForStaticPeering = validatorStack.p2pPort,
      ),
    )
    followerStack.maruApp.start().get()

    followerStack.maruApp.awaitTillMaruHasPeers(1u)
    validatorStack.maruApp.awaitTillMaruHasPeers(1u)

    repeat(blocksToProduce) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    checkValidatorAndFollowerBlocks(blocksToProduce * 2)
  }

  @Test
  fun `Maru follower is able to import blocks after Validator stack goes down`() {
    setupMaruHelper()

    val blocksToProduce = 5
    repeat(blocksToProduce) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    val validatorP2pPort = validatorStack.p2pPort
    // This is here mainly to wait until block propagation is complete
    checkValidatorAndFollowerBlocks(blocksToProduce)

    validatorStack.maruApp.stop().get()
    validatorStack.maruApp.close()
    validatorStack.setMaruApp(
      maruFactory.buildTestMaruValidatorWithP2pPeering(
        ethereumJsonRpcUrl = validatorStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = validatorStack.besuNode.engineRpcUrl().get(),
        dataDir = validatorStack.tmpDir,
        p2pPort = validatorP2pPort,
      ),
    )
    validatorStack.maruApp.start().get()

    followerStack.maruApp.awaitTillMaruHasPeers(1u)
    validatorStack.maruApp.awaitTillMaruHasPeers(1u)

    repeat(blocksToProduce) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    checkValidatorAndFollowerBlocks(blocksToProduce * 2)
  }

  @Test
  fun `Maru follower is able to import blocks after its validator el node goes down`() {
    setupMaruHelper()

    val blocksToProduce = 5
    repeat(blocksToProduce) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    // This is here mainly to wait until block propagation is complete
    checkValidatorAndFollowerBlocks(blocksToProduce)

    cluster.stop()
    Thread.sleep(3000)
    cluster.startWithRetry(followerStack.besuNode, validatorStack.besuNode)

    repeat(blocksToProduce) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    checkValidatorAndFollowerBlocks(blocksToProduce * 2)
  }

  @ParameterizedTest
  @MethodSource("enumeratingSyncingConfigs")
  fun `Maru follower is able to complete initial syncing`(syncingConfig: SyncingConfig) {
    setupMaruHelper(syncingConfig)

    followerStack.maruApp.stop().get()
    followerStack.maruApp.close()

    val residueBlocks = 3 // residue of modulo peerChainHeightGranularity i.e. 10
    val blocksToProduceWithoutResidue = 20 // a block number dividable by 10
    val blocksTotal = residueBlocks + blocksToProduceWithoutResidue

    repeat(blocksTotal) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    // This is here mainly to wait until block propagation is complete
    checkNetworkStacksBlocksProduced(blocksTotal, validatorStack)

    followerStack.setMaruApp(
      maruFactory.buildTestMaruFollowerWithP2pPeering(
        ethereumJsonRpcUrl = followerStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = followerStack.besuNode.engineRpcUrl().get(),
        dataDir = followerStack.tmpDir,
        validatorPortForStaticPeering = validatorStack.p2pPort,
        syncingConfig = syncingConfig,
      ),
    )
    followerStack.maruApp.start().get()
    followerStack.maruApp.awaitTillMaruHasPeers(1u, pollingInterval = 1.seconds)

    when (syncingConfig.syncTargetSelection) {
      is SyncingConfig.SyncTargetSelection.Highest ->
        checkValidatorAndFollowerBlocks(blocksTotal)

      is SyncingConfig.SyncTargetSelection.MostFrequent -> {
        checkValidatorAndFollowerBlocks(blocksToProduceWithoutResidue)
        // ensure that the head of follower is blocksToProduceWithoutResidue
        assertThat(followerStack.besuNode.getBlockNumber()).isEqualTo(blocksToProduceWithoutResidue)
      }
    }
  }

  @ParameterizedTest
  @MethodSource("enumeratingSyncingConfigs")
  fun `Maru follower is able to complete syncing after restarted`(syncingConfig: SyncingConfig) {
    setupMaruHelper(syncingConfig)

    val residueBlocks = 3 // residue of modulo peerChainHeightGranularity i.e. 10
    val blocksToProduceWithoutResidue = 10 // a block number dividable by 10

    repeat(blocksToProduceWithoutResidue) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    // This is here mainly to wait until block propagation is complete
    checkValidatorAndFollowerBlocks(blocksToProduceWithoutResidue)

    followerStack.maruApp.stop().get()
    followerStack.maruApp.close()

    repeat(blocksToProduceWithoutResidue + residueBlocks) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }
    checkNetworkStacksBlocksProduced(2 * blocksToProduceWithoutResidue + residueBlocks, validatorStack)

    followerStack.setMaruApp(
      maruFactory.buildTestMaruFollowerWithP2pPeering(
        ethereumJsonRpcUrl = followerStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = followerStack.besuNode.engineRpcUrl().get(),
        dataDir = followerStack.tmpDir,
        validatorPortForStaticPeering = validatorStack.p2pPort,
        syncingConfig = syncingConfig,
      ),
    )
    followerStack.maruApp.start().get()
    followerStack.maruApp.awaitTillMaruHasPeers(1u, pollingInterval = 1.seconds)

    when (syncingConfig.syncTargetSelection) {
      is SyncingConfig.SyncTargetSelection.Highest ->
        checkValidatorAndFollowerBlocks(
          2 * blocksToProduceWithoutResidue + residueBlocks,
        )

      is SyncingConfig.SyncTargetSelection.MostFrequent -> {
        checkValidatorAndFollowerBlocks(2 * blocksToProduceWithoutResidue)
        // ensure that the head of follower is 2 * blocksToProduceWithoutResidue
        assertThat(followerStack.besuNode.getBlockNumber()).isEqualTo(2 * blocksToProduceWithoutResidue)
      }
    }
  }

  @ParameterizedTest
  @MethodSource("enumeratingSyncingConfigs")
  fun `Maru follower is able to complete syncing after disconnect peers`(syncingConfig: SyncingConfig) {
    setupMaruHelper(syncingConfig)

    val residueBlocks = 3 // residue of modulo peerChainHeightGranularity i.e. 10
    val blocksToProduce = 20 // a block number dividable by 10
    repeat(blocksToProduce) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    // This is here mainly to wait until block propagation is complete
    checkValidatorAndFollowerBlocks(blocksToProduce)

    val followerP2PNetwork = followerStack.maruApp.p2pNetwork
    val peers = followerP2PNetwork.getPeers()
    peers.forEach {
      followerP2PNetwork.dropPeer(it)
    }

    repeat(blocksToProduce + residueBlocks) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }
    checkNetworkStacksBlocksProduced(2 * blocksToProduce + residueBlocks, validatorStack)
    checkNetworkStacksBlocksProduced(blocksToProduce, followerStack)
    // ensure that the head of follower is at blocksToProduce
    assertThat(followerStack.besuNode.getBlockNumber()).isEqualTo(blocksToProduce)

    peers.forEach {
      followerP2PNetwork.addPeer("${it.address}/p2p/${it.nodeId}")
    }
    followerStack.maruApp.awaitTillMaruHasPeers(1u, pollingInterval = 1.seconds)
    when (syncingConfig.syncTargetSelection) {
      is SyncingConfig.SyncTargetSelection.Highest ->
        checkValidatorAndFollowerBlocks(
          blocksToProduce = 2 * blocksToProduce + residueBlocks,
          timeout = 60.seconds,
        )

      is SyncingConfig.SyncTargetSelection.MostFrequent -> {
        checkValidatorAndFollowerBlocks(2 * blocksToProduce)
        // ensure that the head of follower is at 2 * blocksToProduce
        assertThat(followerStack.besuNode.getBlockNumber()).isEqualTo(2 * blocksToProduce)
      }
    }
  }

  private fun checkValidatorAndFollowerBlocks(
    blocksToProduce: Int,
    timeout: kotlin.time.Duration = 30.seconds,
  ) {
    checkAllNodesHaveSameBlocks(
      expectedBlockCount = blocksToProduce,
      validatorStack.besuNode,
      followerStack.besuNode,
      timeout = timeout,
    )
  }

  private fun checkNetworkStacksBlocksProduced(
    blocksProduced: Int,
    vararg stacks: PeeringNodeNetworkStack,
  ) {
    checkAllNodesHaveSameBlocks(blocksProduced, *stacks.map { it.besuNode }.toTypedArray())
  }
}

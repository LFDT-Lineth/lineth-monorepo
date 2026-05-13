/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.test.cluster

import maru.app.MaruApp
import maru.config.MaruConfig
import maru.consensus.ChainFork
import maru.consensus.ClFork
import maru.consensus.ElFork
import maru.extensions.encodeHex
import maru.test.extensions.latestBlockNumber
import maru.test.genesis.GenesisFactory
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.hyperledger.besu.tests.acceptance.dsl.node.BesuNode
import java.nio.file.Files
import java.nio.file.Path
import kotlin.random.Random
import kotlin.time.Instant

enum class NodeRole {
  Sequencer,
  Follower,
  Bootnode,
}

fun NodeRole.isSequencer(): Boolean = this == NodeRole.Sequencer

fun NodeRole.isFollower(): Boolean = this == NodeRole.Follower

fun NodeRole.isBootnode(): Boolean = this == NodeRole.Bootnode

interface ElNode {
  fun start()

  fun stop()

  fun engineApiUrl(): String

  fun ethApiUrl(): String

  fun headBlockNumber(): ULong
}

data class ClusterNode(
  val maru: MaruApp,
  val nodeRole: NodeRole,
  val label: String = nodeRole.name,
  val elNode: ElNode?,
  val elFollowers: List<ElNode> = emptyList(),
) {
  init {
    require(nodeRole != NodeRole.Sequencer || elNode != null) {
      "Maru $nodeRole needs an payload validator defined"
    }
  }
}
typealias ElNodeBuilder = () -> ElNode

data class BesuElNode(
  val besu: BesuNode,
  val besuCluster: BesuCluster,
) : ElNode {
  override fun start() {
    besuCluster.addNodeAndStart(besu, awaitPeerDiscovery = false)
  }

  override fun stop() {
    besu.stop()
  }

  override fun engineApiUrl(): String = besu.engineRpcUrl().get()

  override fun ethApiUrl(): String = besu.jsonRpcBaseUrl().get()

  override fun headBlockNumber(): ULong = besu.latestBlockNumber()
}

enum class RunningState {
  STARTING,
  RUNNING,
  STOPPING,
  STOPPED,
}

fun createClusterDir(): Path =
  Files.createTempDirectory("maru-cluster-${Random.nextBytes(4).encodeHex(false)}").also {
    it.toFile().deleteOnExit()
  }

class MaruCluster(
  chainId: UInt = Random.nextInt(1, Int.MAX_VALUE).toUInt(),
  blockTimeSeconds: UInt = 1u,
  val terminalTotalDifficulty: ULong? = null,
  val chainForks: Map<Instant, ChainFork> =
    mapOf(
      Instant.fromEpochSeconds(0) to ChainFork(clFork = ClFork.QBFT_PHASE0, elFork = ElFork.Prague),
    ),
  val maruConfigTemplate: MaruConfig = configTemplate,
  val maruClusterDataDir: Path = createClusterDir(),
  private val besuClusterWaitPeerDiscovery: Boolean = false,
  val besuCluster: BesuCluster = BesuCluster(),
  private val log: Logger = LogManager.getLogger(MaruCluster::class.java),
) {
  private val genesisFactory: GenesisFactory = GenesisFactory(chainId, blockTimeSeconds)
  private val nodesBuilders: MutableList<NodeBuilder> = mutableListOf()
  internal val nodes = mutableListOf<ClusterNode>()
  var runningState: RunningState = RunningState.STOPPED
    @Synchronized get
    private set

  private fun createNodeBuilder(
    label: String,
    addBesu: Boolean = false,
    configurator: (NodeBuilder) -> Unit,
  ): NodeBuilder {
    assertUniqueLabel(label)
    return NodeBuilder(
      maruConfigTemplate = maruConfigTemplate,
      clusterDataDir = maruClusterDataDir,
      nodeLabel = label,
      createDefaultElBesuNode = addBesu,
    ).also(configurator)
  }

  @Synchronized
  fun addNode(
    label: String,
    addBesu: Boolean = false,
    configurator: (NodeBuilder) -> Unit = {},
  ): MaruCluster {
    if (runningState == RunningState.RUNNING || runningState == RunningState.STARTING) {
      addNewExtraNodeAndStart(label, addBesu, configurator)
    } else {
      nodesBuilders.add(createNodeBuilder(label, addBesu, configurator))
    }
    return this
  }

  /**
   *  Add new node to already existing cluster and start it immediately.
   */
  private fun addNewExtraNodeAndStart(
    nodeLabel: String,
    addBesu: Boolean = false,
    configurator: (NodeBuilder) -> Unit = {},
  ): MaruCluster {
    val nodeStartConfigs =
      createNodeBuilder(nodeLabel, addBesu, configurator)
        .build(genesisFactory::besuGenesis, besuCluster)
    val node =
      buildClusterNode(
        nodeStartingConfig = nodeStartConfigs,
        bootNodesEnrs = getBootnodesEnrs(),
      )
    nodes += node
    return this
  }

  @Synchronized
  fun addNode(
    role: NodeRole,
    withBesuEl: Boolean = false,
    configurator: (NodeBuilder) -> Unit = {},
  ): MaruCluster =
    addNode(label = role.name.lowercase(), withBesuEl) { nodeBuilder ->
      nodeBuilder.withRole(role).let(configurator)
    }

  @Synchronized
  fun node(label: String): ClusterNode =
    nodes.firstOrNull { it.label == label }
      ?: throw IllegalArgumentException("No node with label=$label found in cluster")

  @Synchronized
  fun maruNode(label: String): MaruApp = node(label).maru

  @Synchronized
  fun maruNodes(labelPrefix: String): List<MaruApp> = nodes.filter { it.label.startsWith(labelPrefix) }.map { it.maru }

  @Synchronized
  fun maruNodes(role: NodeRole): List<MaruApp> = nodes.filter { it.nodeRole == role }.map { it.maru }

  @Synchronized
  fun elNode(label: String): ElNode? = node(label).elNode

  @Synchronized
  fun nodes(role: NodeRole): List<ClusterNode> = nodes.filter { it.nodeRole == role }

  @Synchronized
  fun besuNode(label: String): BesuNode = (node(label).elNode as BesuElNode).besu

  private fun startBesuNodes(nodesConfig: List<NodeBuilder.NodeBuildingConfig>) {
    nodesConfig.forEach { nodeStartConfig ->
      (listOf<ElNode?>(nodeStartConfig.elNode) + nodeStartConfig.elFollowers)
        .filter { it != null && it is BesuElNode }
        .forEach { elNode ->
          besuCluster.addNode((elNode as BesuElNode).besu)
        }
    }
    besuCluster.start(awaitPeerDiscovery = besuClusterWaitPeerDiscovery)
    log.info("Besu cluster started")
  }

  @Synchronized
  fun start(): MaruCluster {
    if (runningState != RunningState.STOPPED) {
      throw IllegalStateException("Cannot START MaruCluster that is $runningState")
    }
    this.runningState = RunningState.STARTING
    val nodesStartConfigs = nodesBuilders.map { it.build(genesisFactory::besuGenesis, besuCluster) }

    val sequencersAddresses =
      nodesStartConfigs
        .filter { it.nodeRole.isSequencer() }
        .map { it.nodeKey.address }

    // Initialize Maru with sequencers addresses, then init Besu Factory with ForkSchedule
    genesisFactory.initForkSchedule(
      sequencersAddresses,
      terminalTotalDifficulty,
      chainForks,
    )

    // start Besu bootnodes first
    // startBesuNodes(nodesStartConfigs)

    // Build Maru nodes after Besu are started and we have their enode URLs
    // 1. we start bootnodes, to be able to get their enrs
    // 2. then we start the remaining nodes
    val (bootnodesStartConfigs, nonBootnodesStartConfigs) = nodesStartConfigs.partition { it.nodeRole.isBootnode() }
    bootnodesStartConfigs.forEach { nodeStartingConfig ->
      nodes += buildClusterNode(nodeStartingConfig, emptyList())
    }
    val bootNodesEnrs = getBootnodesEnrs()
    log.info("Maru bootnodes started: enrs=$bootNodesEnrs")
    nonBootnodesStartConfigs.forEach { nodeStartingConfig ->
      nodes += buildClusterNode(nodeStartingConfig, bootNodesEnrs)
    }

    this.runningState = RunningState.RUNNING
    nodesBuilders.clear()
    return this
  }

  private fun buildClusterNode(
    nodeStartingConfig: NodeBuilder.NodeBuildingConfig,
    bootNodesEnrs: List<String>,
  ): ClusterNode {
    startIfBesuNode(nodeStartingConfig.elNode)
    val maruApp =
      createMaru(
        elNode = nodeStartingConfig.elNode,
        config = nodeStartingConfig.maruConfig,
        bootnodes = nodeStartingConfig.overridingBootnodesNodesLables
          ?.let { nodesEnrs(it) }
          ?: bootNodesEnrs,
        staticpeers = nodesAddr(nodeStartingConfig.staticPeersNodesLables ?: emptyList()),
        nodeKeyData = nodeStartingConfig.nodeKey,
        nodeRole = nodeStartingConfig.nodeRole,
        forkSchedule = genesisFactory.maruForkSchedule(),
      )

    maruApp.start().get()

    val clusterNode =
      ClusterNode(
        maru = maruApp,
        nodeRole = nodeStartingConfig.nodeRole,
        label = nodeStartingConfig.label,
        elNode = nodeStartingConfig.elNode,
        elFollowers = nodeStartingConfig.elFollowers,
      )
    return clusterNode
  }

  @Synchronized
  fun stop() {
    this.runningState = RunningState.STOPPING
    nodes.forEach {
      it.maru.stop().get()
      it.maru.close()
      it.elNode?.stop()
    }
    besuCluster.stop()
    this.runningState = RunningState.STOPPED
  }

  @Synchronized
  fun nodeCount(): Int = nodes.size

  private fun nodesEnrs(nodesLabels: List<String>): List<String> =
    nodesLabels.map { nodeLabel ->
      maruNode(nodeLabel).p2pNetwork.enr!!
    }

  private fun nodesAddr(nodesLabels: List<String>): List<String> =
    nodesLabels.map { nodeLabel ->
      val p2pNetwork = maruNode(nodeLabel).p2pNetwork
      "/ip4/127.0.0.1/tcp/${p2pNetwork.port}/p2p/${p2pNetwork.nodeId}"
    }

  private fun startIfBesuNode(elNode: ElNode?) {
    if (elNode is BesuElNode) {
      besuCluster.addNodeAndStart(elNode.besu, awaitPeerDiscovery = besuClusterWaitPeerDiscovery)
    }
  }

  private fun assertUniqueLabel(nodeLabel: String) {
    val allNodesLabels = nodesBuilders.map { it.nodeLabel } + nodes.map { it.label }
    require(!allNodesLabels.contains(nodeLabel)) {
      "Node labels must be unique: label=$nodeLabel already exists in the cluster"
    }
  }

  private fun getBootnodesEnrs(): List<String> =
    nodes
      .filter {
        it.nodeRole.isBootnode()
      }.map { it.maru.p2pNetwork.enr!! }
}

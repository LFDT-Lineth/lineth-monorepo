/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.test.cluster

import linea.kotlin.encodeHex
import maru.app.MaruApp
import maru.config.MaruConfig
import maru.consensus.ChainFork
import maru.consensus.ClFork
import maru.consensus.ElFork
import maru.test.extensions.latestBlockNumber
import maru.test.genesis.GenesisFactory
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.kotlin.await
import org.hyperledger.besu.tests.acceptance.dsl.node.BesuNode
import java.nio.file.Files
import java.nio.file.Path
import kotlin.random.Random
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant
import kotlin.time.toJavaDuration

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
  internal val buildingConfig: NodeBuilder.NodeBuildingConfig,
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
  val terminalBlockNumber: ULong? = null,
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
  private val stoppedMaruNodes = mutableSetOf<String>()
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

  @Synchronized
  fun besuNode(role: NodeRole): BesuNode = (nodes(role).first().elNode as BesuElNode).besu

  @Synchronized
  fun nodesWithBesuEl(): List<ClusterNode> = this.nodes.filter { it.elNode is BesuElNode }

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

  private fun startBesuNodesAndAwaitPreMergeSync(nodesConfig: List<NodeBuilder.NodeBuildingConfig>) {
    val besuNodes =
      nodesConfig
        .mapNotNull { it.elNode as? BesuElNode }
        .map { it.besu }
    if (besuNodes.isEmpty()) {
      return
    }
    besuNodes.forEach { besuCluster.addNode(it) }
    besuCluster.start(awaitPeerDiscovery = true)

    // QBFT EL genesis used in tests has difficulty 0x1 and each block adds 1, so TTD is reached at block (TTD - 1)
    val terminalBlock = terminalBlockNumber ?: (terminalTotalDifficulty!! - 1UL)
    log.info("Besu cluster started, awaiting all Besu nodes to reach terminalBlockNumber={}", terminalBlock)
    await
      .pollInterval(500.milliseconds.toJavaDuration())
      .timeout(60.seconds.toJavaDuration())
      .untilAsserted {
        val heads = besuNodes.associate { it.name to it.latestBlockNumber() }
        assertThat(heads.values)
          .withFailMessage { "Besu nodes did not reach terminalBlockNumber=$terminalBlock: heads=$heads" }
          .allMatch { it >= terminalBlock }
      }
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
      terminalBlockNumber,
      chainForks,
    )

    // start Besu bootnodes first
    // startBesuNodes(nodesStartConfigs)

    // For networks starting pre-merge, start all Besu nodes before any Maru node, so that non-validator ELs can
    // full sync the pre-merge chain while it holds only QBFT blocks. Once the sequencer crosses TTD and produces
    // PoS blocks, a pre-merge Besu node can't full sync from genesis anymore: it decodes the returned PoS headers'
    // extraData as QBFT extraData, which fails, and keeps retrying the same range forever.
    val besuNodesPreStarted = terminalTotalDifficulty != null
    if (besuNodesPreStarted) {
      startBesuNodesAndAwaitPreMergeSync(nodesStartConfigs)
    }

    // Build Maru nodes after Besu are started and we have their enode URLs
    // 1. we start bootnodes, to be able to get their enrs
    // 2. then we start the remaining nodes
    val (bootnodesStartConfigs, nonBootnodesStartConfigs) = nodesStartConfigs.partition { it.nodeRole.isBootnode() }
    bootnodesStartConfigs.forEach { nodeStartingConfig ->
      nodes += buildClusterNode(nodeStartingConfig, emptyList(), startElNode = !besuNodesPreStarted)
    }
    val bootNodesEnrs = getBootnodesEnrs()
    log.info("Maru bootnodes started: enrs=$bootNodesEnrs")
    nonBootnodesStartConfigs.forEach { nodeStartingConfig ->
      nodes += buildClusterNode(nodeStartingConfig, bootNodesEnrs, startElNode = !besuNodesPreStarted)
    }

    this.runningState = RunningState.RUNNING
    nodesBuilders.clear()
    return this
  }

  private fun buildClusterNode(
    nodeStartingConfig: NodeBuilder.NodeBuildingConfig,
    bootNodesEnrs: List<String>,
    restartConfigurator: (MaruConfig) -> MaruConfig = { it },
    startElNode: Boolean = true,
  ): ClusterNode {
    if (startElNode) startIfBesuNode(nodeStartingConfig.elNode)
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
        overridingP2PNetwork = nodeStartingConfig.overridingP2PNetwork,
        p2pNetworkFactory = nodeStartingConfig.p2pNetworkFactory,
        payloadValidationEnabled = nodeStartingConfig.payloadValidationEnabled,
        maruConfigurator = { restartConfigurator(nodeStartingConfig.maruConfigurator(it)) },
      )

    maruApp.start().get()

    val clusterNode =
      ClusterNode(
        maru = maruApp,
        nodeRole = nodeStartingConfig.nodeRole,
        label = nodeStartingConfig.label,
        elNode = nodeStartingConfig.elNode,
        elFollowers = nodeStartingConfig.elFollowers,
        buildingConfig = nodeStartingConfig,
      )
    return clusterNode
  }

  /** Stops and closes the Maru app of a node, keeping its EL node, node key and data directory */
  @Synchronized
  fun stopMaruNode(label: String) {
    require(label !in stoppedMaruNodes) { "Maru node label=$label is already stopped" }
    val maru = maruNode(label)
    maru.stop().get()
    maru.close()
    stoppedMaruNodes += label
  }

  /**
   * Starts a new Maru app for a node stopped with [stopMaruNode], reusing its node key, data directory and EL node.
   * [configurator] allows overriding the node's Maru config, e.g. to restart it with an empty data directory.
   */
  @Synchronized
  fun startMaruNode(
    label: String,
    configurator: (MaruConfig) -> MaruConfig = { it },
  ): MaruApp {
    require(label in stoppedMaruNodes) { "Maru node label=$label is not stopped" }
    val restartedNode =
      buildClusterNode(
        nodeStartingConfig = node(label).buildingConfig,
        bootNodesEnrs = getBootnodesEnrs(),
        restartConfigurator = configurator,
        startElNode = false,
      )
    nodes[nodes.indexOf(node(label))] = restartedNode
    stoppedMaruNodes -= label
    return restartedNode.maru
  }

  @Synchronized
  fun restartMaruNode(
    label: String,
    configurator: (MaruConfig) -> MaruConfig = { it },
  ): MaruApp {
    stopMaruNode(label)
    return startMaruNode(label, configurator)
  }

  @Synchronized
  fun stop() {
    this.runningState = RunningState.STOPPING
    nodes.forEach {
      if (it.label !in stoppedMaruNodes) {
        it.maru.stop().get()
        it.maru.close()
      }
      it.elNode?.stop()
    }
    stoppedMaruNodes.clear()
    // also shuts down the Besu node runner
    besuCluster.close()
    this.runningState = RunningState.STOPPED
  }

  @Synchronized
  fun nodeCount(): Int = nodes.size

  private fun nodesEnrs(nodesLabels: List<String>): List<String> =
    nodesLabels.map { nodeLabel ->
      maruNode(nodeLabel).p2pNetwork.enr!!
    }

  /** The node's P2P multiaddress, e.g. to dial it as a static peer */
  @Synchronized
  fun nodeAddr(label: String): String {
    val p2pNetwork = maruNode(label).p2pNetwork
    return "/ip4/127.0.0.1/tcp/${p2pNetwork.port}/p2p/${p2pNetwork.nodeId}"
  }

  private fun nodesAddr(nodesLabels: List<String>): List<String> = nodesLabels.map(::nodeAddr)

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

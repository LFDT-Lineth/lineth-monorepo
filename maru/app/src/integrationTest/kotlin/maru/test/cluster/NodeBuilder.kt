/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.test.cluster

import linea.testing.besu.BesuFactory
import maru.config.MaruConfig
import maru.crypto.PrivateKeyGenerator
import maru.p2p.P2PNetwork
import maru.p2p.P2PNetworkImpl
import java.nio.file.Path

class NodeBuilder(
  maruConfigTemplate: MaruConfig,
  var nodeLabel: String,
  val clusterDataDir: Path,
  val createDefaultElBesuNode: Boolean,
) {
  class NodeBuildingConfig(
    val nodeRole: NodeRole,
    val label: String = nodeRole.name,
    val maruConfig: MaruConfig,
    // applied when the node is started, see [NodeBuilder.maruConfig]
    val maruConfigurator: (MaruConfig) -> MaruConfig,
    val nodeKey: PrivateKeyGenerator.KeyData,
    private val elNodeBuilder: ElNodeBuilder?,
    private val elFollowersBuilders: List<ElNodeBuilder> = emptyList(),
    // list of nodes lables, e.g bootnode-0, sequencer, ...
    val overridingBootnodesNodesLables: List<String>? = null,
    val staticPeersNodesLables: List<String>? = null,
    val overridingP2PNetwork: P2PNetwork?,
    val p2pNetworkFactory: P2pNetworkFactory,
    val payloadValidationEnabled: Boolean,
  ) {
    val elNode: ElNode? by lazy {
      elNodeBuilder?.invoke()
    }
    val elFollowers: List<ElNode> by lazy {
      elFollowersBuilders.map { it.invoke() }
    }
  }

  private var elNodeBuilder: ElNodeBuilder? = null
  private lateinit var nodeRole: NodeRole
  private var overridingBootnodes: List<String>? = null
  private var staticPeers: List<String>? = null
  private var nodeKey: PrivateKeyGenerator.KeyData? = null
  private var overridingP2PNetwork: P2PNetwork? = null
  private var p2pNetworkFactory: P2pNetworkFactory = ::P2PNetworkImpl
  private var besuValidator: Boolean? = null
  private var payloadValidationEnabled: Boolean? = null
  private val maruConfigurators = mutableListOf<(MaruConfig) -> MaruConfig>()
  private var maruConfig: MaruConfig = updatePersistenceDataPath(nodeLabel, maruConfigTemplate)

  private fun updatePersistenceDataPath(
    label: String,
    prevConfig: MaruConfig,
  ): MaruConfig =
    prevConfig.copy(
      persistence = prevConfig.persistence.copy(
        dataPath = clusterDataDir.resolve("$label-data"),
        privateKeyPath = clusterDataDir.resolve("$label-private-key"),
      ),
    )

  // fun withElBesu(besuBuilder: () -> BesuNode): NodeBuilder {
  //   this.elNodeBuilder = { BesuElNode(besu = besuBuilder(), besuCluster = besuCluster) }
  //   return this
  // }

  fun withElNode(elNodeBuilder: ElNodeBuilder): NodeBuilder {
    this.elNodeBuilder = elNodeBuilder
    return this
  }

  fun withRole(nodeRole: NodeRole): NodeBuilder {
    this.nodeRole = nodeRole
    return this
  }

  private fun getNodeRoleFromLabel(label: String?): NodeRole {
    if (label == null) {
      return NodeRole.Follower
    }
    return when {
      label.contains("sequencer", ignoreCase = true) -> NodeRole.Sequencer
      label.contains("validator", ignoreCase = true) -> NodeRole.Sequencer
      label.contains("bootnode", ignoreCase = true) -> NodeRole.Bootnode
      else -> NodeRole.Follower
    }
  }

  fun withLabel(label: String): NodeBuilder {
    this.nodeLabel = label
    this.maruConfig = updatePersistenceDataPath(nodeLabel, this.maruConfig)
    return this
  }

  fun bootnodes(nodesLabels: List<String>): NodeBuilder {
    this.overridingBootnodes = nodesLabels
    return this
  }

  fun staticPeers(nodesLabels: List<String>): NodeBuilder {
    this.staticPeers = nodesLabels
    return this
  }

  /**
   * Customises the node's Maru config. Applied when the node is started, once the cluster has wired its EL endpoints
   * (validatorElNode, forkTransition.l2EthApiEndpoint), and before it sets the QBFT fee recipient and P2P peers.
   */
  fun maruConfig(function: (MaruConfig) -> MaruConfig): NodeBuilder {
    this.maruConfigurators += function
    return this
  }

  /** Uses a fixed node key instead of a random one, e.g. to keep a stable validator set */
  fun withNodeKey(prefixedPrivateKey: ByteArray): NodeBuilder {
    this.nodeKey = PrivateKeyGenerator.getKeyDataByPrefixedKey(prefixedPrivateKey)
    return this
  }

  fun withP2pNetwork(p2pNetwork: P2PNetwork): NodeBuilder {
    this.overridingP2PNetwork = p2pNetwork
    return this
  }

  fun withP2pNetworkFactory(p2pNetworkFactory: P2pNetworkFactory): NodeBuilder {
    this.p2pNetworkFactory = p2pNetworkFactory
    return this
  }

  /**
   * Overrides whether the default Besu EL is built as a validator (default: only for sequencers).
   * A Besu validator uses the default signer key, so with several sequencers all but one need it disabled,
   * otherwise their Besu nodes share the same enode.
   */
  fun besuValidator(enabled: Boolean): NodeBuilder {
    this.besuValidator = enabled
    return this
  }

  /** Overrides EL payload validation (default: enabled only for sequencers) */
  fun payloadValidation(enabled: Boolean): NodeBuilder {
    this.payloadValidationEnabled = enabled
    return this
  }

  private fun initBesuBuilderFunctionIfNecessary(
    besuGenesisProvider: () -> String,
    besuCluster: BesuCluster,
  ) {
    if (elNodeBuilder == null && createDefaultElBesuNode) {
      this.elNodeBuilder = {
        BesuElNode(
          BesuFactory.buildTestBesu(
            genesisFile = besuGenesisProvider(),
            validator = besuValidator ?: nodeRole.isSequencer(),
            nodeName = "$nodeLabel-besu",
          ),
          besuCluster = besuCluster,
        )
      }
    }
  }

  internal fun build(
    besuGenesisProvider: () -> String,
    besuCluster: BesuCluster,
  ): NodeBuildingConfig {
    initBesuBuilderFunctionIfNecessary(besuGenesisProvider, besuCluster)
    if (!this::nodeRole.isInitialized) {
      nodeRole = getNodeRoleFromLabel(this.nodeLabel)
    }
    require(!(nodeRole.isSequencer() && elNodeBuilder == null)) {
      "Node with $nodeRole role needs an EL node builder defined"
    }

    val maruConfigurators = this.maruConfigurators.toList()
    return NodeBuildingConfig(
      maruConfig = this.maruConfig,
      maruConfigurator = { config -> maruConfigurators.fold(config) { acc, configurator -> configurator(acc) } },
      elNodeBuilder = this.elNodeBuilder,
      nodeRole = nodeRole,
      label = nodeLabel,
      overridingBootnodesNodesLables = this.overridingBootnodes,
      staticPeersNodesLables = this.staticPeers,
      nodeKey = nodeKey ?: PrivateKeyGenerator.generatePrivateKey(),
      overridingP2PNetwork = overridingP2PNetwork,
      p2pNetworkFactory = p2pNetworkFactory,
      payloadValidationEnabled = payloadValidationEnabled ?: nodeRole.isSequencer(),
    )
  }
}

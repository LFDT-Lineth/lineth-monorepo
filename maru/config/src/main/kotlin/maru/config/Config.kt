/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.config

import linea.config.docs.ConfigDoc
import linea.config.docs.ConfigSection
import linea.domain.BlockParameter
import linea.domain.RetryConfig
import linea.kotlin.assertIs20Bytes
import linea.kotlin.encodeHex
import java.net.InetAddress
import java.net.URL
import java.nio.file.Path
import kotlin.math.max
import kotlin.time.Duration
import kotlin.time.Duration.Companion.hours
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds

data class Persistence(
  @param:ConfigDoc(
    description = "Directory where Maru stores its persistent on-disk state (database, keystore).",
    example = "/data/maru",
  )
  val dataPath: Path,
  @param:ConfigDoc(
    description = "Path to the node private key file. Defaults to a 'private-key' file under data-path.",
    default = "data-path/private-key",
  )
  val privateKeyPath: Path = dataPath.resolve("private-key"),
)

data class ApiEndpointConfig(
  val endpoint: URL,
  val jwtSecretPath: String? = null,
  val requestRetries: RetryConfig = RetryConfig.noRetries,
  val timeout: Duration = 1.minutes,
)

data class FollowersConfig(
  val followers: Map<String, ApiEndpointConfig>,
)

data class P2PConfig(
  @param:ConfigDoc(
    description = "IP address the node listens on for P2P traffic. Defaults to localhost for security.",
    default = "127.0.0.1",
  )
  val ipAddress: String = "127.0.0.1", // default to localhost for security
  @param:ConfigDoc(
    description = "TCP port the node listens on for P2P traffic. UDP discovery uses p2p.discovery.port.",
    default = "9000",
  )
  val port: UInt = 9000u,
  @param:ConfigDoc(
    description = "Static peer addresses (enodes) the node stays connected to.",
  )
  val staticPeers: List<String> = emptyList(),
  @param:ConfigDoc(
    description = "Delay before reconnecting to a dropped peer.",
    default = "PT5S",
  )
  val reconnectDelay: Duration = 5.seconds,
  @param:ConfigDoc(
    description = "Maximum number of peers the node maintains.",
    default = "25",
  )
  val maxPeers: Int = 25,
  @param:ConfigDoc(
    description = "Maximum number of out-of-sync peers tolerated before throttling sync from them. " +
      "Reserved; not currently enforced by the node.",
    default = "2",
  )
  val maxUnsyncedPeers: Int = max(1, maxPeers / 10),
  @param:ConfigSection("Discovery (node lookup) settings. Omit to disable discovery.")
  val discovery: Discovery? = null,
  @param:ConfigSection("Peer status update polling settings.")
  val statusUpdate: StatusUpdate = StatusUpdate(),
  @param:ConfigSection("Peer reputation scoring settings.")
  val reputation: Reputation = Reputation(),
  @param:ConfigDoc(
    description = "Leeway time during which a peer is tolerated despite a fork mismatch before being penalized.",
    default = "PT20S",
  )
  val peeringForkMismatchLeewayTime: Duration = 20.seconds,
  @param:ConfigSection("Gossipsub parameters. Wraps Teku's GossipConfig per the Ethereum consensus p2p spec.")
  val gossiping: Gossiping = Gossiping(),
) {
  init {
    validateIpAddress(ipAddress)
    require(reputation.smallChange > 0) {
      "smallChange must be a positive number"
    }
    require(reputation.largeChange > reputation.smallChange) {
      "largeChange must be greater than smallChange"
    }
  }

  data class Discovery(
    @param:ConfigDoc(
      description = "UDP port used for discovery.",
      default = "9000",
    )
    val port: UInt = 9000u,
    @param:ConfigDoc(
      description = "Bootnode addresses (enodes) used to bootstrap discovery.",
    )
    val bootnodes: List<String> = emptyList(),
    @param:ConfigDoc(
      description = "Interval between discovery table refresh cycles.",
    )
    val refreshInterval: Duration,
    @param:ConfigDoc(
      description = "Interval between discovery search runs.",
      default = "PT1S",
    )
    val searchInterval: Duration = 1.seconds,
    @param:ConfigDoc(
      description = "Timeout for a single discovery search run.",
      default = "PT30S",
    )
    val searchTimeout: Duration = 30.seconds,
    @param:ConfigDoc(
      description = "Timeout before retrying a failed discovery request.",
      default = "PT10S",
    )
    val retryTimeout: Duration = 10.seconds,
    @param:ConfigDoc(
      description = "IP address advertised to peers for discovery. Omit to use the listen IP address.",
    )
    val advertisedIp: String? = null,
  ) {
    init {
      advertisedIp?.let { validateIpAddress(it) }
    }
  }

  companion object {
    private fun validateIpAddress(ip: String) {
      require(ip.isNotBlank()) {
        "IP address must not be blank"
      }
      // InetAddress.getByName accepts both IP addresses and hostnames.
      // We need to ensure it's actually an IP address by checking that
      // the parsed address matches the input (no DNS resolution occurred)
      val address = InetAddress.getByName(ip)
      require(address.hostAddress == ip) {
        "Invalid IP address format: $ip"
      }
    }
  }

  data class StatusUpdate(
    @param:ConfigDoc(
      description = "Interval between peer status refreshes.",
      default = "PT30S",
    )
    val refreshInterval: Duration = 30.seconds,
    @param:ConfigDoc(
      description = "Leeway applied to the peer status refresh interval.",
      default = "PT5S",
    )
    val refreshIntervalLeeway: Duration = 5.seconds,
    @param:ConfigDoc(
      description = "Timeout for a single peer status update request.",
      default = "PT10S",
    )
    val timeout: Duration = 10.seconds,
  )

  data class Reputation(
    @param:ConfigDoc(
      description = "Maximum number of peers tracked in the reputation table.",
      default = "1024",
    )
    val capacity: Int = 1024,
    @param:ConfigDoc(
      description = "Reputation score delta applied for a large positive/negative event.",
      default = "10",
    )
    val largeChange: Int = 10,
    @param:ConfigDoc(
      description = "Reputation score delta applied for a small positive/negative event.",
      default = "3",
    )
    val smallChange: Int = 3,
    @param:ConfigDoc(
      description = "Reputation score below which a peer is disconnected. Defaults to -large-change.",
      default = "-10",
    )
    val disconnectScoreThreshold: Int = -largeChange,
    @param:ConfigDoc(
      description = "Maximum reputation score a peer can reach. Defaults to 2 * large-change.",
      default = "20",
    )
    val maxReputation: Int = 2 * largeChange,
    @param:ConfigDoc(
      description = "Duration a peer's reputation is held before decaying after a change.",
      default = "PT2M",
    )
    val cooldownPeriod: Duration = 2.minutes,
    @param:ConfigDoc(
      description = "Duration a peer is banned after dropping below the disconnect threshold.",
      default = "PT1H",
    )
    val banPeriod: Duration = 1.hours,
  )

  /**
   * Gossip options wrapping Teku's tech.pegasys.teku.networking.p2p.gossip.config.GossipConfig
   * https://github.com/ethereum/consensus-specs/blob/v0.11.1/specs/phase0/p2p-interface.md#the-gossip-domain-gossipsub
   */
  data class Gossiping(
    @param:ConfigDoc(
      description = "Target mesh degree (number of peers each topic is gossiped to).",
      default = "8",
    )
    val d: Int = 8,
    @param:ConfigDoc(
      description = "Lower bound on the mesh degree; peers are added when the mesh drops below this.",
      default = "6",
    )
    val dLow: Int = 6,
    @param:ConfigDoc(
      description = "Upper bound on the mesh degree; peers are pruned when the mesh exceeds this. " +
        "Defaults to 2 * d.",
      default = "16",
    )
    val dHigh: Int = d * 2,
    @param:ConfigDoc(
      description = "Degree of lazy (non-mesh) peers used for gossip amplification.",
      default = "6",
    )
    val dLazy: Int = 6,
    @param:ConfigDoc(
      description = "Time-to-live for gossip fanout messages sent to peers outside the mesh.",
      default = "PT1M",
    )
    val fanoutTTL: Duration = 60.seconds,
    @param:ConfigDoc(
      description = "Number of history windows advertised via IHAVE messages (libp2p gossipSize).",
      default = "3",
    )
    val gossipSize: Int = 3,
    @param:ConfigDoc(
      description = "Number of gossip history windows retained in the message cache (libp2p gossipHistoryLength).",
      default = "6",
    )
    val history: Int = 6,
    @param:ConfigDoc(
      description = "Interval between gossipsub heartbeat rounds.",
      default = "PT0.7S",
    )
    val heartbeatInterval: Duration = 700.milliseconds,
    @param:ConfigDoc(
      description = "Time-to-live for the seen-message cache. Defaults to 700ms * 1115.",
      default = "PT780.5S",
    )
    val seenTTL: Duration = 700.milliseconds * 1115,
    @param:ConfigDoc(
      description = "Maximum message size above which flood publishing is skipped. Defaults to 16KiB.",
      default = "16384",
    )
    val floodPublishMaxMessageSizeThreshold: Int = 1 shl 14, // 16KiB
    @param:ConfigDoc(
      description = "Fraction of non-mesh peers that receive gossip messages (libp2p gossipFactor).",
      default = "0.25",
    )
    val gossipFactor: Double = 0.25,
    @param:ConfigDoc(
      description = "Whether all peers are scored as direct peers in gossip peer scoring " +
        "(libp2p GossipPeerScoreParams.isDirect).",
      default = "false",
    )
    val considerPeersAsDirect: Boolean = false,
  )
}

data class ValidatorElNode(
  val engineApiEndpoint: ApiEndpointConfig,
  val payloadValidationEnabled: Boolean,
)

data class QbftConfig(
  val minBlockBuildTime: Duration = 500.milliseconds,
  val messageQueueLimit: Int = 1000,
  val roundExpiry: Duration? = null,
  val roundExpiryCoefficient: Double = 2.0,
  val duplicateMessageLimit: Int = 100,
  val futureMessageMaxDistance: Long = 10L,
  val futureMessagesLimit: Long = 1000L,
  val feeRecipient: ByteArray,
) {
  init {
    feeRecipient.assertIs20Bytes("feeRecipient")
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as QbftConfig

    if (messageQueueLimit != other.messageQueueLimit) return false
    if (duplicateMessageLimit != other.duplicateMessageLimit) return false
    if (futureMessageMaxDistance != other.futureMessageMaxDistance) return false
    if (futureMessagesLimit != other.futureMessagesLimit) return false
    if (minBlockBuildTime != other.minBlockBuildTime) return false
    if (roundExpiry != other.roundExpiry) return false
    if (roundExpiryCoefficient != other.roundExpiryCoefficient) return false
    if (!feeRecipient.contentEquals(other.feeRecipient)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = messageQueueLimit
    result = 31 * result + duplicateMessageLimit
    result = 31 * result + futureMessageMaxDistance.hashCode()
    result = 31 * result + futureMessagesLimit.hashCode()
    result = 31 * result + minBlockBuildTime.hashCode()
    result = 31 * result + (roundExpiry?.hashCode() ?: 0)
    result = 31 * result + roundExpiryCoefficient.hashCode()
    result = 31 * result + feeRecipient.contentHashCode()
    return result
  }

  override fun toString(): String =
    "QbftConfig(" +
      "minBlockBuildTime=$minBlockBuildTime, " +
      "messageQueueLimit=$messageQueueLimit, " +
      "roundExpiry=$roundExpiry, " +
      "roundExpiryCoefficient=$roundExpiryCoefficient, " +
      "duplicateMessageLimit=$duplicateMessageLimit, " +
      "futureMessageMaxDistance=$futureMessageMaxDistance, " +
      "futureMessagesLimit=$futureMessagesLimit, " +
      "feeRecipient=${feeRecipient.encodeHex()}" +
      ")"
}

data class ObservabilityConfig(
  @param:ConfigDoc(
    description = "Port serving observability endpoints (metrics, health).",
    default = "9545",
  )
  val port: UInt = 9545u,
  @param:ConfigDoc(
    description = "Whether Prometheus metrics are exposed on the observability port.",
    default = "true",
  )
  val prometheusMetricsEnabled: Boolean = true,
  @param:ConfigDoc(
    description = "Whether JVM-level metrics are exposed in addition to application metrics.",
    default = "true",
  )
  val jvmMetricsEnabled: Boolean = true,
)

data class LineaConfig(
  val contractAddress: ByteArray,
  val l1EthApiEndpoint: ApiEndpointConfig,
  val l1PollingInterval: Duration = 6.seconds,
  val l1HighestBlockTag: BlockParameter = BlockParameter.Tag.FINALIZED,
  val l2EthApiEndpoint: ApiEndpointConfig,
) {
  init {
    contractAddress.assertIs20Bytes("contractAddress")
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as LineaConfig

    if (!contractAddress.contentEquals(other.contractAddress)) return false
    if (l1EthApiEndpoint != other.l1EthApiEndpoint) return false
    if (l1PollingInterval != other.l1PollingInterval) return false
    if (l1HighestBlockTag != other.l1HighestBlockTag) return false
    if (l2EthApiEndpoint != other.l2EthApiEndpoint) return false

    return true
  }

  override fun hashCode(): Int {
    var result = contractAddress.contentHashCode()
    result = 31 * result + l1EthApiEndpoint.hashCode()
    result = 31 * result + l1PollingInterval.hashCode()
    result = 31 * result + l1HighestBlockTag.hashCode()
    result = 31 * result + l2EthApiEndpoint.hashCode()
    return result
  }
}

data class ApiConfig(
  @param:ConfigDoc(
    description = "Port serving the Maru JSON-RPC API.",
    default = "5060",
  )
  val port: UInt = 5060u,
)

data class SyncingConfig(
  @param:ConfigDoc(
    description = "Interval between polls for peer chain height updates.",
  )
  val peerChainHeightPollingInterval: Duration,
  @param:ConfigDoc(
    description = "Sync target selection strategy. Use a bare string 'Highest' to sync to the " +
      "highest peer head, or an inline table " +
      "{ _type = 'MostFrequent', peer-chain-height-granularity = <n> } to sync to the most " +
      "frequent peer chain height. Sealed-type dispatch is enabled by the loader.",
  )
  val syncTargetSelection: SyncTargetSelection,
  @param:ConfigDoc(
    description = "Optional interval to refresh the execution-layer sync status. Omit to disable.",
  )
  val elSyncStatusRefreshInterval: Duration? = null,
  @param:ConfigDoc(
    description = "How far the sync target may be ahead of this node's head before the node is " +
      "considered desynced.",
    default = "5",
  )
  val desyncTolerance: ULong = 5UL,
  @param:ConfigSection("Block download settings used while syncing.")
  val download: Download = Download(),
) {
  sealed interface SyncTargetSelection {
    data object Highest : SyncTargetSelection

    data class MostFrequent(
      val peerChainHeightGranularity: UInt,
    ) : SyncTargetSelection {
      init {
        require(peerChainHeightGranularity > 0U) {
          "peerChainHeightGranularity must be higher than 0"
        }
      }
    }
  }

  data class Download(
    @param:ConfigDoc(
      description = "Timeout for a single block-range download request.",
      default = "PT5S",
    )
    val blockRangeRequestTimeout: Duration = 5.seconds,
    @param:ConfigDoc(
      description = "Number of blocks requested in a single download batch.",
      default = "100",
    )
    val blocksBatchSize: UInt = 100u,
    @param:ConfigDoc(
      description = "Number of block-range download requests issued in parallel.",
      default = "1",
    )
    val blocksParallelism: UInt = 1u,
    @param:ConfigDoc(
      description = "Maximum number of retries for a failed download request.",
      default = "5",
    )
    val maxRetries: UInt = 5u,
    @param:ConfigDoc(
      description = "Backoff delay between download retries.",
      default = "PT1S",
    )
    val backoffDelay: Duration = 1.seconds,
    @param:ConfigDoc(
      description = "Whether to pick a random download peer unconditionally on each request.",
      default = "false",
    )
    val useUnconditionalRandomDownloadPeer: Boolean = false,
  )
}

data class ForkTransition(
  val l2EthApiEndpoint: ApiEndpointConfig? = null,
  val protocolTransitionPollingInterval: Duration = 1.seconds,
)

data class MaruConfig(
  val allowEmptyBlocks: Boolean = false,
  val persistence: Persistence,
  val qbft: QbftConfig?,
  val p2p: P2PConfig?,
  val validatorElNode: ValidatorElNode?,
  val followers: FollowersConfig,
  val observability: ObservabilityConfig,
  val linea: LineaConfig? = null,
  val api: ApiConfig,
  val syncing: SyncingConfig,
  val forkTransition: ForkTransition,
  val useVertxTimers: Boolean = false,
) {
  init {
    if (qbft != null) {
      require(validatorElNode != null) {
        "Validator EL node is required when a node is a QBFT Validator"
      }
      require(validatorElNode.payloadValidationEnabled) {
        "When node is a Validator, payload validation must be enabled"
      }
    }
    if (validatorElNode != null) {
      require(
        !followers.followers.values
          .map { it.endpoint }
          .contains(validatorElNode.engineApiEndpoint.endpoint),
      ) {
        "Validator EL node cannot be defined as a follower"
      }
    }
  }
}

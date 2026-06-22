/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.config

import com.sksamuel.hoplite.ConfigException
import com.sksamuel.hoplite.ExperimentalHoplite
import linea.domain.BlockParameter
import linea.domain.RetryConfig
import linea.kotlin.decodeHex
import maru.config.MaruConfigLoader.parseConfig
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import java.net.URI
import kotlin.io.path.Path
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

@OptIn(ExperimentalHoplite::class)
class HopliteFriendlinessTest {
  private val protocolTransitionPollingInterval = 2.seconds
  private val staticPeer =
    "/dns4/bootnode.linea.build/tcp/3322/p2p/" +
      "16Uiu2HAmFjVuJoKD6sobrxwyJyysM1rgCsfWKzFLwvdB2HKuHwTg"
  private val discoveryBootnode =
    "enr:-Iu4QHk0YN5IRRnufqsWkbO6Tn0iGTx4H_hnyiIEdXDuhIe0KKrxmaECisyvO40mEmmqKLhz_tdIhx2y" +
      "FBK8XFKhvxABgmlkgnY0gmlwhH8AAAGJc2VjcDI1NmsxoQOgBvD-dv0cX5szOeEsiAMtwxnP1q5CA5toY" +
      "DrgUyOhV4N0Y3CCJBKDdWRwgiQT"

  private val baseConfigWithoutQbftAndPayloadToml =
    """
    [persistence]
    data-path="/some/path"
    private-key-path = "/private-key/path"

    [p2p]
    port = 3322
    ip-address = "10.11.12.13"
    static-peers = ["$staticPeer"]
    reconnect-delay = "500 ms"
    peering-fork-mismatch-leeway-time = "10 seconds"

    [p2p.discovery]
    port = 3324
    bootnodes = ["$discoveryBootnode"]
    refresh-interval = "30 seconds"
    search-timeout = "10 seconds"
    search-interval = "2 seconds"
    advertised-ip = "13.12.11.10"

    [p2p.reputation]
    small-change = 1
    large-change = 5
    disconnect-score-threshold = -20
    capacity = 100
    cooldown-period = "2 seconds"
    ban-period = "30 seconds"

    [p2p.status-update]
    refresh-interval = "30 seconds"
    refresh-interval-leeway = "10 seconds"
    timeout = "3 seconds"

    [observability]
    port = 9090

    [api]
    port = 8080

    [syncing]
    desync-tolerance = 10
    peer-chain-height-polling-interval = "5 seconds"
    el-sync-status-refresh-interval = "6 seconds"
    sync-target-selection = { _type = "MostFrequent", peer-chain-height-granularity = 10 }

    [syncing.download]
    block-range-request-timeout = "10 seconds"
    blocks-batch-size = 64
    blocks-parallelism = 10
    max-retries = 5
    backoff-delay = "10 seconds"
    use-unconditional-random-download-peer = true
    """.trimIndent()

  private val defaultsSectionToml =
    """
    [defaults]
    l2-eth-endpoint = { endpoint = "http://localhost:8545" }
    """.trimIndent()

  private val qbftSectionToml =
    """
    [qbft]
    fee-recipient = "0xdead000000000000000000000000000000000000"
    """.trimIndent()

  private val forkTransitionToml =
    """
    [fork-transition]
    l2-eth-api-endpoint = { endpoint = "http://localhost:8595" }
    protocol-transition-polling-interval = "2s"
    """.trimIndent()

  private val payloadValidatorSectionToml =
    """
    [payload-validator]
    engine-api-endpoint = { endpoint = "http://localhost:8555", jwt-secret-path = "/secret/path" }
    payload-validation-enabled = true
    """.trimIndent()

  private val emptyFollowersConfigToml =
    """
    $baseConfigWithoutQbftAndPayloadToml

    $defaultsSectionToml

    $qbftSectionToml

    $payloadValidatorSectionToml

    $forkTransitionToml
    """.trimIndent()
  private val rawConfigToml =
    """
    $emptyFollowersConfigToml

    [follower-engine-apis]
    follower1 = { endpoint = "http://localhost:1234", jwt-secret-path = "/secret/path" }
    follower2 = { endpoint = "http://localhost:4321", timeout = "25 seconds" }
    """.trimIndent()
  private val dataPath = Path("/some/path")
  private val privateKeyPath = Path("/private-key/path")
  private val persistence = Persistence(dataPath, privateKeyPath)
  private val p2pConfig =
    P2PConfig(
      ipAddress = "10.11.12.13",
      port = 3322u,
      staticPeers = listOf(
        staticPeer,
      ),
      reconnectDelay = 500.milliseconds,
      peeringForkMismatchLeewayTime = 10.seconds,
      discovery = P2PConfig.Discovery(
        port = 3324u,
        bootnodes = listOf(
          discoveryBootnode,
        ),
        refreshInterval = 30.seconds,
        searchTimeout = 10.seconds,
        searchInterval = 2.seconds,
        advertisedIp = "13.12.11.10",
      ),
      reputation = P2PConfig.Reputation(
        smallChange = 1,
        largeChange = 5,
        disconnectScoreThreshold = -20,
        capacity = 100,
        cooldownPeriod = 2.seconds,
        banPeriod = 30.seconds,
      ),
      statusUpdate = P2PConfig.StatusUpdate(
        refreshInterval = 30.seconds,
        refreshIntervalLeeway = 10.seconds,
        timeout = 3.seconds,
      ),
    )
  private val defaultEthApiEndpoint =
    ApiEndpointConfig(
      endpoint = URI.create("http://localhost:8545").toURL(),
      requestRetries = RetryConfig.noRetries,
    )
  private val engineApiEndpoint =
    ApiEndpointConfig(
      endpoint = URI.create("http://localhost:8555").toURL(),
      jwtSecretPath = "/secret/path",
      requestRetries = RetryConfig.endlessRetry(
        backoffDelay = 1.seconds,
        failuresWarningThreshold = 3u,
      ),
    )
  private val payloadValidator =
    PayloadValidatorDto(
      engineApiEndpoint = ApiEndpointDto(
        endpoint = URI.create("http://localhost:8555").toURL(),
        jwtSecretPath = "/secret/path",
      ),
      payloadValidationEnabled = true,
    )
  private val follower1 =
    ApiEndpointDto(
      endpoint = URI.create("http://localhost:1234").toURL(),
      jwtSecretPath = "/secret/path",
    )
  private val follower2 =
    ApiEndpointDto(
      endpoint = URI.create("http://localhost:4321").toURL(),
      timeout = 25.seconds,
    )
  private val followersConfig =
    FollowersConfig(
      mapOf(
        "follower1" to
          ApiEndpointConfig(
            endpoint = URI.create("http://localhost:1234").toURL(),
            jwtSecretPath = "/secret/path",
          ),
        "follower2" to
          ApiEndpointConfig(
            endpoint = URI.create("http://localhost:4321").toURL(),
            timeout = 25.seconds,
          ),
      ),
    )
  private val emptyFollowersConfig = FollowersConfig(emptyMap())
  private val qbftOptions =
    QbftOptionsDtoToml(
      minBlockBuildTime = 500.milliseconds,
      messageQueueLimit = 1000,
      roundExpiry = null,
      duplicateMessageLimit = 100,
      futureMessageMaxDistance = 10L,
      futureMessagesLimit = 1000L,
      feeRecipient = "0xdead000000000000000000000000000000000000".decodeHex(),
    )
  private val syncingConfig =
    SyncingConfig(
      peerChainHeightPollingInterval = 5.seconds,
      syncTargetSelection = SyncingConfig.SyncTargetSelection.MostFrequent(
        peerChainHeightGranularity = 10U,
      ),
      elSyncStatusRefreshInterval = 6.seconds,
      desyncTolerance = 10UL,
      download = SyncingConfig.Download(
        blockRangeRequestTimeout = 10.seconds,
        blocksBatchSize = 64u,
        blocksParallelism = 10u,
        maxRetries = 5u,
        backoffDelay = 10.seconds,
        useUnconditionalRandomDownloadPeer = true,
      ),
    )
  private val defaultsDto = DefaultsDtoToml(l2EthEndpoint = ApiEndpointDto(URI.create("http://localhost:8545").toURL()))
  private val forkTransitionOverrideEndpoint =
    ApiEndpointConfig(
      endpoint = URI.create("http://localhost:8595").toURL(),
      requestRetries = RetryConfig.noRetries,
    )
  private val expectedForkTransition =
    ForkTransition(
      l2EthApiEndpoint = forkTransitionOverrideEndpoint,
      protocolTransitionPollingInterval = protocolTransitionPollingInterval,
    )

  private val expectedEmptyFollowersBase =
    MaruConfigDtoToml(
      allowEmptyBlocks = false,
      defaults = defaultsDto,
      persistence = persistence,
      qbft = qbftOptions,
      p2p = p2pConfig,
      payloadValidator = payloadValidator,
      followerEngineApis = null,
      observability = ObservabilityConfig(port = 9090u),
      api = ApiConfig(port = 8080u),
      syncing = syncingConfig,
      forkTransition = ForkTransitionDtoToml(
        l2EthApiEndpoint = ApiEndpointDto(URI.create("http://localhost:8595").toURL()),
        protocolTransitionPollingInterval = protocolTransitionPollingInterval,
      ),
    )

  @Test
  fun appConfigFileIsParseable() {
    val config = parseConfig<MaruConfigDtoToml>(rawConfigToml)
    assertThat(config).isEqualTo(
      expectedEmptyFollowersBase.copy(followerEngineApis = mapOf("follower1" to follower1, "follower2" to follower2)),
    )
  }

  @Test
  fun supportsEmptyFollowers() {
    val config = parseConfig<MaruConfigDtoToml>(emptyFollowersConfigToml)
    assertThat(config).isEqualTo(
      expectedEmptyFollowersBase,
    )
  }

  @Test
  fun `throws when el validator is also specified as follower`() {
    val exception =
      assertThrows<IllegalArgumentException> {
        MaruConfig(
          persistence = persistence,
          qbft = qbftOptions.toDomain(),
          p2p = p2pConfig,
          validatorElNode = payloadValidator.domainFriendly(),
          followers = FollowersConfig(mapOf("el-validator" to payloadValidator.engineApiEndpoint.domainFriendly())),
          observability = ObservabilityConfig(port = 9090u),
          linea = null,
          api = ApiConfig(port = 8080u),
          syncing = syncingConfig,
          forkTransition = ForkTransition(protocolTransitionPollingInterval = protocolTransitionPollingInterval),
        )
      }
    assertThat(exception.message).isEqualTo("Validator EL node cannot be defined as a follower")
  }

  @Test
  fun appConfigFileIsConvertableToDomain() {
    val config = parseConfig<MaruConfigDtoToml>(rawConfigToml)
    assertThat(config.domainFriendly()).isEqualTo(
      MaruConfig(
        persistence = persistence,
        qbft = qbftOptions.toDomain(),
        p2p = p2pConfig,
        validatorElNode = ValidatorElNode(
          engineApiEndpoint = engineApiEndpoint,
          payloadValidationEnabled = true,
        ),
        followers = followersConfig,
        observability = ObservabilityConfig(port = 9090u),
        api = ApiConfig(port = 8080u),
        syncing = syncingConfig,
        forkTransition = expectedForkTransition,
      ),
    )
  }

  @Test
  fun emptyFollowersAreConvertableToDomain() {
    val config = parseConfig<MaruConfigDtoToml>(emptyFollowersConfigToml)
    assertThat(config.domainFriendly()).isEqualTo(
      MaruConfig(
        persistence = persistence,
        qbft = qbftOptions.toDomain(),
        p2p = p2pConfig,
        validatorElNode = ValidatorElNode(
          engineApiEndpoint = engineApiEndpoint,
          payloadValidationEnabled = true,
        ),
        followers = emptyFollowersConfig,
        observability = ObservabilityConfig(port = 9090u),
        api = ApiConfig(port = 8080u),
        syncing = syncingConfig,
        forkTransition = expectedForkTransition,
      ),
    )
  }

  private val qbftOptionsToml =
    """
    min-block-build-time=200m
    message-queue-limit = 1001
    round-expiry = "10 seconds"
    round-expiry-coefficient = 3.5
    duplicateMessageLimit = 99
    future-message-max-distance = 11
    future-messages-limit = 100
    fee-recipient = "0x0000000000000000000000000000000000000001"
    """.trimIndent()

  @Test
  fun validatorDutiesAreParseable() {
    val config = parseConfig<QbftConfig>(qbftOptionsToml)
    assertThat(config).isEqualTo(
      QbftConfig(
        minBlockBuildTime = 200.milliseconds,
        messageQueueLimit = 1001,
        roundExpiry = 10.seconds,
        roundExpiryCoefficient = 3.5,
        duplicateMessageLimit = 99,
        futureMessageMaxDistance = 11,
        futureMessagesLimit = 100,
        feeRecipient = "0x0000000000000000000000000000000000000001".decodeHex(),
      ),
    )
  }

  @Test
  fun payloadValidationEnablementFlagIsParseableWhenTrue() {
    val payloadValidatorToml =
      """
      engine-api-endpoint = { endpoint = "http://localhost:8555", jwt-secret-path = "/secret/path" }
      payload-validation-enabled = true
      """.trimIndent()
    val config = parseConfig<PayloadValidatorDto>(payloadValidatorToml)
    assertThat(config).isEqualTo(
      payloadValidator.copy(payloadValidationEnabled = true),
    )
  }

  @Test
  fun payloadValidationIsEnabledByDefault() {
    val payloadValidatorToml =
      """
      engine-api-endpoint = { endpoint = "http://localhost:8555", jwt-secret-path = "/secret/path" }
      """.trimIndent()
    val config = parseConfig<PayloadValidatorDto>(payloadValidatorToml)
    assertThat(config).isEqualTo(
      payloadValidator.copy(payloadValidationEnabled = true),
    )
  }

  @Test
  fun `payload validation can be disabled for non-validator nodes`() {
    val payloadValidatorDisabledToml =
      """
      engine-api-endpoint = { endpoint = "http://localhost:8555", jwt-secret-path = "/secret/path" }
      payload-validation-enabled = false
      """.trimIndent()
    val config = parseConfig<PayloadValidatorDto>(payloadValidatorDisabledToml)
    assertThat(config).isEqualTo(
      payloadValidator.copy(payloadValidationEnabled = false),
    )

    val fullConfigToml =
      """
      $baseConfigWithoutQbftAndPayloadToml

      $defaultsSectionToml

      [payload-validator]
      engine-api-endpoint = { endpoint = "http://localhost:8555", jwt-secret-path = "/secret/path" }
      payload-validation-enabled = false
      """.trimIndent()

    val domainConfig = parseConfig<MaruConfigDtoToml>(fullConfigToml).domainFriendly()
    assertThat(domainConfig.validatorElNode).isNotNull
    assertThat(domainConfig.validatorElNode?.payloadValidationEnabled).isFalse()
  }

  data class SyncTargetSelectionWrapper(
    val syncTargetSelection: SyncingConfig.SyncTargetSelection,
  )

  private val syncTargetSelectorForMostFrequentToml =
    """
    sync-target-selection = { _type = "MostFrequent", peer-chain-height-granularity = 10 }
    """.trimIndent()

  @Test
  fun syncTargetSelectorForMostFrequentIsParseable() {
    val config = parseConfig<SyncTargetSelectionWrapper>(syncTargetSelectorForMostFrequentToml)
    assertThat(config.syncTargetSelection).isEqualTo(
      SyncingConfig.SyncTargetSelection.MostFrequent(
        peerChainHeightGranularity = 10U,
      ),
    )
  }

  private val syncTargetSelectorMostFrequentWithInvalidGranularityToml =
    """
    sync-target-selection = { _type = "MostFrequent", peer-chain-height-granularity = 0 }
    """.trimIndent()

  @Test
  fun syncTargetSelectorForMostFrequentWithInvalidGranularityIsNotParseable() {
    assertThatThrownBy {
      parseConfig<SyncTargetSelectionWrapper>(syncTargetSelectorMostFrequentWithInvalidGranularityToml)
    }.isInstanceOf(ConfigException::class.java)
      .hasMessageContaining("peerChainHeightGranularity must be higher than 0")
  }

  private val syncTargetSelectorMostFrequentWithoutGranularityToml =
    """
    _type = "MostFrequent"
    """.trimIndent()

  @Test
  fun syncTargetSelectorForMostFrequentWithoutGranularityIsNotParseable() {
    assertThatThrownBy {
      parseConfig<SyncingConfig.SyncTargetSelection.MostFrequent>(syncTargetSelectorMostFrequentWithoutGranularityToml)
    }.isInstanceOf(ConfigException::class.java)
      .hasMessageContaining("Missing class kotlin.UInt from config")
  }

  private val syncTargetSelectorInvalidTypeToml =
    """
    sync-target-selection = { _type = "leastFrequent" }
    """.trimIndent()

  @Test
  fun syncTargetSelectorWithInvalidTypeIsNotParseable() {
    assertThatThrownBy {
      parseConfig<SyncTargetSelectionWrapper>(syncTargetSelectorInvalidTypeToml)
    }.isInstanceOf(ConfigException::class.java)
      .hasMessageContaining("No sealed subtype of ")
      .hasMessageContaining(" was found using the discriminator value `leastFrequent`")
  }

  private val syncTargetSelectorForHighestToml =
    """
    sync-target-selection = "Highest"
    """.trimIndent()

  @Test
  fun syncTargetSelectorForHighestIsParseable() {
    val config = parseConfig<SyncTargetSelectionWrapper>(syncTargetSelectorForHighestToml)
    assertThat(config.syncTargetSelection).isEqualTo(
      SyncingConfig.SyncTargetSelection.Highest,
    )
  }

  @Test
  fun `should parse allowEmptyBlocks = true`() {
    val configToml =
      """
      allow-empty-blocks = true
      $emptyFollowersConfigToml
      """.trimIndent()
    val config = parseConfig<MaruConfigDtoToml>(configToml)

    assertThat(config).isEqualTo(
      expectedEmptyFollowersBase.copy(allowEmptyBlocks = true),
    )

    assertThat(config.domainFriendly()).isEqualTo(
      MaruConfig(
        allowEmptyBlocks = true,
        persistence = persistence,
        validatorElNode = ValidatorElNode(
          engineApiEndpoint = engineApiEndpoint,
          payloadValidationEnabled = true,
        ),
        qbft = qbftOptions.toDomain(),
        p2p = p2pConfig,
        followers = emptyFollowersConfig,
        observability = ObservabilityConfig(port = 9090u),
        api = ApiConfig(port = 8080u),
        syncing = syncingConfig,
        forkTransition = expectedForkTransition,
      ),
    )
  }

  @Test
  fun `should parse config with linea settings`() {
    val contractAddress = "0xB218f8A4Bc926cF1cA7b3423c154a0D627Bdb7E5"
    val l1EthApiUrl = "http://ethereum-mainnet"
    val l1PollingInterval = 6.seconds
    val l1HighestBlockTag = "latest"
    val configToml =
      """
      $emptyFollowersConfigToml

      [linea]
      contract-address = "$contractAddress"
      l1-eth-api = { endpoint = "$l1EthApiUrl" }
      l1-polling-interval = "6 seconds"
      l1-highest-block-tag = "$l1HighestBlockTag"
      """.trimIndent()
    val l1EthApiEndpoint =
      ApiEndpointDto(
        endpoint = URI.create("http://ethereum-mainnet").toURL(),
      )
    val contractAddressBytes = contractAddress.decodeHex()
    val expectedTomlConfig =
      LineaConfigDtoToml(
        contractAddress = contractAddressBytes,
        l1EthApi = l1EthApiEndpoint,
        l1PollingInterval = l1PollingInterval,
        l1HighestBlockTag = l1HighestBlockTag,
      )
    val expectedLineaConfig =
      LineaConfig(
        contractAddress = contractAddressBytes,
        l1EthApiEndpoint = l1EthApiEndpoint.domainFriendly(),
        l1PollingInterval = l1PollingInterval,
        l1HighestBlockTag = BlockParameter.Tag.LATEST,
        l2EthApiEndpoint = ApiEndpointConfig(URI.create("http://localhost:8545").toURL()),
      )
    val config = parseConfig<MaruConfigDtoToml>(configToml)

    assertThat(config).isEqualTo(
      expectedEmptyFollowersBase.copy(linea = expectedTomlConfig),
    )

    assertThat(config.domainFriendly()).isEqualTo(
      MaruConfig(
        linea = expectedLineaConfig,
        persistence = persistence,
        p2p = p2pConfig,
        validatorElNode = ValidatorElNode(
          engineApiEndpoint = engineApiEndpoint,
          payloadValidationEnabled = true,
        ),
        qbft = qbftOptions.toDomain(),
        followers = emptyFollowersConfig,
        observability = ObservabilityConfig(port = 9090u),
        api = ApiConfig(port = 8080u),
        syncing = syncingConfig,
        forkTransition = expectedForkTransition,
      ),
    )
  }

  @Test
  fun `linea config parses with legacy l1-eth-api only`() {
    val contractAddress = "0xB218f8A4Bc926cF1cA7b3423c154a0D627Bdb7E5"
    val legacyUrl = "http://ethereum-legacy"

    val toml =
      """
      contract-address = "$contractAddress"
      l1-eth-api = { endpoint = "$legacyUrl" }
      """.trimIndent()

    val expected =
      LineaConfigDtoToml(
        contractAddress = contractAddress.decodeHex(),
        l1EthApi = ApiEndpointDto(URI.create(legacyUrl).toURL()),
      )

    val parsed = parseConfig<LineaConfigDtoToml>(toml)

    assertThat(parsed).isEqualTo(expected)
    // Fallback applied: l1EthApiEndpoint defaults to l1EthApi
    assertThat(parsed.l1EthApiEndpoint).isEqualTo(expected.l1EthApi)
  }

  @Test
  fun `linea config parses with l1-eth-api-endpoint only`() {
    val contractAddress = "0xB218f8A4Bc926cF1cA7b3423c154a0D627Bdb7E5"
    val endpointUrl = "http://ethereum-endpoint"

    val toml =
      """
      contract-address = "$contractAddress"
      l1-eth-api-endpoint = { endpoint = "$endpointUrl" }
      """.trimIndent()

    val expected =
      LineaConfigDtoToml(
        contractAddress = contractAddress.decodeHex(),
        l1EthApiEndpoint = ApiEndpointDto(URI.create(endpointUrl).toURL()),
      )

    val parsed = parseConfig<LineaConfigDtoToml>(toml)

    assertThat(parsed).isEqualTo(expected)
  }

  @Test
  fun `linea config fails when neither l1-eth-api nor l1-eth-api-endpoint provided`() {
    val contractAddress = "0xB218f8A4Bc926cF1cA7b3423c154a0D627Bdb7E5"

    val toml =
      """
      contract-address = "$contractAddress"
      """.trimIndent()

    assertThatThrownBy {
      parseConfig<LineaConfigDtoToml>(toml)
    }.isInstanceOf(ConfigException::class.java)
      .hasMessageContaining("l1-eth-api-endpoint has to be defined!")
  }

  @Test
  fun `gossiping config can be overridden`() {
    val customGossipingConfig =
      """
      d = 12
      d-low = 10
      d-high = 16
      d-lazy = 10
      fanout-ttl = "120 seconds"
      gossip-size = 6
      history = 10
      heartbeat-interval = "600 milliseconds"
      seen-ttl = "1200 seconds"
      flood-publish-max-message-size-threshold = 65536
      gossip-factor = 0.5
      consider-peers-as-direct = true
      """.trimIndent()
    val config = parseConfig<P2PConfig.Gossiping>(customGossipingConfig)

    assertThat(config).isEqualTo(
      P2PConfig.Gossiping(
        d = 12,
        dLow = 10,
        dHigh = 16,
        dLazy = 10,
        fanoutTTL = 120.seconds,
        gossipSize = 6,
        history = 10,
        heartbeatInterval = 600.milliseconds,
        seenTTL = 1200.seconds,
        floodPublishMaxMessageSizeThreshold = 65536,
        gossipFactor = 0.5,
        considerPeersAsDirect = true,
      ),
    )
  }

  @Test
  fun `gossiping config can be partially overridden with defaults for unspecified fields`() {
    val partialGossipingConfig =
      """
      d = 12
      heartbeat-interval = "600 milliseconds"
      """.trimIndent()
    val config = parseConfig<P2PConfig.Gossiping>(partialGossipingConfig)

    assertThat(config).isEqualTo(
      P2PConfig.Gossiping(
        d = 12,
        dLow = 6, // default
        dHigh = 24, // default (d * 2)
        dLazy = 6, // default
        fanoutTTL = 60.seconds, // default
        gossipSize = 3, // default
        history = 6, // default
        heartbeatInterval = 600.milliseconds,
        seenTTL = 700.milliseconds * 1115, // default
        floodPublishMaxMessageSizeThreshold = 16384, // default
        gossipFactor = 0.25, // default
        considerPeersAsDirect = false, // default
      ),
    )
  }

  @Test
  fun `gossiping config is parseable as standalone`() {
    val gossipingToml =
      """
      d = 10
      d-low = 8
      d-high = 14
      d-lazy = 8
      fanout-ttl = "90 seconds"
      gossip-size = 5
      history = 8
      heartbeat-interval = "500 milliseconds"
      seen-ttl = "1000 seconds"
      flood-publish-max-message-size-threshold = 32768
      gossip-factor = 0.3
      consider-peers-as-direct = true
      """.trimIndent()
    val config = parseConfig<P2PConfig.Gossiping>(gossipingToml)

    assertThat(config).isEqualTo(
      P2PConfig.Gossiping(
        d = 10,
        dLow = 8,
        dHigh = 14,
        dLazy = 8,
        fanoutTTL = 90.seconds,
        gossipSize = 5,
        history = 8,
        heartbeatInterval = 500.milliseconds,
        seenTTL = 1000.seconds,
        floodPublishMaxMessageSizeThreshold = 32768,
        gossipFactor = 0.3,
        considerPeersAsDirect = true,
      ),
    )
  }

  @Test
  fun `p2p discovery with invalid advertisedIp format should fail`() {
    val discoveryConfigToml =
      """
      port = 9000
      refresh-interval = "30 seconds"
      advertised-ip = "127.0.256.1"
      """.trimIndent()

    assertThatThrownBy {
      parseConfig<P2PConfig.Discovery>(discoveryConfigToml)
    }.isInstanceOf(ConfigException::class.java)
      .hasMessageContaining("UnknownHostException")
      .hasMessageContaining("127.0.256.1")
  }

  @Test
  fun `p2p config with invalid ipAddress format should fail`() {
    val p2pConfigToml =
      """
      ip-address = "127.O.0H.1"
      """.trimIndent()

    assertThatThrownBy {
      parseConfig<P2PConfig>(p2pConfigToml)
    }.isInstanceOf(ConfigException::class.java)
      .hasMessageContaining("UnknownHostException")
      .hasMessageContaining("127.O.0H.1")
  }

  @Test
  fun `p2p config with valid dns instead of IP format should fail`() {
    val p2pConfigToml =
      """
      ip-address = "localhost"
      """.trimIndent()

    assertThatThrownBy {
      parseConfig<P2PConfig>(p2pConfigToml)
    }.isInstanceOf(ConfigException::class.java)
      .hasMessageContaining("IllegalArgumentException")
      .hasMessageContaining("Invalid IP address format")
      .hasMessageContaining("localhost")
  }

  @Test
  fun `fails when payload-validator section is missing while qbft one is there`() {
    val tomlNoPayload =
      """
      $baseConfigWithoutQbftAndPayloadToml

      $defaultsSectionToml

      $qbftSectionToml
      """.trimIndent()

    val parsed = parseConfig<MaruConfigDtoToml>(tomlNoPayload)
    assertThat(parsed).isEqualTo(
      expectedEmptyFollowersBase.copy(
        payloadValidator = null,
        forkTransition = ForkTransitionDtoToml(),
      ),
    )

    assertThatThrownBy { parsed.domainFriendly() }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessage("Validator EL node is required when a node is a QBFT Validator")
  }

  @Test
  fun `follower config when both qbft and payload-validator are absent`() {
    val tomlNoPayloadNoQbft =
      """
      $baseConfigWithoutQbftAndPayloadToml

      $defaultsSectionToml
      """.trimIndent()

    val parsed = parseConfig<MaruConfigDtoToml>(tomlNoPayloadNoQbft)
    assertThat(parsed).isEqualTo(
      expectedEmptyFollowersBase.copy(
        payloadValidator = null,
        qbft = null,
        forkTransition = ForkTransitionDtoToml(),
      ),
    )

    assertThat(parsed.domainFriendly()).isEqualTo(
      MaruConfig(
        persistence = persistence,
        qbft = null,
        p2p = p2pConfig,
        validatorElNode = null,
        followers = emptyFollowersConfig,
        observability = ObservabilityConfig(port = 9090u),
        api = ApiConfig(port = 8080u),
        syncing = syncingConfig,
        forkTransition = ForkTransition(
          l2EthApiEndpoint = defaultEthApiEndpoint,
          protocolTransitionPollingInterval = 1.seconds,
        ),
      ),
    )
  }

  @Test
  fun `follower config without defaults section`() {
    val tomlNoDefaults =
      """
      $baseConfigWithoutQbftAndPayloadToml
      """.trimIndent()

    val parsed = parseConfig<MaruConfigDtoToml>(tomlNoDefaults)
    assertThat(parsed).isEqualTo(
      expectedEmptyFollowersBase.copy(
        payloadValidator = null,
        qbft = null,
        defaults = null,
        forkTransition = ForkTransitionDtoToml(),
      ),
    )

    assertThat(parsed.domainFriendly()).isEqualTo(
      MaruConfig(
        persistence = persistence,
        qbft = null,
        p2p = p2pConfig,
        validatorElNode = null,
        followers = emptyFollowersConfig,
        observability = ObservabilityConfig(port = 9090u),
        api = ApiConfig(port = 8080u),
        syncing = syncingConfig,
        forkTransition = ForkTransition(),
      ),
    )
  }

  @Test
  fun `fork-transition l2-eth-api-endpoint override takes precedence over defaults`() {
    val configToml =
      """
      $baseConfigWithoutQbftAndPayloadToml

      $defaultsSectionToml

      $qbftSectionToml

      $payloadValidatorSectionToml

      [fork-transition]
      l2-eth-api-endpoint = { endpoint = "http://localhost:8595" }
      protocol-transition-polling-interval = "2s"
      """.trimIndent()

    val parsed = parseConfig<MaruConfigDtoToml>(configToml)
    val domain = parsed.domainFriendly()

    val expectedEthApi =
      ApiEndpointConfig(
        endpoint = URI.create("http://localhost:8595").toURL(),
        requestRetries = RetryConfig.noRetries,
      )

    assertThat(domain.forkTransition.l2EthApiEndpoint).isEqualTo(expectedEthApi)
  }

  @Test
  fun `fork-transition without l2-eth-api-endpoint uses defaults`() {
    val configToml =
      """
      $baseConfigWithoutQbftAndPayloadToml

      $defaultsSectionToml

      $qbftSectionToml

      $payloadValidatorSectionToml

      [fork-transition]
      protocol-transition-polling-interval = "3s"
      """.trimIndent()

    val domain = parseConfig<MaruConfigDtoToml>(configToml).domainFriendly()

    val expectedEthApi =
      ApiEndpointConfig(
        endpoint = URI.create("http://localhost:8545").toURL(),
        requestRetries = RetryConfig.noRetries,
      )

    assertThat(domain.forkTransition.l2EthApiEndpoint).isEqualTo(expectedEthApi)
    assertThat(domain.forkTransition.protocolTransitionPollingInterval).isEqualTo(3.seconds)
  }
}

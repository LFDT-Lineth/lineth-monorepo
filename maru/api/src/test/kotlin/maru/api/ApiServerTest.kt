/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.api

import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import linea.kotlin.encodeHex
import maru.VersionProvider
import maru.api.beacon.GetBlock
import maru.api.beacon.GetBlockHeader
import maru.api.beacon.GetBlockHeaderResponse
import maru.api.beacon.GetBlockResponse
import maru.api.beacon.GetStateValidator
import maru.api.beacon.GetStateValidatorResponse
import maru.api.beacon.GetStateValidators
import maru.api.beacon.GetStateValidatorsResponse
import maru.api.beacon.SignedBeaconBlock
import maru.api.beacon.SignedBeaconBlockHeader
import maru.api.beacon.Validator
import maru.api.beacon.ValidatorResponse
import maru.api.beacon.toBeaconBlock
import maru.api.beacon.toBeaconBlockHeader
import maru.api.node.GetHealth
import maru.api.node.GetNetworkIdentity
import maru.api.node.GetNetworkIdentityResponse
import maru.api.node.GetPeer
import maru.api.node.GetPeerCount
import maru.api.node.GetPeerCountResponse
import maru.api.node.GetPeerResponse
import maru.api.node.GetPeers
import maru.api.node.GetPeersResponse
import maru.api.node.GetSyncingStatus
import maru.api.node.GetSyncingStatusResponse
import maru.api.node.GetVersion
import maru.api.node.GetVersionResponse
import maru.api.node.Metadata
import maru.api.node.NetworkIdentity
import maru.api.node.PeerCountData
import maru.api.node.PeerData
import maru.api.node.PeerMetaData
import maru.api.node.SyncingStatusData
import maru.api.node.VersionData
import maru.core.BeaconState
import maru.core.SealedBeaconBlock
import maru.core.ext.DataGenerators
import maru.database.InMemoryBeaconChain
import maru.p2p.NetworkDataProvider
import maru.p2p.PeerInfo
import maru.syncing.AlwaysSyncedController
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.OkHttpClient
import okhttp3.Request
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import java.util.concurrent.TimeUnit

class ApiServerTest {
  private val defaultObjectMapper = jacksonObjectMapper()

  private lateinit var apiServer: ApiServerImpl
  private lateinit var client: OkHttpClient
  private lateinit var apiServerUrl: String

  private val fakeNetworkDataProvider =
    object : NetworkDataProvider {
      override fun getNodeId(): String = "TEST_NODE_ID"

      override fun getEnr(): String = "TEST_ENR"

      override fun getNodeAddresses(): List<String> = listOf("TEST_NODE_ADDRESS")

      override fun getDiscoveryAddresses(): List<String> = listOf("TEST_DISCOVERY_ADDRESS")

      override fun getPeers(): List<PeerInfo> =
        listOf(
          PeerInfo(
            nodeId = "TEST_PEER_ID",
            enr = "TEST_PEER_ENR",
            address = "TEST_PEER_ADDRESS",
            status = PeerInfo.PeerStatus.CONNECTED,
            direction = PeerInfo.PeerDirection.OUTBOUND,
          ),
        )

      override fun getPeer(peerId: String): PeerInfo? = getPeers().firstOrNull { it.nodeId == peerId }
    }

  private val fakeVersionProvider =
    object : VersionProvider {
      override fun getVersion(): String = "maru/1.0.0-test"
    }

  private val fakeChainDataProvider =
    object : ChainDataProvider {
      val SEALED_BEACON_BLOCK = DataGenerators.randomSealedBeaconBlock(
        1u,
      )
      val BEACON_STATE = DataGenerators.randomBeaconState(
        1u,
      )

      override fun getLatestBeaconState(): BeaconState = BEACON_STATE

      override fun getBeaconStateByStateRoot(stateRoot: ByteArray): BeaconState {
        if (stateRoot.contentEquals(BEACON_STATE.beaconBlockHeader.stateRoot)) {
          return BEACON_STATE
        }
        throw BeaconStateNotFoundException()
      }

      override fun getBeaconBlockByNumber(blockNumber: ULong): SealedBeaconBlock {
        if (blockNumber == SEALED_BEACON_BLOCK.beaconBlock.beaconBlockHeader.number) {
          return SEALED_BEACON_BLOCK
        }
        throw BlockNotFoundException()
      }

      override fun getLatestBeaconBlock(): SealedBeaconBlock = SEALED_BEACON_BLOCK

      override fun getBeaconBlockByBlockRoot(blockRoot: String): SealedBeaconBlock {
        if (blockRoot ==
          SEALED_BEACON_BLOCK.beaconBlock.beaconBlockHeader
            .hash()
            .encodeHex()
        ) {
          return SEALED_BEACON_BLOCK
        }
        throw BlockNotFoundException()
      }
    }

  private val fakeBeaconChain =
    InMemoryBeaconChain(
      DataGenerators.randomBeaconState(
        number = 0u,
        timestamp = 0u,
      ),
    )
  private val fakeElOnlineProvider =
    object : () -> Boolean {
      var isElOnline: Boolean = false

      override fun invoke(): Boolean = isElOnline
    }

  @BeforeEach
  fun beforeEach() {
    apiServer = ApiServerImpl(
      config = ApiServerImpl.Config(port = 0u),
      networkDataProvider = fakeNetworkDataProvider,
      versionProvider = fakeVersionProvider,
      chainDataProvider = fakeChainDataProvider,
      syncStatusProvider = AlwaysSyncedController(beaconChain = fakeBeaconChain),
      isElOnlineProvider = fakeElOnlineProvider,
    )
    apiServer.start()
    apiServerUrl = "http://localhost:${apiServer.port()}"
    client = OkHttpClient.Builder().readTimeout(0, TimeUnit.SECONDS).build()
  }

  @AfterEach
  fun afterEach() {
    apiServer.stop()
  }

  fun makeRequest(
    apiPath: String,
    expectedStatusCode: Int = 200,
  ): String? {
    val url = (apiServerUrl + apiPath).toHttpUrl()
    val request = Request.Builder().url(url).build()
    val httpResponse = client.newCall(request).execute()
    assertThat(httpResponse).isNotNull
    assertThat(httpResponse.code).isEqualTo(expectedStatusCode)
    return httpResponse.body?.string()
  }

  fun assertResponse(
    apiPath: String,
    expectedStatusCode: Int = 200,
    expectedResponse: Any,
  ) {
    val responseBody = makeRequest(apiPath, expectedStatusCode)
    val response =
      defaultObjectMapper.readValue(
        responseBody,
        expectedResponse::class.java,
      )
    assertThat(response).isEqualTo(expectedResponse)
  }

  fun assert200okResponse(
    apiPath: String,
    expectedResponse: Any,
  ) = assertResponse(
    apiPath = apiPath,
    expectedStatusCode = 200,
    expectedResponse = expectedResponse,
  )

  @Test
  fun `test GetNetworkIdentity method`() {
    val networkIdentity =
      NetworkIdentity(
        peerId = fakeNetworkDataProvider.getNodeId(),
        enr = fakeNetworkDataProvider.getEnr(),
        p2pAddresses = fakeNetworkDataProvider.getNodeAddresses(),
        discoveryAddresses = fakeNetworkDataProvider.getDiscoveryAddresses(),
        metadata = Metadata(
          seqNumber = "0",
          attnets = "0x",
          syncnets = "0x",
        ),
      )

    assert200okResponse(GetNetworkIdentity.ROUTE, GetNetworkIdentityResponse(networkIdentity))
  }

  @Test
  fun `test GetPeers method`() {
    val peerData =
      PeerData(
        peerId = "TEST_PEER_ID",
        enr = "TEST_PEER_ENR",
        lastSeenP2PAddress = "TEST_PEER_ADDRESS",
        state = "connected",
        direction = "outbound",
      )

    val expectedResponse = GetPeersResponse(data = listOf(peerData), meta = PeerMetaData(count = 1))
    assert200okResponse(GetPeers.ROUTE, expectedResponse)
  }

  @Test
  fun `test GetPeerById method`() {
    val peerData =
      PeerData(
        peerId = "TEST_PEER_ID",
        enr = "TEST_PEER_ENR",
        lastSeenP2PAddress = "TEST_PEER_ADDRESS",
        state = "connected",
        direction = "outbound",
      )

    val expectedResponse = GetPeerResponse(data = peerData)

    assert200okResponse(GetPeer.ROUTE.replace("{${GetPeer.PEER_ID}}", "TEST_PEER_ID"), expectedResponse)
  }

  @Test
  fun `test GetPeerById method when peer not found`() {
    assertResponse(
      GetPeer.ROUTE.replace("{${GetPeer.PEER_ID}}", "TEST_PEER_ID_2"),
      expectedStatusCode = 404,
      expectedResponse = ApiExceptionResponse(404, "Peer not found"),
    )
  }

  @Test
  fun `test GetPeerCount method`() {
    val peerCountData =
      PeerCountData(
        disconnected = "0",
        connected = "1",
        connecting = "0",
        disconnecting = "0",
      )

    val expectedResponse = GetPeerCountResponse(data = peerCountData)
    assert200okResponse(GetPeerCount.ROUTE, expectedResponse)
  }

  @Test
  fun `test GetVersion method`() {
    val expectedResponse = GetVersionResponse(data = VersionData(version = fakeVersionProvider.getVersion()))
    assert200okResponse(GetVersion.ROUTE, expectedResponse)
  }

  @Test
  fun `test GetSyncingStatus method`() {
    fakeBeaconChain.newBeaconChainUpdater().apply {
      putBeaconState(
        DataGenerators.randomBeaconState(
          number = 100u,
          timestamp = 100u,
        ),
      )
      commit()
    }
    fakeElOnlineProvider.isElOnline = true

    assert200okResponse(
      GetSyncingStatus.ROUTE,
      GetSyncingStatusResponse(
        data = SyncingStatusData(
          headSlot = "100",
          syncDistance = "0",
          isSyncing = false,
          isOptimistic = true,
          elOffline = false,
        ),
      ),
    )

    fakeBeaconChain.newBeaconChainUpdater().apply {
      putBeaconState(
        DataGenerators.randomBeaconState(
          number = 200u,
          timestamp = 100u,
        ),
      )
      commit()
    }
    fakeElOnlineProvider.isElOnline = false

    assert200okResponse(
      GetSyncingStatus.ROUTE,
      GetSyncingStatusResponse(
        data = SyncingStatusData(
          headSlot = "200",
          syncDistance = "0",
          isSyncing = false,
          isOptimistic = true,
          elOffline = true,
        ),
      ),
    )
  }

  @Test
  fun `test GetHealth method`() {
    val url = (apiServerUrl + GetHealth.ROUTE).toHttpUrl()
    val request = Request.Builder().url(url).build()
    val expectedResponse = "Node is ready"
    val httpResponse = client.newCall(request).execute()
    assertThat(httpResponse).isNotNull
    assertThat(httpResponse.code).isEqualTo(200)
    val response = httpResponse.body?.string()
    assertThat(response).isEqualTo(expectedResponse)
  }

  @Test
  fun `test GetBlockHeader method`() {
    val expectedResponse =
      GetBlockHeaderResponse(
        executionOptimistic = false,
        finalized = false,
        data = SignedBeaconBlockHeader(
          message = fakeChainDataProvider.SEALED_BEACON_BLOCK.beaconBlock.beaconBlockHeader
            .toBeaconBlockHeader(),
          signature = "0x",
        ),
      )

    assert200okResponse(
      GetBlockHeader.ROUTE.replace(
        "{${GetBlockHeader.BLOCK_ID}}",
        fakeChainDataProvider.SEALED_BEACON_BLOCK.beaconBlock.beaconBlockHeader.number
          .toString(),
      ),
      expectedResponse,
    )
  }

  @Test
  fun `test GetBlock method`() {
    val expectedResponse =
      GetBlockResponse(
        executionOptimistic = false,
        finalized = false,
        data = SignedBeaconBlock(
          message = fakeChainDataProvider.SEALED_BEACON_BLOCK.toBeaconBlock(),
          signature = "0x",
        ),
        version = "maru",
      )
    assert200okResponse(
      GetBlock.ROUTE.replace(
        "{${GetBlock.BLOCK_ID}}",
        "head",
      ),
      expectedResponse,
    )
  }

  @Test
  fun `test GetStateValidator method`() {
    val validator = fakeChainDataProvider.BEACON_STATE.validators.first()
    val expectedResponse =
      GetStateValidatorResponse(
        executionOptimistic = false,
        finalized = false,
        data = ValidatorResponse(
          index = "0",
          balance = "",
          status = "active_ongoing",
          validator = Validator(
            pubkey = validator.address.encodeHex(),
            withdrawalCredentials = "0x",
            effectiveBalance = "",
            slashed = false,
            activationEligibilityEpoch = "",
            activationEpoch = "",
            exitEpoch = "",
            withdrawableEpoch = "",
          ),
        ),
      )

    val path =
      GetStateValidator.ROUTE
        .replace("{${GetStateValidator.STATE_ID}}", "head")
        .replace("{${GetStateValidator.VALIDATOR_ID}}", validator.address.encodeHex())
    assert200okResponse(path, expectedResponse)
  }

  @Test
  fun `test GetStateValidators method`() {
    val validators =
      fakeChainDataProvider.BEACON_STATE.validators.mapIndexed { index, validator ->
        ValidatorResponse(
          index = index.toString(),
          balance = "",
          status = "active_ongoing",
          validator = Validator(
            pubkey = validator.address.encodeHex(),
            withdrawalCredentials = "0x",
            effectiveBalance = "",
            slashed = false,
            activationEligibilityEpoch = "",
            activationEpoch = "",
            exitEpoch = "",
            withdrawableEpoch = "",
          ),
        )
      }
    val expectedResponse =
      GetStateValidatorsResponse(
        executionOptimistic = false,
        finalized = false,
        data = validators,
      )

    assert200okResponse(GetStateValidators.ROUTE.replace("{${GetStateValidators.STATE_ID}}", "head"), expectedResponse)
  }
}

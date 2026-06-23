/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.discovery

import linea.kotlin.decodeHex
import linea.kotlin.toULong
import linea.timer.JvmTimerFactory
import maru.config.P2PConfig
import maru.consensus.ElFork
import maru.consensus.ForkIdManagerFactory.createForkIdHashManager
import maru.core.ext.DataGenerators
import maru.database.InMemoryBeaconChain
import maru.database.InMemoryP2PState
import maru.database.P2PState
import maru.p2p.discovery.MaruDiscoveryService.Companion.FORK_ID_HASH_FIELD_NAME
import maru.p2p.discovery.MaruDiscoveryService.Companion.isValidNodeRecord
import org.apache.tuweni.bytes.Bytes
import org.apache.tuweni.crypto.SECP256K1
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.kotlin.await
import org.ethereum.beacon.discovery.crypto.DefaultSigner
import org.ethereum.beacon.discovery.schema.IdentitySchemaInterpreter
import org.ethereum.beacon.discovery.schema.NodeRecord
import org.ethereum.beacon.discovery.schema.NodeRecordBuilder
import org.ethereum.beacon.discovery.schema.NodeRecordFactory
import org.ethereum.beacon.discovery.util.Functions
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import tech.pegasys.teku.networking.p2p.discovery.DiscoveryPeer
import java.net.InetAddress
import java.net.InetSocketAddress
import java.util.Optional
import kotlin.random.Random
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

class MaruDiscoveryServiceTest {
  companion object {
    // private val IPV4 = NetworkHelper.listIpsV4(excludeLoopback = true).first()
    // Tests seem to fail when using IP address of the machine... ¯\_(ツ)_/¯
    private val IPV4 = "127.0.0.1"

    private const val PORT1 = 9334u
    private const val PORT2 = 9335u
    private const val PORT3 = 9336u
    private const val PORT4 = 9337u
    private const val PORT5 = 9338u
    private const val PORT6 = 9339u

    private val key1 = "0x12c0b113e2b0c37388e2b484112e13f05c92c4471e3ee1dfaa368fa5045325b2".decodeHex()
    private val key2 = "0xf3d2fffa99dc8906823866d96316492ebf7a8478713a89a58b7385af85b088a1".decodeHex()
    private val key3 = "0x4437acb8e84bc346f7640f239da84abe99bc6f97b7855f204e34688d2977fd57".decodeHex()

    private val chainId = 1337u
    private val beaconChain = inMemoryBeaconChainFromGenesis()
    private val forkIdHashProvider =
      createForkIdHashManager(
        chainId = chainId,
        beaconChain = beaconChain,
        elFork = ElFork.Prague,
      )

    private fun inMemoryBeaconChainFromGenesis(
      genesisTimestampSeconds: ULong = 0UL,
    ): InMemoryBeaconChain {
      val (beaconState, sealedBlock) = DataGenerators.genesisState(
        genesisTimestamp = genesisTimestampSeconds,
      )
      return InMemoryBeaconChain(beaconState, sealedBlock)
    }
  }

  private lateinit var p2PState: P2PState
  private lateinit var service: MaruDiscoveryService

  private val keyPair = SECP256K1.KeyPair.random()
  private val publicKey = Functions.deriveCompressedPublicKeyFromPrivate(keyPair.secretKey())
  private val dummyAddr = Optional.of(InetSocketAddress(InetAddress.getByName("1.1.1.1"), 1234))

  private fun createValidNodeRecord(
    forkIdHash: ByteArray? = forkIdHashProvider.currentForkHash(),
    tcpAddress: Optional<InetSocketAddress> = dummyAddr,
  ): NodeRecord {
    val nrBuilder =
      NodeRecordBuilder()
        .nodeRecordFactory(NodeRecordFactory(IdentitySchemaInterpreter.V4))
        .seq(1)
        .signer(DefaultSigner(keyPair.secretKey()))
    if (forkIdHash != null) {
      nrBuilder.customField(FORK_ID_HASH_FIELD_NAME, Bytes.wrap(forkIdHash))
    }
    if (tcpAddress.isPresent) {
      nrBuilder.address(tcpAddress.get().address.hostAddress, tcpAddress.get().port)
    }

    return nrBuilder.build()
  }

  @BeforeEach
  fun setUp() {
    val p2pConfig =
      P2PConfig(
        ipAddress = "127.0.0.1",
        port = 9001u,
        discovery = P2PConfig.Discovery(
          port = 9000u,
          bootnodes = listOf(),
          refreshInterval = 10.seconds,
        ),
      )
    p2PState = InMemoryP2PState()
    service = MaruDiscoveryService(
      privateKeyBytes = keyPair.secretKey().bytesArray(),
      p2pConfig = p2pConfig,
      forkIdHashManager = forkIdHashProvider,
      p2PState = p2PState,
      timerFactory = JvmTimerFactory(),
    )
  }

  @Test
  fun `seq number updates on local record updates`() {
    val sequenceNumberAfterInitialization = 1uL
    assertThat(p2PState.getLocalNodeRecordSequenceNumber()).isEqualTo(sequenceNumberAfterInitialization)
    service.start()
    assertThat(
      service
        .getLocalNodeRecord()
        .seq
        .toBigInteger()
        .toULong(),
    ).isEqualTo(sequenceNumberAfterInitialization)

    service.updateForkIdHash("update 1".toByteArray())
    assertThat(p2PState.getLocalNodeRecordSequenceNumber()).isEqualTo(sequenceNumberAfterInitialization + 1uL)
    assertThat(
      service
        .getLocalNodeRecord()
        .seq
        .toBigInteger()
        .toULong(),
    ).isEqualTo(sequenceNumberAfterInitialization + 1uL)

    service.updateForkIdHash("update 2".toByteArray())
    assertThat(p2PState.getLocalNodeRecordSequenceNumber()).isEqualTo(sequenceNumberAfterInitialization + 2uL)
    assertThat(
      service
        .getLocalNodeRecord()
        .seq
        .toBigInteger()
        .toULong(),
    ).isEqualTo(sequenceNumberAfterInitialization + 2uL)

    service.stop().get()
  }

  @Test
  fun `updateForkIdHash updates local`() {
    val nextForkId = Random.nextBytes(4)
    service.updateForkIdHash(nextForkId)

    assertThat(service.getLocalNodeRecord().get(FORK_ID_HASH_FIELD_NAME))
      .isEqualTo(Bytes.wrap(nextForkId))
  }

  @Test
  fun `discovery finds nodes`() {
    val bootnode =
      MaruDiscoveryService(
        privateKeyBytes = key1,
        p2pConfig = P2PConfig(
          ipAddress = IPV4,
          port = PORT1,
          discovery = P2PConfig.Discovery(
            port = PORT2,
            bootnodes = emptyList(),
            refreshInterval = 5.seconds,
          ),
        ),
        forkIdHashManager = forkIdHashProvider,
        p2PState = InMemoryP2PState(),
        timerFactory = JvmTimerFactory(),
      )

    val discoveryService2 =
      MaruDiscoveryService(
        privateKeyBytes = key2,
        p2pConfig = P2PConfig(
          ipAddress = IPV4,
          port = PORT3,
          discovery = P2PConfig.Discovery(
            port = PORT4,
            bootnodes = listOf(bootnode.getLocalNodeRecord().asEnr()),
            refreshInterval = 500.milliseconds,
          ),
        ),
        forkIdHashManager = forkIdHashProvider,
        p2PState = InMemoryP2PState(),
        timerFactory = JvmTimerFactory(),
      )

    val discoveryService3 =
      MaruDiscoveryService(
        privateKeyBytes = key3,
        p2pConfig = P2PConfig(
          ipAddress = IPV4,
          port = PORT5,
          discovery = P2PConfig.Discovery(
            port = PORT6,
            bootnodes = listOf(bootnode.getLocalNodeRecord().asEnr()),
            refreshInterval = 500.milliseconds,
          ),
        ),
        forkIdHashManager = forkIdHashProvider,
        p2PState = InMemoryP2PState(),
        timerFactory = JvmTimerFactory(),
      )

    try {
      bootnode.start()
      discoveryService2.start()
      discoveryService3.start()

      await
        .timeout(20.seconds.toJavaDuration())
        .untilAsserted {
          val foundPeers =
            discoveryService2
              .searchForPeers()
              .join()

          foundPeersContains(foundPeers, bootnode, discoveryService3)
        }

      await
        .timeout(10.seconds.toJavaDuration())
        .untilAsserted {
          val foundPeers =
            discoveryService3
              .searchForPeers()
              .join()
          foundPeersContains(foundPeers, bootnode, discoveryService2)
        }
    } finally {
      bootnode.stop().get()
      discoveryService2.stop().get()
      discoveryService3.stop().get()
    }
  }

  private fun foundPeersContains(
    foundPeers: Collection<DiscoveryPeer>,
    vararg nodes: MaruDiscoveryService,
  ) {
    nodes.forEach { node -> assertThat(foundPeers.any { it.nodeId == node.getLocalNodeRecord().nodeId }).isTrue }
  }

  @Test
  fun `isValidNodeRecord returns true for valid node record`() {
    val node = createValidNodeRecord()

    val result = isValidNodeRecord(forkIdHashProvider, node)

    assertTrue(result)
  }

  @Test
  fun `isValidNodeRecord returns false when forkId field is missing`() {
    val node = createValidNodeRecord(forkIdHash = null)

    val result = isValidNodeRecord(forkIdHashProvider, node)

    assertThat(result).isFalse()
  }

  @Test
  fun `isValidNodeRecord returns false when address is missing`() {
    val node = createValidNodeRecord(tcpAddress = Optional.empty())

    val result = isValidNodeRecord(forkIdHashProvider, node)

    assertThat(result).isFalse()
  }

  @Test
  fun `local node record uses advertisedIp when set`() {
    val ipAddress = "127.0.0.1"
    val advertisedIp = "203.0.113.50"
    val port = 9001u
    val discoveryPort = 9000u

    val p2pConfig =
      P2PConfig(
        ipAddress = ipAddress,
        port = port,
        discovery = P2PConfig.Discovery(
          port = discoveryPort,
          bootnodes = listOf(),
          refreshInterval = 10.seconds,
          advertisedIp = advertisedIp,
        ),
      )

    val discoveryService =
      MaruDiscoveryService(
        privateKeyBytes = keyPair.secretKey().bytesArray(),
        p2pConfig = p2pConfig,
        forkIdHashManager = forkIdHashProvider,
        p2PState = InMemoryP2PState(),
        timerFactory = JvmTimerFactory(),
      )

    val localNodeRecord = discoveryService.getLocalNodeRecord()

    assertThat(localNodeRecord.udpAddress.isPresent).isTrue()
    assertThat(
      localNodeRecord.udpAddress
        .get()
        .address.hostAddress,
    ).isEqualTo(advertisedIp)
    assertThat(localNodeRecord.udpAddress.get().port).isEqualTo(discoveryPort.toInt())
    assertThat(localNodeRecord.tcpAddress.isPresent).isTrue()
    assertThat(
      localNodeRecord.tcpAddress
        .get()
        .address.hostAddress,
    ).isEqualTo(advertisedIp)
    assertThat(localNodeRecord.tcpAddress.get().port).isEqualTo(port.toInt())
  }
}

/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p

import io.libp2p.core.Connection
import io.libp2p.core.ConnectionHandler
import io.libp2p.core.Host
import io.libp2p.core.PeerId
import io.libp2p.core.crypto.KeyType
import io.libp2p.core.crypto.PrivKey
import io.libp2p.core.crypto.generateKeyPair
import io.libp2p.core.crypto.marshalPrivateKey
import io.libp2p.core.dsl.host
import io.libp2p.core.multiformats.Multiaddr
import io.libp2p.core.mux.StreamMuxer
import io.libp2p.core.mux.StreamMuxerProtocol
import io.libp2p.core.security.SecureChannel
import io.libp2p.security.noise.NoiseXXSecureChannel
import io.libp2p.security.secio.SecIoSecureChannel
import io.libp2p.transport.tcp.TcpTransport
import linea.timer.JvmTimerFactory
import maru.config.P2PConfig
import maru.consensus.ForkIdManagerFactory.createForkIdHashManager
import maru.core.ext.DataGenerators
import maru.core.ext.metrics.TestMetrics
import maru.database.InMemoryBeaconChain
import maru.database.InMemoryP2PState
import maru.p2p.messages.StatusManager
import maru.p2p.testutils.NetworkUtil.findFreePort
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.hyperledger.besu.metrics.noop.NoOpMetricsSystem
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.parallel.Execution
import org.junit.jupiter.api.parallel.ExecutionMode
import tech.pegasys.teku.networking.p2p.libp2p.MultiaddrPeerAddress
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.TimeUnit

@Execution(ExecutionMode.SAME_THREAD)
class TransportCompatibilityTest {
  private companion object {
    const val IPV4 = "127.0.0.1"
    const val CHAIN_ID = 1337u
    const val CONNECTION_TIMEOUT_SECONDS = 30L
  }

  private val startedMaruNetworks = mutableListOf<P2PNetworkImpl>()
  private val startedRemoteHosts = mutableListOf<Host>()
  private val remoteInboundConnections = CopyOnWriteArrayList<Connection>()

  @AfterEach
  fun tearDown() {
    startedRemoteHosts.forEach { host ->
      runCatching { host.stop().get() }
    }
    startedRemoteHosts.clear()

    startedMaruNetworks.forEach { network ->
      runCatching { network.stop().get() }
      runCatching { network.close() }
    }
    startedMaruNetworks.clear()

    remoteInboundConnections.clear()
  }

  @Test
  fun `peer supporting only noise and yamux can dial maru`() {
    val maruNetwork = startMaruNetwork()

    // spawn host that uses noise / yamux
    val remoteHost = startRemoteHost(secureChannel = ::NoiseXXSecureChannel, muxer = StreamMuxerProtocol.getYamux())

    assertDialsMaru(remoteHost = remoteHost, maruNetwork = maruNetwork)
  }

  @Test
  fun `peer supporting only secio and mplex can dial maru`() {
    val maruNetwork = startMaruNetwork()

    // spawn host that uses existing secio / mplex
    val remoteHost = startRemoteHost(secureChannel = ::SecIoSecureChannel, muxer = StreamMuxerProtocol.Mplex)

    assertDialsMaru(remoteHost = remoteHost, maruNetwork = maruNetwork)
  }

  @Test
  fun `maru can dial a peer supporting only noise and yamux`() {
    val maruNetwork = startMaruNetwork()
    val remotePort = findFreePort()
    val remoteHost =
      startRemoteHost(
        secureChannel = ::NoiseXXSecureChannel,
        muxer = StreamMuxerProtocol.getYamux(),
        port = remotePort,
      )

    maruNetwork.addStaticPeer(
      MultiaddrPeerAddress.fromAddress("/ip4/$IPV4/tcp/$remotePort/p2p/${remoteHost.peerId.toBase58()}"),
    )

    await().timeout(CONNECTION_TIMEOUT_SECONDS, TimeUnit.SECONDS).untilAsserted {
      assertThat(remoteInboundConnections).isNotEmpty()
    }
  }

  private fun assertDialsMaru(
    remoteHost: Host,
    maruNetwork: P2PNetworkImpl,
  ) {
    val maruPeerId = PeerId.fromBase58(maruNetwork.nodeId)
    val connection =
      remoteHost.network
        .connect(maruPeerId, Multiaddr("/ip4/$IPV4/tcp/${maruNetwork.port}"))
        .get(CONNECTION_TIMEOUT_SECONDS, TimeUnit.SECONDS)

    assertThat(connection.secureSession().remoteId).isEqualTo(maruPeerId)
  }

  private fun startRemoteHost(
    secureChannel: (PrivKey, List<StreamMuxer>) -> SecureChannel,
    muxer: StreamMuxerProtocol,
    port: UInt = 0u,
  ): Host {
    val remoteHost =
      host {
        identity {
          random(KeyType.SECP256K1)
        }
        transports {
          add(::TcpTransport)
        }
        secureChannels {
          add(secureChannel)
        }
        muxers {
          add(muxer)
        }
        connectionHandlers {
          add(
            object : ConnectionHandler {
              override fun handleConnection(conn: Connection) {
                remoteInboundConnections.add(conn)
              }
            },
          )
        }
        network {
          listen("/ip4/$IPV4/tcp/$port")
        }
      }

    startedRemoteHosts.add(remoteHost)

    remoteHost.start().get(CONNECTION_TIMEOUT_SECONDS, TimeUnit.SECONDS)

    return remoteHost
  }

  private fun startMaruNetwork(): P2PNetworkImpl {
    val (beaconState, sealedBlock) = DataGenerators.genesisState(genesisTimestamp = 0UL)
    val beaconChain = InMemoryBeaconChain(beaconState, sealedBlock)
    val forkIdHashManager = createForkIdHashManager(chainId = CHAIN_ID, beaconChain = beaconChain)

    val network =
      P2PNetworkImpl(
        privateKeyBytes = marshalPrivateKey(generateKeyPair(KeyType.SECP256K1).first),
        p2pConfig = P2PConfig(
          ipAddress = IPV4,
          port = 0u,
        ),
        chainId = CHAIN_ID,
        blockHashing = DataGenerators.testForkAwareBlockHashing(
          chainId = CHAIN_ID,
          validatorSet = setOf(DataGenerators.randomValidator()),
        ),
        metricsFacade = TestMetrics.TestMetricsFacade,
        statusManager = StatusManager(beaconChain, forkIdHashManager),
        beaconChain = beaconChain,
        metricsSystem = NoOpMetricsSystem(),
        forkIdHashManager = forkIdHashManager,
        isBlockImportEnabledProvider = { true },
        p2PState = InMemoryP2PState(),
        timerFactory = JvmTimerFactory(),
      )

    startedMaruNetworks.add(network)

    network.start().get()

    return network
  }
}

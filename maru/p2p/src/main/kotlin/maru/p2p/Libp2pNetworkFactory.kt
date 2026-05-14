/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p

import io.libp2p.core.ConnectionHandler
import io.libp2p.core.Host
import io.libp2p.core.PeerId
import io.libp2p.core.crypto.PrivKey
import io.libp2p.core.dsl.host
import io.libp2p.core.multiformats.Multiaddr
import io.libp2p.core.mux.StreamMuxerProtocol
import io.libp2p.pubsub.PubsubApiImpl
import io.libp2p.pubsub.gossip.Gossip
import io.libp2p.pubsub.gossip.GossipPeerScoreParams
import io.libp2p.pubsub.gossip.GossipScoreParams
import io.libp2p.pubsub.gossip.GossipTopicsScoreParams
import io.libp2p.pubsub.gossip.builders.GossipParamsBuilder
import io.libp2p.pubsub.gossip.builders.GossipRouterBuilder
import io.libp2p.security.secio.SecIoSecureChannel
import io.libp2p.transport.tcp.TcpTransport
import maru.config.P2PConfig
import maru.core.SealedBeaconBlock
import maru.p2p.topics.ImmediateTopicHandler
import maru.p2p.topics.TopicHandlerWithInOrderDelivering
import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.consensus.qbft.core.types.QbftMessage
import pubsub.pb.Rpc
import tech.pegasys.teku.infrastructure.async.AsyncRunner
import tech.pegasys.teku.infrastructure.unsigned.UInt64
import tech.pegasys.teku.networking.p2p.libp2p.LibP2PNetwork
import tech.pegasys.teku.networking.p2p.libp2p.LibP2PNodeId
import tech.pegasys.teku.networking.p2p.libp2p.PeerManager
import tech.pegasys.teku.networking.p2p.libp2p.gossip.GossipTopicHandlers
import tech.pegasys.teku.networking.p2p.libp2p.gossip.LibP2PGossipNetwork
import tech.pegasys.teku.networking.p2p.libp2p.gossip.PreparedPubsubMessage
import tech.pegasys.teku.networking.p2p.libp2p.rpc.RpcHandler
import tech.pegasys.teku.networking.p2p.network.P2PNetwork
import tech.pegasys.teku.networking.p2p.network.PeerHandler
import tech.pegasys.teku.networking.p2p.peer.Peer
import tech.pegasys.teku.networking.p2p.reputation.ReputationManager
import tech.pegasys.teku.networking.p2p.rpc.RpcMethod
import java.util.Optional
import kotlin.random.Random
import kotlin.time.toJavaDuration
import org.hyperledger.besu.plugin.services.MetricsSystem as BesuMetricsSystem

data class TekuLibP2PNetwork(
  val p2PNetwork: P2PNetwork<Peer>,
  val host: Host,
  val peerLookup: PeerLookup,
)

class Libp2pNetworkFactory(
  private val domain: String,
) {
  fun build(
    privateKey: PrivKey,
    port: UInt,
    ipAddress: String,
    sealedBlocksTopicHandler: TopicHandlerWithInOrderDelivering<SealedBeaconBlock>,
    sealedBlocksTopicId: String,
    qbftTopicHandler: ImmediateTopicHandler<QbftMessage>,
    qbftTopicId: String,
    rpcMethods: List<RpcMethod<*, *, *>>,
    maruPeerManager: MaruPeerManager,
    metricsSystem: BesuMetricsSystem,
    asyncRunner: AsyncRunner,
    reputationManager: ReputationManager,
    gossipingConfig: P2PConfig.Gossiping,
  ): TekuLibP2PNetwork {
    val ipv4Address = Multiaddr("/ip4/$ipAddress/tcp/$port")
    val gossipTopicHandlers = GossipTopicHandlers()

    gossipTopicHandlers.add(
      sealedBlocksTopicId,
      sealedBlocksTopicHandler,
    )

    gossipTopicHandlers.add(
      qbftTopicId,
      qbftTopicHandler,
    )

    val gossipParams =
      GossipParamsBuilder()
        .heartbeatInterval(gossipingConfig.heartbeatInterval.toJavaDuration())
        .gossipHistoryLength(gossipingConfig.history)
        .D(gossipingConfig.d)
        .DLow(gossipingConfig.dLow)
        .DLazy(gossipingConfig.dLazy)
        .DHigh(gossipingConfig.dHigh)
        .fanoutTTL(gossipingConfig.fanoutTTL.toJavaDuration())
        .seenTTL(gossipingConfig.seenTTL.toJavaDuration())
        .gossipSize(gossipingConfig.gossipSize)
        .floodPublishMaxMessageSizeThreshold(gossipingConfig.floodPublishMaxMessageSizeThreshold)
        .gossipFactor(gossipingConfig.gossipFactor)
        .build()
    val gossipRouterBuilder =
      GossipRouterBuilder()
        .apply {
          params = gossipParams
          scoreParams = GossipScoreParams(
            GossipPeerScoreParams(isDirect = { _ -> gossipingConfig.considerPeersAsDirect }),
            GossipTopicsScoreParams(),
          )
          messageFactory = { getMessageFactory(it, gossipTopicHandlers) }
        }
    val gossipRouter = gossipRouterBuilder.build()
    val pubsubApiImpl = PubsubApiImpl(gossipRouter)
    val gossip = Gossip(gossipRouter, pubsubApiImpl)

    val publisherApi = gossip.createPublisher(privateKey, Random.nextLong())
    val gossipNetwork = LibP2PGossipNetwork(metricsSystem, gossip, publisherApi, gossipTopicHandlers)

    val peerId = PeerId.fromPubKey(privateKey.publicKey())
    val libP2PNodeId = LibP2PNodeId(peerId)

    val rpcHandlers =
      rpcMethods.map { rpcMethod ->
        RpcHandler(asyncRunner, rpcMethod, metricsSystem)
      }

    val peerManager =
      PeerManager(
        metricsSystem,
        reputationManager,
        listOf<PeerHandler>(maruPeerManager),
        rpcHandlers,
        { gossip.getGossipScore(it) },
      )

    val host =
      createHost(
        privateKey = privateKey,
        connectionHandlers = listOf(gossip, peerManager),
        gossip = gossip,
        rpcHandlers = rpcHandlers,
        ipv4Address = ipv4Address,
      )

    val advertisedAddresses = listOf(ipv4Address)

    val p2pNetwork =
      LibP2PNetwork(
        /* privKey = */
        privateKey,
        /* nodeId = */
        libP2PNodeId,
        /* host = */
        host,
        /* peerManager = */
        peerManager,
        /* advertisedAddresses = */
        advertisedAddresses,
        /* gossipNetwork = */
        gossipNetwork,
        /* listenPorts = */
        listOf(port.toInt()),
      )
    return TekuLibP2PNetwork(p2pNetwork, host, maruPeerManager)
  }

  private fun getMessageFactory(
    msg: Rpc.Message,
    gossipTopicHandlers: GossipTopicHandlers,
  ): PreparedPubsubMessage {
    val arrivalTimestamp = Optional.empty<UInt64>()
    val topic = msg.getTopicIDs(0)
    val payload = Bytes.wrap(msg.data.toByteArray())

    val preparedMessage =
      gossipTopicHandlers
        .getHandlerForTopic(topic)
        .map { handler -> handler.prepareMessage(payload, arrivalTimestamp) }
        .orElse(
          MaruPreparedGossipMessage(
            origMessage = payload,
            arrTimestamp = arrivalTimestamp,
            domain = domain,
            topicId = topic,
          ),
        )

    return PreparedPubsubMessage(msg, preparedMessage)
  }

  private fun createHost(
    privateKey: PrivKey,
    connectionHandlers: List<ConnectionHandler>,
    gossip: Gossip,
    rpcHandlers: List<RpcHandler<*, *, *>>,
    ipv4Address: Multiaddr,
  ): Host =
    host {
      protocols {
        +gossip
        rpcHandlers.forEach { rpcHandler -> add(rpcHandler) }
      }
      network {
        listen(ipv4Address.toString())
      }
      transports {
        add(::TcpTransport)
      }
      identity {
        factory = { privateKey }
      }
      secureChannels {
        add { localKey, muxerProtocols -> SecIoSecureChannel(localKey, muxerProtocols) }
      }
      connectionHandlers {
        connectionHandlers.forEach { handler ->
          add(handler)
        }
      }
      muxers {
        add(StreamMuxerProtocol.Mplex)
      }
    }
}

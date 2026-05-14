/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p

import maru.database.BeaconChain
import maru.p2p.messages.BeaconBlocksByRangeHandler
import maru.p2p.messages.BeaconBlocksByRangeRequestMessageSerDe
import maru.p2p.messages.BeaconBlocksByRangeRequestSerDe
import maru.p2p.messages.BeaconBlocksByRangeResponseMessageSerDe
import maru.p2p.messages.BeaconBlocksByRangeResponseSerDe
import maru.p2p.messages.BlockRetrievalStrategy
import maru.p2p.messages.SizeLimitBlockRetrievalStrategy
import maru.p2p.messages.StatusHandler
import maru.p2p.messages.StatusManager
import maru.p2p.messages.StatusMessageSerDe
import maru.p2p.messages.StatusRequestMessageSerDe
import maru.p2p.messages.StatusSerDe
import maru.serialization.rlp.MaruCompressorRLPSerDe
import maru.serialization.rlp.RLPSerializers

open class RpcMethods(
  statusManager: StatusManager,
  lineaRpcProtocolIdGenerator: LineaRpcProtocolIdGenerator,
  private val peerLookup: () -> PeerLookup,
  beaconChain: BeaconChain,
  blockRetrievalStrategy: BlockRetrievalStrategy = SizeLimitBlockRetrievalStrategy(),
) {
  val statusMessageSerDe = StatusMessageSerDe(StatusSerDe())

  val statusRpcMethod =
    MaruRpcMethod(
      messageType = RpcMessageType.STATUS,
      rpcMessageHandler = StatusHandler(statusManager = statusManager),
      requestMessageSerDe = StatusRequestMessageSerDe(statusMessageSerDe),
      responseMessageSerDe = statusMessageSerDe,
      peerLookup = peerLookup,
      protocolIdGenerator = lineaRpcProtocolIdGenerator,
      version = Version.V1,
      encoding = Encoding.RLP,
    )

  val beaconBlocksByRangeRequestMessageSerDe =
    BeaconBlocksByRangeRequestMessageSerDe(
      beaconBlocksByRangeRequestSerDe = MaruCompressorRLPSerDe(
        serDe = BeaconBlocksByRangeRequestSerDe(),
      ),
    )
  val beaconBlocksByRangeResponseMessageSerDe =
    BeaconBlocksByRangeResponseMessageSerDe(
      beaconBlocksByRangeResponseSerDe = MaruCompressorRLPSerDe(
        serDe = BeaconBlocksByRangeResponseSerDe(RLPSerializers.SealedBeaconBlockSerializer),
      ),
    )

  val beaconBlocksByRangeRpcMethod =
    MaruRpcMethod(
      messageType = RpcMessageType.BEACON_BLOCKS_BY_RANGE,
      rpcMessageHandler = BeaconBlocksByRangeHandler(beaconChain, blockRetrievalStrategy),
      requestMessageSerDe = beaconBlocksByRangeRequestMessageSerDe,
      responseMessageSerDe = beaconBlocksByRangeResponseMessageSerDe,
      peerLookup = peerLookup,
      protocolIdGenerator = lineaRpcProtocolIdGenerator,
      version = Version.V1,
      encoding = Encoding.RLP_SNAPPY,
    )

  fun status() = statusRpcMethod

  open fun beaconBlocksByRange() = beaconBlocksByRangeRpcMethod

  open fun all(): List<MaruRpcMethod<*, *>> = listOf(statusRpcMethod, beaconBlocksByRangeRpcMethod)
}

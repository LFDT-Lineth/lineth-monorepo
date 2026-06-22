/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p

import tech.pegasys.teku.infrastructure.ssz.SszData
import tech.pegasys.teku.infrastructure.ssz.SszMutableData
import tech.pegasys.teku.infrastructure.ssz.schema.SszSchema
import tech.pegasys.teku.infrastructure.ssz.tree.TreeNode
import tech.pegasys.teku.spec.datastructures.networking.libp2p.rpc.RpcRequest

enum class Version : Comparable<Version> {
  V1,
}

enum class GossipMessageType : MessageType {
  QBFT,
  BEACON_BLOCK,
}

enum class RpcMessageType : MessageType {
  STATUS,
  BEACON_BLOCKS_BY_RANGE,
}

enum class Encoding {
  RLP,
  RLP_SNAPPY,
}

sealed interface MessageType

interface Message<TPayload, TMessageType : MessageType> {
  val type: TMessageType
  val version: Version
  val payload: TPayload
}

data class MessageData<TPayload, TMessageType : MessageType>(
  override val type: TMessageType,
  override val version: Version = Version.V1,
  override val payload: TPayload,
) : Message<TPayload, TMessageType>

data class RequestMessageAdapter<TPayload, TMessageType : MessageType>(
  val message: Message<TPayload, TMessageType>,
) : RpcRequest,
  Message<TPayload, TMessageType> by message {
  override fun getMaximumResponseChunks(): Int {
    TODO("Not yet implemented")
  }

  override fun createWritableCopy(): SszMutableData {
    TODO("Not yet implemented")
  }

  override fun getSchema(): SszSchema<out SszData?> {
    TODO("Not yet implemented")
  }

  override fun getBackingNode(): TreeNode {
    TODO("Not yet implemented")
  }
}

interface MessageIdGenerator {
  fun id(
    messageName: String,
    version: Version,
    encoding: Encoding = Encoding.RLP,
  ): String
}

class LineaMessageIdGenerator(
  private val chainId: UInt,
) : MessageIdGenerator {
  override fun id(
    messageName: String,
    version: Version,
    encoding: Encoding,
  ): String = "/linea/$chainId/${messageName.lowercase()}/$version/${encoding.name.lowercase()}"
}

class LineaRpcProtocolIdGenerator(
  private val chainId: UInt,
) : MessageIdGenerator {
  override fun id(
    messageName: String,
    version: Version,
    encoding: Encoding,
  ): String = "/linea/req/$chainId/${messageName.lowercase()}/$version/${encoding.name.lowercase()}"
}

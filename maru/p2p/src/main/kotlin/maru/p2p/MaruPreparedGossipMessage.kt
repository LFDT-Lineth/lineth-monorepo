/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p

import maru.crypto.Hashing
import org.apache.tuweni.bytes.Bytes
import tech.pegasys.teku.infrastructure.unsigned.UInt64
import tech.pegasys.teku.networking.p2p.gossip.PreparedGossipMessage
import tech.pegasys.teku.spec.logic.common.helpers.MathHelpers
import java.util.Optional

class MaruPreparedGossipMessage(
  private val origMessage: Bytes,
  private val arrTimestamp: Optional<UInt64>,
  private val domain: String,
  private val topicId: String,
) : PreparedGossipMessage {
  companion object {
  }

  // It's for deduplication of messages
  // topic length comes from Altair consensus spec
  // https://github.com/ethereum/consensus-specs/blob/dev/specs/altair/p2p-interface.md#the-gossip-domain-gossipsub
  override fun getMessageId(): Bytes =
    Bytes.wrap(
      Hashing.shortShaHash(
        domain.toByteArray() + encodeTopicLength(topicId.toByteArray()).toArray() +
          topicId.toByteArray() + origMessage.toArray(),
      ),
    )

  override fun getDecodedMessage(): PreparedGossipMessage.DecodedMessageResult =
    PreparedGossipMessage.DecodedMessageResult.successful(origMessage)

  override fun getOriginalMessage(): Bytes = origMessage

  override fun getArrivalTimestamp(): Optional<UInt64> = arrTimestamp

  private fun encodeTopicLength(topicBytes: ByteArray): Bytes = MathHelpers.uint64ToBytes(topicBytes.size.toLong())
}

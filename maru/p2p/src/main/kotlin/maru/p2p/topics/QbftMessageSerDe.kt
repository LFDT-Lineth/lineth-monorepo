/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.topics

import maru.serialization.rlp.RLPSerDe
import org.hyperledger.besu.consensus.qbft.core.types.QbftMessage
import org.hyperledger.besu.ethereum.rlp.RLPInput
import org.hyperledger.besu.ethereum.rlp.RLPOutput
import org.hyperledger.besu.ethereum.p2p.rlpx.wire.MessageData as BesuMessageData

class QbftMessageSerDe : RLPSerDe<QbftMessage> {
  private val besuMessageDataSerDe = BesuMessageDataSerDe()

  override fun writeTo(
    value: QbftMessage,
    rlpOutput: RLPOutput,
  ) {
    besuMessageDataSerDe.writeTo(value.data, rlpOutput)
  }

  override fun readFrom(rlpInput: RLPInput): QbftMessage {
    val messageData = besuMessageDataSerDe.readFrom(rlpInput)
    return MaruQbftMessage(messageData)
  }

  internal class MaruQbftMessage(
    private val messageData: BesuMessageData,
  ) : QbftMessage {
    override fun getData(): BesuMessageData = messageData

    override fun toString(): String = "QbftMessage(data=$messageData)"
  }
}

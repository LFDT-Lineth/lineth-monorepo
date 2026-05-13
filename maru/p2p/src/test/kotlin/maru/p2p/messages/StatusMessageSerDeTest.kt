/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.messages

import maru.core.ext.DataGenerators.randomStatus
import maru.p2p.MessageData
import maru.p2p.RequestMessageAdapter
import maru.p2p.RpcMessageType
import maru.p2p.Version
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class StatusMessageSerDeTest {
  private val responseMessageSerDe = StatusMessageSerDe(StatusSerDe())
  private val requestMessageSerDe = StatusRequestMessageSerDe(responseMessageSerDe)

  @Test
  fun `can serialize and deserialize same request value`() {
    val testValue =
      RequestMessageAdapter(
        MessageData(
          RpcMessageType.STATUS,
          Version.V1,
          randomStatus(100U),
        ),
      )

    val serializedData = requestMessageSerDe.serialize(testValue)
    val deserializedValue = requestMessageSerDe.deserialize(serializedData)

    assertThat(deserializedValue).isEqualTo(testValue)
  }

  @Test
  fun `can serialize and deserialize same response value`() {
    val testValue =
      MessageData(
        RpcMessageType.STATUS,
        Version.V1,
        randomStatus(100U),
      )

    val serializedData = responseMessageSerDe.serialize(testValue)
    val deserializedValue = responseMessageSerDe.deserialize(serializedData)

    assertThat(deserializedValue).isEqualTo(testValue)
  }
}

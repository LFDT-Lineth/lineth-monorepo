/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.messages

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import tech.pegasys.teku.networking.eth2.rpc.core.RpcException
import tech.pegasys.teku.networking.eth2.rpc.core.RpcResponseStatus
import kotlin.random.Random

class RpcExceptionSerDeTest {
  private val serDe = RpcExceptionSerDe()

  @Test
  fun `can serialize and deserialize same RpcException`() {
    val testValue =
      RpcException(
        RpcResponseStatus.SERVER_ERROR_CODE,
        "test for serialize and deserialize",
      )

    val serializedData = serDe.serialize(testValue)
    val deserializedValue = serDe.deserialize(serializedData)

    assertThat(deserializedValue).isEqualTo(testValue)
  }

  @Test
  fun `can serialize and deserialize same RpcException with error message length capped at 256`() {
    val errorMessage = Random.nextBytes(130).toHexString()
    val testValue =
      RpcException(
        RpcResponseStatus.SERVER_ERROR_CODE,
        errorMessage,
      )
    // The length of RpcException errorMessage should be capped at 256
    val expectedValue =
      RpcException(
        RpcResponseStatus.SERVER_ERROR_CODE,
        errorMessage.slice(0..RpcException.MAXIMUM_ERROR_MESSAGE_LENGTH - 1),
      )

    val serializedData = serDe.serialize(testValue)
    val deserializedValue = serDe.deserialize(serializedData)

    assertThat(deserializedValue).isEqualTo(expectedValue)
  }

  @Test
  fun `can serialize and deserialize same RpcException with empty error message`() {
    val testValue = RpcException(RpcResponseStatus.SERVER_ERROR_CODE, "")

    val serializedData = serDe.serialize(testValue)
    val deserializedValue = serDe.deserialize(serializedData)

    assertThat(deserializedValue).isEqualTo(testValue)
  }
}

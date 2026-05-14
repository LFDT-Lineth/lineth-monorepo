/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.serialization.rlp

import maru.core.ext.DataGenerators.randomExecutionPayload
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class ExecutionPayloadSerializerTest {
  private val serializer = ExecutionPayloadSerDe()

  @Test
  fun `can serialize and deserialize same value`() {
    val testValue = randomExecutionPayload()
    val serializedData = serializer.serialize(testValue)
    val deserializedValue = serializer.deserialize(serializedData)

    assertThat(deserializedValue).isEqualTo(testValue)
  }

  @Test
  fun `can serialize and deserialize execution payload with zero transactions`() {
    val testValue = randomExecutionPayload()
    val serializedData = serializer.serialize(testValue)
    val deserializedValue = serializer.deserialize(serializedData)

    assertThat(deserializedValue).isEqualTo(testValue)
  }
}

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
import org.apache.tuweni.bytes.Bytes
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.hyperledger.besu.ethereum.rlp.RLP
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

  @Test
  fun `Amsterdam fields survive serialization with unsigned slot`() {
    val payload = randomExecutionPayload().copy(
      blockAccessList = byteArrayOf(0xc0.toByte()),
      slotNumber = ULong.MAX_VALUE,
    )
    val decoded = serializer.deserialize(serializer.serialize(payload))
    assertThat(decoded).isEqualTo(payload)
    assertThat(decoded.blockAccessList).containsExactly(0xc0.toByte())
    assertThat(decoded.slotNumber).isEqualTo(ULong.MAX_VALUE)
  }

  @Test
  fun `legacy payload encoding remains a fourteen field list`() {
    val payload = randomExecutionPayload()
    val encoded = serializer.serialize(payload)
    val input = RLP.input(Bytes.wrap(encoded))
    input.enterList()
    repeat(14) { input.skipNext() }
    assertThat(input.isEndOfCurrentList).isTrue()
    input.leaveList()
    val decoded = serializer.deserialize(encoded)
    assertThat(decoded.blockAccessList).isNull()
    assertThat(decoded.slotNumber).isNull()
    assertThat(serializer.serialize(decoded)).containsExactly(*encoded)
  }

  @Test
  fun `partial Amsterdam extension is rejected`() {
    val encoded = serializer.serialize(randomExecutionPayload())
    val input = RLP.input(Bytes.wrap(encoded))
    input.enterList()
    val partial = RLP.encode { output ->
      output.startList()
      repeat(14) { output.writeRLPBytes(input.readAsRlp().raw()) }
      output.writeBytes(Bytes.of(0xc0))
      output.endList()
    }
    assertThatThrownBy { serializer.deserialize(partial.toArray()) }
      .isInstanceOf(RuntimeException::class.java)
  }
}

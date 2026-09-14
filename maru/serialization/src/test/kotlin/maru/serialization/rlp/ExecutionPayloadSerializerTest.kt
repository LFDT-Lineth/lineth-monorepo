/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.serialization.rlp

import maru.core.ExecutionPayload
import maru.core.ext.DataGenerators.randomExecutionPayload
import org.apache.tuweni.bytes.Bytes
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import java.math.BigInteger

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
    val testValue = randomExecutionPayload(numberOfTransactions = 0)
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
  }

  @Test
  fun `pre-Amsterdam payloads retain their original wire encoding`() {
    val payload = ExecutionPayload(
      parentHash = ByteArray(32) { 1 },
      feeRecipient = ByteArray(20) { 2 },
      stateRoot = ByteArray(32) { 3 },
      receiptsRoot = ByteArray(32) { 4 },
      logsBloom = ByteArray(256) { 5 },
      prevRandao = ByteArray(32) { 6 },
      blockNumber = 42UL,
      gasLimit = 60_000_000UL,
      gasUsed = 21_000UL,
      timestamp = 12345UL,
      extraData = byteArrayOf(8, 9),
      baseFeePerGas = BigInteger.valueOf(1_000_000_000),
      blockHash = ByteArray(32) { 7 },
      transactions = listOf(byteArrayOf(1, 2), byteArrayOf(3)),
    )
    val legacyEncoding = Bytes.fromHexString(
      requireNotNull(javaClass.getResource("/execution-payload-pre-amsterdam.rlp.hex")).readText().trim(),
    ).toArray()

    assertThat(serializer.deserialize(legacyEncoding)).isEqualTo(payload)
    assertThat(serializer.serialize(payload)).containsExactly(*legacyEncoding)
  }
}

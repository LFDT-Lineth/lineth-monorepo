/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer.mappers

import maru.core.ext.DataGenerators.randomExecutionPayload
import maru.executionlayer.manager.PayloadAttributes
import maru.executionlayer.mappers.Mappers.toDomainExecutionPayload
import maru.executionlayer.mappers.Mappers.toExecutionPayloadV4
import maru.executionlayer.mappers.Mappers.toPayloadAttributesV1
import maru.executionlayer.mappers.Mappers.toPayloadAttributesV4
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import tech.pegasys.teku.infrastructure.unsigned.UInt64

class AmsterdamMappersTest {
  @Test
  fun `Amsterdam payload mapping preserves access list and unsigned slot`() {
    val payload = randomExecutionPayload().copy(
      blockAccessList = byteArrayOf(0xc1.toByte(), 0xc0.toByte()),
      slotNumber = ULong.MAX_VALUE,
    )
    val wirePayload = payload.toExecutionPayloadV4()
    assertThat(wirePayload.blockAccessList.toArray()).containsExactly(*payload.blockAccessList!!)
    assertThat(wirePayload.slotNumber).isEqualTo(UInt64.MAX_VALUE)
    val decoded = wirePayload.toDomainExecutionPayload()
    assertThat(decoded).isEqualTo(payload)
    assertThat(decoded.blockAccessList).containsExactly(*payload.blockAccessList!!)
    assertThat(decoded.slotNumber).isEqualTo(ULong.MAX_VALUE)
  }

  @Test
  fun `Amsterdam payload mapping rejects missing fork fields`() {
    assertThatThrownBy { randomExecutionPayload().toExecutionPayloadV4() }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("blockAccessList")
  }

  @Test
  fun `Amsterdam attributes use supplied slot and target and reject absent values`() {
    val attributes = PayloadAttributes(
      timestamp = 123UL,
      suggestedFeeRecipient = ByteArray(20),
      slotNumber = ULong.MAX_VALUE,
      targetGasLimit = 60_000_000UL,
    )
    assertThat(attributes.toPayloadAttributesV4().slotNumber).isEqualTo(UInt64.MAX_VALUE)
    assertThat(attributes.toPayloadAttributesV4().targetGasLimit).isEqualTo(UInt64.valueOf(60_000_000))
    assertThatThrownBy { attributes.copy(slotNumber = null).toPayloadAttributesV4() }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("slotNumber")
    assertThatThrownBy { attributes.copy(targetGasLimit = null).toPayloadAttributesV4() }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("targetGasLimit")
    assertThat(attributes.copy(targetGasLimit = ULong.MAX_VALUE).toPayloadAttributesV4().targetGasLimit)
      .isEqualTo(UInt64.MAX_VALUE)
  }

  @Test
  fun `legacy attributes ignore Amsterdam fields`() {
    val attributes = PayloadAttributes(timestamp = 123UL, suggestedFeeRecipient = ByteArray(20))
    assertThat(attributes.copy(slotNumber = 10UL, targetGasLimit = 60_000_000UL).toPayloadAttributesV1())
      .isEqualTo(attributes.toPayloadAttributesV1())
  }
}

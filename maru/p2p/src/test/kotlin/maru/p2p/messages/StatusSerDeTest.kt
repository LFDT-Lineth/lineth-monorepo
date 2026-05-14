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
import kotlin.random.Random
import kotlin.random.nextULong

class StatusSerDeTest {
  private val serDe = StatusSerDe()

  @Test
  fun `can serialize and deserialize same value`() {
    val testValue = Status(Random.nextBytes(32), Random.nextBytes(32), Random.nextULong())

    val serializedData = serDe.serialize(testValue)
    val deserializedValue = serDe.deserialize(serializedData)
    assertThat(deserializedValue).isEqualTo(testValue)
  }
}

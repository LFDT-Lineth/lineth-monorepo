/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.serialization.rlp

import maru.core.Seal
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.random.Random

class SealSerializerTest {
  private val serializer = SealSerDe()

  @Test
  fun `can serialize and deserialize same value`() {
    val testValue = Seal(Random.nextBytes(128))
    val serializedData = serializer.serialize(testValue)
    val deserializedValue = serializer.deserialize(serializedData)

    assertThat(deserializedValue).isEqualTo(testValue)
  }
}

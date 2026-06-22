/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.serialization.rlp

import maru.core.BeaconBlockBody
import maru.core.Seal
import maru.core.ext.DataGenerators.randomExecutionPayload
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.random.Random

class BeaconBlockBodySerializerTest {
  private val serializer =
    BeaconBlockBodySerDe(
      sealSerializer = SealSerDe(),
      executionPayloadSerializer = ExecutionPayloadSerDe(),
    )

  @Test
  fun `can serialize and deserialize same value`() {
    val testValue =
      BeaconBlockBody(
        prevCommitSeals = buildSet(3) {
          add(Seal(Random.nextBytes(96)))
        },
        executionPayload = randomExecutionPayload(),
      )
    val serializedData = serializer.serialize(testValue)
    val deserializedValue = serializer.deserialize(serializedData)

    assertThat(deserializedValue).isEqualTo(testValue)
  }
}

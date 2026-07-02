/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.serialization.rlp

import maru.core.ext.DataGenerators
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.random.Random
import kotlin.random.nextULong

class BeaconBlockHeaderSerializerTest {
  private val validatorSerializer = ValidatorSerDe()
  private val serializer =
    BeaconBlockHeaderSerDe(
      beaconBlockHeaderRLPSerializer = BeaconBlockHeaderRLPSerializer(validatorSerializer),
      validatorSerializer = validatorSerializer,
      headerHashFunction = HashUtil::headerHash,
    )

  @Test
  fun `can serialize and deserialize same value`() {
    val testValue =
      DataGenerators.randomBeaconBlockHeader(
        Random.nextULong(),
      )
    val serializedData = serializer.serialize(testValue)
    val deserializedValue = serializer.deserialize(serializedData)

    assertThat(deserializedValue).isEqualTo(testValue)
  }
}

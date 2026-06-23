/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.serialization.rlp

import maru.core.BeaconBlock
import maru.core.BeaconBlockBody
import maru.core.Seal
import maru.core.SealedBeaconBlock
import maru.core.ext.DataGenerators
import maru.core.ext.DataGenerators.randomExecutionPayload
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.random.Random
import kotlin.random.nextULong

class MaruCompressorRLPSerDeTest {
  private val compressorRLPSerDe =
    MaruCompressorRLPSerDe(
      serDe = RLPSerializers.SealedBeaconBlockSerializer,
    )

  @Test
  fun `can serialize and deserialize same value`() {
    val beaconBlockHeader =
      DataGenerators.randomBeaconBlockHeader(
        Random.nextULong(),
      )
    val beaconBlockBody =
      BeaconBlockBody(
        prevCommitSeals = buildSet(3) { add(Seal(Random.nextBytes(96))) },
        executionPayload = randomExecutionPayload(),
      )
    val sealedBlock =
      SealedBeaconBlock(
        beaconBlock = BeaconBlock(
          beaconBlockHeader = beaconBlockHeader,
          beaconBlockBody = beaconBlockBody,
        ),
        commitSeals = buildSet(3) { add(Seal(Random.nextBytes(96))) },
      )
    val serializedData = compressorRLPSerDe.serialize(sealedBlock)
    val deserializedValue = compressorRLPSerDe.deserialize(serializedData)

    assertThat(deserializedValue).isEqualTo(sealedBlock)
  }
}

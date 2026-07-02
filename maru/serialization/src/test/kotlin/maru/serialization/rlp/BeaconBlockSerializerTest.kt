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
import maru.core.ext.DataGenerators
import maru.core.ext.DataGenerators.randomExecutionPayload
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.random.Random
import kotlin.random.nextULong

class BeaconBlockSerializerTest {
  private val validatorSerializer = ValidatorSerDe()
  private val beaconBlockHeaderRLPSerializer = BeaconBlockHeaderRLPSerializer(validatorSerializer)
  private val blockHeaderSerializer =
    BeaconBlockHeaderSerDe(
      beaconBlockHeaderRLPSerializer = beaconBlockHeaderRLPSerializer,
      validatorSerializer = validatorSerializer,
      beaconBlockIdHashFunction = HashUtil::headerHash,
    )
  private val blockBodySerializer =
    BeaconBlockBodySerDe(
      sealSerializer = SealSerDe(),
      executionPayloadSerializer = ExecutionPayloadSerDe(),
    )
  private val blockSerializer =
    BeaconBlockSerDe(
      beaconBlockRLPSerializer = BeaconBlockRLPSerializer(beaconBlockHeaderRLPSerializer, blockBodySerializer),
      beaconBlockHeaderSerializer = blockHeaderSerializer,
      beaconBlockBodySerializer = blockBodySerializer,
    )

  @Test
  fun `can serialize and deserialize same value`() {
    val beaconBLockHeader =
      DataGenerators.randomBeaconBlockHeader(
        Random.nextULong(),
      )
    val beaconBlockBody =
      BeaconBlockBody(
        prevCommitSeals = buildSet(3) { add(Seal(Random.nextBytes(96))) },
        executionPayload = randomExecutionPayload(),
      )
    val testValue =
      BeaconBlock(
        beaconBlockHeader = beaconBLockHeader,
        beaconBlockBody = beaconBlockBody,
      )
    val serializedData = blockSerializer.serialize(testValue)
    val deserializedValue = blockSerializer.deserialize(serializedData)

    assertThat(deserializedValue).isEqualTo(testValue)
  }
}

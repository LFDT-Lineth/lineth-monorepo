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

class SealedBeaconBlockSerializerTest {
  private val validatorSerializer = ValidatorSerDe()
  private val beaconBlockHeaderRLPSerializer = BeaconBlockHeaderRLPSerializer(validatorSerializer)
  private val blockHeaderSerializer =
    BeaconBlockHeaderSerDe(
      beaconBlockHeaderRLPSerializer = beaconBlockHeaderRLPSerializer,
      validatorSerializer = validatorSerializer,
      headerHashFunction = HashUtil::headerHash,
    )
  private val sealSerializer = SealSerDe()
  private val blockBodySerializer =
    BeaconBlockBodySerDe(
      sealSerializer = sealSerializer,
      executionPayloadSerializer = ExecutionPayloadSerDe(),
    )
  private val beaconBlockRLPSerializer = BeaconBlockRLPSerializer(beaconBlockHeaderRLPSerializer, blockBodySerializer)
  private val blockSerializer =
    BeaconBlockSerDe(
      beaconBlockRLPSerializer = beaconBlockRLPSerializer,
      beaconBlockHeaderSerializer = blockHeaderSerializer,
      beaconBlockBodySerializer = blockBodySerializer,
    )
  private val sealedBlockSerializer =
    SealedBeaconBlockSerDe(
      sealedBeaconBlockRLPSerializer = SealedBeaconBlockRLPSerializer(beaconBlockRLPSerializer, sealSerializer),
      beaconBlockSerializer = blockSerializer,
      sealSerializer = sealSerializer,
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
    val serializedData = sealedBlockSerializer.serialize(sealedBlock)
    val deserializedValue = sealedBlockSerializer.deserialize(serializedData)

    assertThat(deserializedValue).isEqualTo(sealedBlock)
  }
}

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
import maru.core.HashUtil
import maru.core.Seal
import maru.core.SealedBeaconBlock
import maru.core.ext.DataGenerators
import maru.core.ext.DataGenerators.randomExecutionPayload
import maru.crypto.Hashing
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.random.Random
import kotlin.random.nextULong

class SealedBeaconBlockSerializerTest {
  private val blockHeaderSerializer =
    BeaconBlockHeaderSerDe(
      validatorSerializer = ValidatorSerDe(),
      hasher = Hashing::keccak,
      headerHashFunction = HashUtil::headerHash,
    )
  private val sealSerializer = SealSerDe()
  private val blockSerializer =
    BeaconBlockSerDe(
      beaconBlockHeaderSerializer = blockHeaderSerializer,
      beaconBlockBodySerializer = BeaconBlockBodySerDe(
        sealSerializer = sealSerializer,
        executionPayloadSerializer = ExecutionPayloadSerDe(),
      ),
    )
  private val sealedBlockSerializer =
    SealedBeaconBlockSerDe(
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

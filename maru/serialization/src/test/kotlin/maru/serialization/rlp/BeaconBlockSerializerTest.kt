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
import maru.core.ext.DataGenerators
import maru.core.ext.DataGenerators.randomExecutionPayload
import maru.crypto.Hashing
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.random.Random
import kotlin.random.nextULong

class BeaconBlockSerializerTest {
  private val blockHeaderSerializer =
    BeaconBlockHeaderSerDe(
      validatorSerializer = ValidatorSerDe(),
      hasher = Hashing::keccak,
      headerHashFunction = HashUtil::headerHash,
    )
  private val blockBodySerializer =
    BeaconBlockSerDe(
      beaconBlockHeaderSerializer = blockHeaderSerializer,
      beaconBlockBodySerializer = BeaconBlockBodySerDe(
        sealSerializer = SealSerDe(),
        executionPayloadSerializer = ExecutionPayloadSerDe(),
      ),
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
    val serializedData = blockBodySerializer.serialize(testValue)
    val deserializedValue = blockBodySerializer.deserialize(serializedData)

    assertThat(deserializedValue).isEqualTo(testValue)
  }
}

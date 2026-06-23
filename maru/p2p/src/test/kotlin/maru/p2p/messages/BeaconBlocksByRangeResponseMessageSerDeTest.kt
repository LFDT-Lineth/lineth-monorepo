/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.messages

import maru.core.ext.DataGenerators
import maru.p2p.MessageData
import maru.p2p.RpcMessageType
import maru.p2p.Version
import maru.serialization.rlp.RLPSerializers
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class BeaconBlocksByRangeResponseMessageSerDeTest {
  @Test
  fun `response message serDe serializes and deserializes correctly`() {
    val messageSerDe =
      BeaconBlocksByRangeResponseMessageSerDe(
        beaconBlocksByRangeResponseSerDe = BeaconBlocksByRangeResponseSerDe(RLPSerializers.SealedBeaconBlockSerializer),
      )

    val response =
      BeaconBlocksByRangeResponse(
        blocks = listOf(
          DataGenerators.randomSealedBeaconBlock(
            number = 5UL,
          ),
        ),
      )
    val message =
      MessageData(
        type = RpcMessageType.BEACON_BLOCKS_BY_RANGE,
        version = Version.V1,
        payload = response,
      )

    val serialized = messageSerDe.serialize(message)
    val deserialized = messageSerDe.deserialize(serialized)

    assertThat(deserialized.type).isEqualTo(RpcMessageType.BEACON_BLOCKS_BY_RANGE)
    assertThat(deserialized.version).isEqualTo(Version.V1)
    assertThat(deserialized.payload.blocks).hasSize(1)
    assertThat(deserialized.payload).isEqualTo(response)
  }
}

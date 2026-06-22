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

class BeaconBlocksByRangeRequestSerDeTest {
  private val serDe =
    BeaconBlocksByRangeRequestSerDe()

  @Test
  fun `request serDe serializes and deserializes correctly`() {
    val request =
      BeaconBlocksByRangeRequest(
        startBlockNumber = 200UL,
        count = 10UL,
      )
    val serialized = serDe.serialize(request)
    val deserialized = serDe.deserialize(serialized)

    assertThat(deserialized).isEqualTo(request)
  }
}

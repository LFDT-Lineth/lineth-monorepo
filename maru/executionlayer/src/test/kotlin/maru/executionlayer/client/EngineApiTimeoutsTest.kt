/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer.client

import maru.consensus.ElFork
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds

class EngineApiTimeoutsTest {
  @Test
  fun `defaults match the Teku client for every fork's method versions`() {
    ElFork.entries.map(EngineApiForkMethods::of).forEach { methods ->
      assertThat(EngineApiTimeouts.defaultFor(methods.getPayload)).isEqualTo(2.seconds)
      assertThat(EngineApiTimeouts.defaultFor(methods.newPayload)).isEqualTo(8.seconds)
      assertThat(EngineApiTimeouts.defaultFor(methods.forkChoiceUpdated)).isEqualTo(8.seconds)
    }
    assertThat(EngineApiTimeouts.defaultFor("eth_getBlockByNumber")).isEqualTo(1.seconds)
  }

  @Test
  fun `configured timeout caps but never extends the defaults`() {
    val longCap = EngineApiTimeouts.provider(maxTimeout = 1.minutes)
    assertThat(longCap("engine_getPayloadV4")).isEqualTo(2.seconds)
    assertThat(longCap("engine_newPayloadV4")).isEqualTo(8.seconds)
    assertThat(longCap("eth_getBlockByNumber")).isEqualTo(1.seconds)

    val shortCap = EngineApiTimeouts.provider(maxTimeout = 500.milliseconds)
    assertThat(shortCap("engine_getPayloadV4")).isEqualTo(500.milliseconds)
    assertThat(shortCap("engine_forkchoiceUpdatedV3")).isEqualTo(500.milliseconds)

    assertThat(EngineApiTimeouts.provider(maxTimeout = null)("engine_getPayloadV4")).isEqualTo(2.seconds)
  }
}

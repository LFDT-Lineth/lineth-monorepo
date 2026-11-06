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
import kotlin.time.Duration.Companion.seconds

class EngineApiTimeoutsTest {
  @Test
  fun `spec defaults apply to every fork's method versions`() {
    ElFork.entries.map(EngineApiForkMethods::of).forEach { methods ->
      assertThat(EngineApiTimeouts.specDefault(methods.getPayload)).isEqualTo(1.seconds)
      assertThat(EngineApiTimeouts.specDefault(methods.newPayload)).isEqualTo(8.seconds)
      assertThat(EngineApiTimeouts.specDefault(methods.forkChoiceUpdated)).isEqualTo(8.seconds)
    }
    assertThat(EngineApiTimeouts.specDefault("eth_getBlockByNumber")).isEqualTo(8.seconds)
    assertThat(EngineApiTimeouts.specDefault("engine_getPayloadBodiesByHashV1")).isEqualTo(8.seconds)
  }

  @Test
  fun `configured timeout overrides every method`() {
    val provider = EngineApiTimeouts.provider(override = 30.seconds)

    assertThat(provider("engine_getPayloadV4")).isEqualTo(30.seconds)
    assertThat(provider("eth_getBlockByNumber")).isEqualTo(30.seconds)
    assertThat(EngineApiTimeouts.provider(override = null)("engine_getPayloadV4")).isEqualTo(1.seconds)
  }
}

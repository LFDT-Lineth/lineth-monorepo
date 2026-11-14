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
  fun `configured timeout replaces every default, whether longer or shorter`() {
    val longer = EngineApiTimeouts.provider(override = 1.minutes)
    assertThat(longer("engine_getPayloadV4")).isEqualTo(1.minutes)
    assertThat(longer("engine_forkchoiceUpdatedV3")).isEqualTo(1.minutes)
    assertThat(longer("eth_getBlockByNumber")).isEqualTo(1.minutes)

    val shorter = EngineApiTimeouts.provider(override = 500.milliseconds)
    assertThat(shorter("engine_getPayloadV4")).isEqualTo(500.milliseconds)
    assertThat(shorter("engine_newPayloadV4")).isEqualTo(500.milliseconds)
  }

  @Test
  fun `no configured timeout keeps the defaults`() {
    val defaults = EngineApiTimeouts.provider(override = null)
    assertThat(defaults("engine_getPayloadV4")).isEqualTo(2.seconds)
    assertThat(defaults("engine_forkchoiceUpdatedV3")).isEqualTo(8.seconds)
    assertThat(defaults("eth_getBlockByNumber")).isEqualTo(1.seconds)
  }
}

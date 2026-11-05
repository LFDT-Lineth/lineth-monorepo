/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer.client

import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

/**
 * Request timeouts from https://github.com/ethereum/execution-apis/blob/main/src/engine/common.md#timeouts.
 * Methods the spec does not cover, such as `eth_getBlockByNumber`, use [UNSPECIFIED_METHOD_TIMEOUT].
 */
object EngineApiTimeouts {
  val GET_PAYLOAD: Duration = 1.seconds
  val NEW_PAYLOAD: Duration = 8.seconds
  val FORK_CHOICE_UPDATED: Duration = 8.seconds
  val UNSPECIFIED_METHOD_TIMEOUT: Duration = 8.seconds

  private val getPayloadMethod = Regex("engine_getPayloadV\\d+")
  private val newPayloadMethod = Regex("engine_newPayloadV\\d+")
  private val forkChoiceUpdatedMethod = Regex("engine_forkchoiceUpdatedV\\d+")

  fun specDefault(method: String): Duration =
    when {
      getPayloadMethod.matches(method) -> GET_PAYLOAD
      newPayloadMethod.matches(method) -> NEW_PAYLOAD
      forkChoiceUpdatedMethod.matches(method) -> FORK_CHOICE_UPDATED
      else -> UNSPECIFIED_METHOD_TIMEOUT
    }

  /** @param override when set, applies to every method instead of the spec defaults. */
  fun provider(override: Duration?): (String) -> Duration = { method -> override ?: specDefault(method) }
}

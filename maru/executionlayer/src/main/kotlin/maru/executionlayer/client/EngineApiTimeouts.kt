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
 * Per-method request timeouts, matching the Teku execution client Maru used before (`Web3JExecutionEngineClient`).
 *
 * They are compatible with the Engine API spec (https://github.com/ethereum/execution-apis/tree/main/src/engine), which
 * sets the timeout of each method next to its definition, for example
 * https://github.com/ethereum/execution-apis/blob/main/src/engine/paris.md#engine_newpayloadv1:
 * - `engine_newPayload` and `engine_forkchoiceUpdated` specify 8s (6s for `engine_newPayloadV5`; the spec allows
 *   waiting longer).
 * - `engine_getPayload` has no specified timeout and must be served immediately; Teku allows 2s.
 * - Other methods, such as `eth_getBlockByNumber`, are not Engine API block-execution calls; Teku allows 1s.
 */
object EngineApiTimeouts {
  val GET_PAYLOAD: Duration = 2.seconds
  val NEW_PAYLOAD: Duration = 8.seconds
  val FORK_CHOICE_UPDATED: Duration = 8.seconds
  val NON_BLOCK_EXECUTION: Duration = 1.seconds

  private val getPayloadMethod = Regex("engine_getPayloadV\\d+")
  private val newPayloadMethod = Regex("engine_newPayloadV\\d+")
  private val forkChoiceUpdatedMethod = Regex("engine_forkchoiceUpdatedV\\d+")

  fun defaultFor(method: String): Duration =
    when {
      getPayloadMethod.matches(method) -> GET_PAYLOAD
      newPayloadMethod.matches(method) -> NEW_PAYLOAD
      forkChoiceUpdatedMethod.matches(method) -> FORK_CHOICE_UPDATED
      else -> NON_BLOCK_EXECUTION
    }

  /** @param maxTimeout when set, caps every method's default, so it can shorten but never extend a timeout. */
  fun provider(maxTimeout: Duration?): (String) -> Duration =
    { method -> maxTimeout?.let { minOf(defaultFor(method), it) } ?: defaultFor(method) }
}

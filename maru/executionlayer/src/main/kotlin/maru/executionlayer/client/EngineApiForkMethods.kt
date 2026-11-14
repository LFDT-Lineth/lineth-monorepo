/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer.client

import maru.consensus.ElFork

/**
 * Engine API method versions used for each fork, see https://github.com/ethereum/execution-apis/tree/main/src/engine.
 *
 * [schemaVersion] is the ExecutionPayload and PayloadAttributes structure version exchanged for the fork.
 */
internal data class EngineApiForkMethods(
  val getPayloadVersion: Int,
  val newPayloadVersion: Int,
  val forkChoiceUpdatedVersion: Int,
  val schemaVersion: Int,
) {
  val getPayload: String get() = "engine_getPayloadV$getPayloadVersion"
  val newPayload: String get() = "engine_newPayloadV$newPayloadVersion"
  val forkChoiceUpdated: String get() = "engine_forkchoiceUpdatedV$forkChoiceUpdatedVersion"

  companion object {
    fun of(fork: ElFork): EngineApiForkMethods =
      when (fork) {
        ElFork.Paris -> EngineApiForkMethods(1, 1, 1, schemaVersion = 1)
        ElFork.Shanghai -> EngineApiForkMethods(2, 2, 2, schemaVersion = 2)
        ElFork.Cancun -> EngineApiForkMethods(3, 3, 3, schemaVersion = 3)
        ElFork.Prague -> EngineApiForkMethods(4, 4, 3, schemaVersion = 3)
        ElFork.Osaka -> EngineApiForkMethods(5, 4, 3, schemaVersion = 3)
        ElFork.Amsterdam -> EngineApiForkMethods(6, 5, 4, schemaVersion = 4)
      }
  }
}

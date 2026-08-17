/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer.manager.ext

import maru.executionlayer.manager.ExecutionPayloadStatus
import maru.executionlayer.manager.ForkChoiceUpdatedResult
import maru.executionlayer.manager.PayloadStatus
import kotlin.random.Random

object DataGenerators {
  fun randomValidForkChoiceUpdatedResult(payloadId: ByteArray? = Random.nextBytes(8)): ForkChoiceUpdatedResult {
    val expectedPayloadStatus =
      PayloadStatus(
        ExecutionPayloadStatus.VALID,
        latestValidHash = Random.nextBytes(32),
        validationError = null,
      )
    return ForkChoiceUpdatedResult(expectedPayloadStatus, payloadId)
  }

  fun randomValidPayloadStatus(): PayloadStatus =
    PayloadStatus(ExecutionPayloadStatus.VALID, latestValidHash = Random.nextBytes(32), validationError = null)
}

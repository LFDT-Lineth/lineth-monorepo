/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.state

import maru.core.BeaconBlockBody
import maru.extensions.encodeHex
import org.apache.logging.log4j.LogManager

typealias FinalizationProvider = (BeaconBlockBody) -> FinalizationState

object InstantFinalizationProvider : FinalizationProvider {
  private val log = LogManager.getLogger(this.javaClass)

  override fun invoke(beaconBlockBody: BeaconBlockBody): FinalizationState {
    log.debug(
      "instant finalization: blockNumber={} blockHash={}",
      beaconBlockBody.executionPayload.blockNumber,
      beaconBlockBody.executionPayload.blockHash.encodeHex(),
    )

    return FinalizationState(
      safeBlockHash = beaconBlockBody.executionPayload.blockHash,
      finalizedBlockHash = beaconBlockBody.executionPayload.blockHash,
    )
  }
}

data class FinalizationState(
  val safeBlockHash: ByteArray,
  val finalizedBlockHash: ByteArray,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as FinalizationState

    if (!safeBlockHash.contentEquals(other.safeBlockHash)) return false
    if (!finalizedBlockHash.contentEquals(other.finalizedBlockHash)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = safeBlockHash.contentHashCode()
    result = 31 * result + finalizedBlockHash.contentHashCode()
    return result
  }
}

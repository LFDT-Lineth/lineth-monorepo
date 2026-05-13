/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer.manager

import maru.extensions.encodeHex

enum class ExecutionPayloadStatus(
  private val validity: Validity,
) {
  VALID(Validity.VALID),
  INVALID(Validity.INVALID),
  SYNCING(Validity.NOT_VALIDATED),
  ACCEPTED(Validity.NOT_VALIDATED),
  ;

  fun isValid(): Boolean = validity == Validity.VALID

  enum class Validity {
    VALID,
    NOT_VALIDATED,
    INVALID,
  }
}

data class PayloadStatus(
  val status: ExecutionPayloadStatus,
  val latestValidHash: ByteArray?,
  val validationError: String?,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as PayloadStatus

    if (status != other.status) return false
    if (latestValidHash != null) {
      if (other.latestValidHash == null) return false
      if (!latestValidHash.contentEquals(other.latestValidHash)) return false
    } else if (other.latestValidHash != null) {
      return false
    }
    if (validationError != other.validationError) return false

    return true
  }

  override fun hashCode(): Int {
    var result = status.hashCode()
    result = 31 * result + (latestValidHash?.contentHashCode() ?: 0)
    result = 31 * result + (validationError?.hashCode() ?: 0)
    return result
  }

  override fun toString(): String =
    "PayloadStatus(status=$status, latestValidHash=${latestValidHash?.encodeHex()}, validationError=$validationError)"
}

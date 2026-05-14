/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.compression

interface MaruCompressor {
  fun compress(payload: ByteArray): ByteArray

  fun decompress(payload: ByteArray): ByteArray
}

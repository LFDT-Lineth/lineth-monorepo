/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.core

import maru.serialization.SerDe

typealias HeaderHashFunction = (BeaconBlockHeader) -> ByteArray

fun interface Hasher {
  fun hash(input: ByteArray): ByteArray
}

fun interface ObjHasher<T> {
  fun hash(obj: T): ByteArray
}

/**
 * Utility class for hashing various parts of the beacon chain
 */
object HashUtil {
  fun headerHash(
    serDe: SerDe<BeaconBlockHeader>,
    hasher: Hasher,
  ): HeaderHashFunction = { header -> rootHash(header, serDe, hasher) }

  fun <T> rootHash(
    t: T,
    serDe: SerDe<T>,
    hasher: Hasher,
  ): ByteArray {
    val serialized = serDe.serialize(t)
    return hasher.hash(serialized)
  }
}

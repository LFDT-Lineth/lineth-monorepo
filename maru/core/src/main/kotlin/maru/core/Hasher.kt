/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.core

typealias BeaconBlockIdHashFunction = (BeaconBlockHeader) -> ByteArray

fun interface Hasher {
  fun hash(input: ByteArray): ByteArray
}

fun interface ObjHasher<T> {
  fun hash(obj: T): ByteArray
}

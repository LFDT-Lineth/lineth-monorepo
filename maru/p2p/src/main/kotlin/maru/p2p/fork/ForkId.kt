/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.fork

import linea.kotlin.encodeHex
import maru.core.Hasher
import maru.core.ObjHasher
import maru.crypto.Keccak256Hasher
import maru.serialization.Serializer
import java.nio.ByteBuffer

data class ForkId(
  val prevForkIdDigest: ByteArray,
  val forkSpecDigest: ByteArray,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ForkId

    if (!prevForkIdDigest.contentEquals(other.prevForkIdDigest)) return false
    if (!forkSpecDigest.contentEquals(other.forkSpecDigest)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = prevForkIdDigest.contentHashCode()
    result = 31 * result + forkSpecDigest.contentHashCode()
    return result
  }

  override fun toString(): String =
    "ForkId(prevForkIdDigest=${prevForkIdDigest.encodeHex()}, forkSpecDigest=${forkSpecDigest.encodeHex()})"
}

object ForkIdSerializer : Serializer<ForkId> {
  override fun serialize(value: ForkId): ByteArray {
    val buffer =
      ByteBuffer
        .allocate(value.prevForkIdDigest.size + value.forkSpecDigest.size)
        .put(value.prevForkIdDigest)
        .put(value.forkSpecDigest)
    return buffer.array()
  }
}

class ForkIdDigester(
  private val hasher: Hasher = Keccak256Hasher,
  private val serializer: (ForkId) -> ByteArray = ForkIdSerializer::serialize,
) : ObjHasher<ForkId> {
  override fun hash(obj: ForkId): ByteArray {
    val hash = hasher.hash(serializer(obj))
    return hash.sliceArray(hash.size - 4 until hash.size)
  }
}

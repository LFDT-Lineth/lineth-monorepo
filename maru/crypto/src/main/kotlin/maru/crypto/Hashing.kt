/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.crypto

import maru.core.Hasher
import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.datatypes.Hash
import java.security.MessageDigest

object Hashing {
  fun shortShaHash(inputData: ByteArray): ByteArray = sha256(inputData).slice(0 until 20).toByteArray()

  private fun sha256(input: ByteArray): ByteArray {
    val digest: MessageDigest = MessageDigest.getInstance("SHA-256")
    return digest.digest(input)
  }

  fun keccak(serializedBytes: ByteArray): ByteArray = Hash.hash(Bytes.wrap(serializedBytes)).bytes.toArray()
}

object Keccak256Hasher : Hasher {
  override fun hash(input: ByteArray): ByteArray = Hash.hash(Bytes.wrap(input)).bytes.toArray()
}

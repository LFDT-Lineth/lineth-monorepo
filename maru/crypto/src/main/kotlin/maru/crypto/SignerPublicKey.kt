/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.crypto

import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.crypto.SECPPublicKey

const val SECP256K1_PUBLIC_KEY_SIZE = 64

fun decodeSecp256k1PublicKey(encodedPublicKey: ByteArray): SECPPublicKey {
  require(encodedPublicKey.size == SECP256K1_PUBLIC_KEY_SIZE) {
    "Signer public key must be $SECP256K1_PUBLIC_KEY_SIZE bytes, got ${encodedPublicKey.size}"
  }
  val publicKey =
    try {
      SecpCrypto.signatureAlgorithm.createPublicKey(Bytes.wrap(encodedPublicKey.copyOf()))
    } catch (error: Exception) {
      throw IllegalArgumentException("Invalid signer public key", error)
    }
  require(SecpCrypto.signatureAlgorithm.isValidPublicKey(publicKey)) {
    "Signer public key is not a valid secp256k1 point"
  }
  return publicKey
}

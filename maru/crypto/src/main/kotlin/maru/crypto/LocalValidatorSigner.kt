/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.crypto

import linea.crypto.Secp256k1Signature
import linea.crypto.Signer
import org.apache.tuweni.bytes.Bytes32
import tech.pegasys.teku.infrastructure.async.SafeFuture

class LocalValidatorSigner(
  privateKeyBytes: ByteArray,
) : Signer<Secp256k1Signature> {
  private val keyPair =
    SecpCrypto.signatureAlgorithm.createKeyPair(
      SecpCrypto.signatureAlgorithm.createPrivateKey(Bytes32.wrap(privateKeyBytes.copyOf())),
    )
  private val publicKey = keyPair.publicKey.encodedBytes.toArray()

  override fun publicKey(): ByteArray = publicKey.copyOf()

  override fun sign(bytes: ByteArray): SafeFuture<Secp256k1Signature> {
    require(bytes.size == Bytes32.SIZE) {
      "Validator signer input must be ${Bytes32.SIZE} bytes, got ${bytes.size}"
    }
    val signature = SecpCrypto.signatureAlgorithm.sign(Bytes32.wrap(bytes.copyOf()), keyPair)
    return SafeFuture.completedFuture(Secp256k1Signature(signature.r, signature.s))
  }
}

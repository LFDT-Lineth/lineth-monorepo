/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.crypto

import maru.core.Seal
import maru.core.Validator
import org.apache.tuweni.bytes.Bytes
import org.apache.tuweni.bytes.Bytes32
import org.hyperledger.besu.crypto.SignatureAlgorithmFactory
import org.hyperledger.besu.datatypes.Hash
import org.hyperledger.besu.ethereum.core.Util

interface Crypto {
  fun privateKeyToValidator(rawPrivateKey: ByteArray): Validator

  fun privateKeyToAddress(rawPrivateKey: ByteArray): ByteArray

  fun signatureToAddress(
    signature: Seal,
    hash: ByteArray,
  ): ByteArray

  fun privateKeyBytesWithoutPrefix(privateKey: ByteArray): ByteArray
}

/**
 * SECP256K1 implementation of cryptographic operations.
 */
object SecpCrypto : Crypto {
  val signatureAlgorithm: org.hyperledger.besu.crypto.SignatureAlgorithm

  init {
    // CRITICAL: Cache the singleton so Besu's internal code (BftMessage.readPayload,
    // Util.signatureToAddress, CommitPayload.readFrom) returns the cached instance
    // instead of creating a new SECP256K1 + BouncyCastleProvider on every call.
    // Without pinning the curve, Besu may repeatedly initialize crypto providers
    // on the hot path, which is expensive for the event loop.
    //
    // Besu 26.3 removed SignatureAlgorithmFactory.isInstanceSet() and setDefaultInstance().
    // Use switchInstance(DEFAULT_EC_CURVE_NAME) to select secp256k1 once, then getInstance()
    // returns that configured singleton (replacing the old "set default if not set" pattern).
    SignatureAlgorithmFactory.switchInstance(SignatureAlgorithmFactory.DEFAULT_EC_CURVE_NAME)
    signatureAlgorithm = SignatureAlgorithmFactory.getInstance()
  }

  override fun privateKeyToValidator(rawPrivateKey: ByteArray): Validator =
    Validator(privateKeyToAddress(rawPrivateKey))

  override fun privateKeyToAddress(rawPrivateKey: ByteArray): ByteArray {
    val privateKey = signatureAlgorithm.createPrivateKey(Bytes32.wrap(rawPrivateKey))
    val keyPair = signatureAlgorithm.createKeyPair(privateKey)

    return Util.publicKeyToAddress(keyPair.publicKey).bytes.toArray()
  }

  override fun signatureToAddress(
    signature: Seal,
    hash: ByteArray,
  ): ByteArray {
    val secpSignature = signatureAlgorithm.decodeSignature(Bytes.wrap(signature.signature))
    return Util.signatureToAddress(secpSignature, Hash.wrap(Bytes32.wrap(hash))).bytes.toArray()
  }

  override fun privateKeyBytesWithoutPrefix(privateKey: ByteArray) = privateKey.takeLast(32).toByteArray()
}

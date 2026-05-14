/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.validation

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import maru.core.BeaconBlockHeader
import maru.core.Seal
import maru.core.Validator
import maru.crypto.SecpCrypto
import org.apache.tuweni.bytes.Bytes
import org.apache.tuweni.bytes.Bytes32
import org.hyperledger.besu.crypto.SignatureAlgorithm
import org.hyperledger.besu.ethereum.core.Util

interface SealVerifier {
  data class SealValidationError(
    val message: String,
  )

  fun extractValidator(
    seal: Seal,
    beaconBlockHeader: BeaconBlockHeader,
  ): Result<Validator, SealValidationError>
}

class SCEP256SealVerifier(
  private val signatureAlgorithm: SignatureAlgorithm = SecpCrypto.signatureAlgorithm,
) : SealVerifier {
  override fun extractValidator(
    seal: Seal,
    beaconBlockHeader: BeaconBlockHeader,
  ): Result<Validator, SealVerifier.SealValidationError> {
    val signature = signatureAlgorithm.decodeSignature(Bytes.wrap(seal.signature))
    val blockHash = beaconBlockHeader.hash
    val publicKey = signatureAlgorithm.recoverPublicKeyFromSignature(Bytes32.wrap(blockHash), signature)
    return if (publicKey.isEmpty) {
      Err(
        SealVerifier.SealValidationError(
          "Could not extract validator from seal for block. seal=$seal block=$beaconBlockHeader",
        ),
      )
    } else {
      Ok(Validator(Util.publicKeyToAddress(publicKey.get()).bytes.toArray()))
    }
  }
}

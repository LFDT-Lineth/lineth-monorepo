/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import linea.crypto.Secp256k1Signature
import linea.crypto.Signer
import linea.kotlin.encodeHex
import maru.config.QbftConfig
import maru.config.ValidatorSignerType
import maru.consensus.DifficultyAwareQbftConfig
import maru.consensus.ForkSpec
import maru.consensus.ForksSchedule
import maru.consensus.QbftConsensusConfig
import maru.core.Validator
import maru.crypto.SecpCrypto
import org.apache.logging.log4j.LogManager
import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.ethereum.core.Util

internal object ValidatorIdentityValidator {
  private const val PUBLIC_KEY_SIZE = 64
  private val log = LogManager.getLogger(ValidatorIdentityValidator::class.java)

  fun validate(
    qbftConfig: QbftConfig,
    beaconGenesisConfig: ForksSchedule,
    signer: Signer<Secp256k1Signature>,
  ) {
    val validator = validatorFor(signer)
    val validatorsFromAllForks: Set<Validator> =
      beaconGenesisConfig.forks
        .flatMap<ForkSpec, Validator> {
          when (val configuration = it.configuration) {
            is DifficultyAwareQbftConfig -> configuration.postTtdConfig.validatorSet
            is QbftConsensusConfig -> configuration.validatorSet
            else ->
              throw IllegalArgumentException(
                "Unsupported consensus configuration: ${configuration::class.qualifiedName}",
              )
          }
        }.toSet()

    if (validator !in validatorsFromAllForks) {
      if (qbftConfig.validatorSigner.type == ValidatorSignerType.CUSTOM) {
        throw IllegalArgumentException(
          "Custom validator signer '${qbftConfig.validatorSigner.name}' address " +
            "${validator.address.encodeHex()} is not present in any configured validator set",
        )
      }
      log.warn(
        "Local validator={} is not present in any validator set in the genesis file",
        validator,
      )
    }
  }

  fun validatorFor(signer: Signer<Secp256k1Signature>): Validator {
    val encodedPublicKey = signer.publicKey().copyOf()
    require(encodedPublicKey.size == PUBLIC_KEY_SIZE) {
      "Validator signer public key must be $PUBLIC_KEY_SIZE bytes, got ${encodedPublicKey.size}"
    }
    val publicKey =
      try {
        SecpCrypto.signatureAlgorithm.createPublicKey(Bytes.wrap(encodedPublicKey))
      } catch (error: Exception) {
        throw IllegalArgumentException("Invalid validator signer public key", error)
      }
    require(SecpCrypto.signatureAlgorithm.isValidPublicKey(publicKey)) {
      "Validator signer public key is not a valid secp256k1 point"
    }
    return Validator(Util.publicKeyToAddress(publicKey).bytes.toArray())
  }
}

/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import linea.crypto.CloseableSigner
import linea.crypto.Secp256k1Signature
import linea.crypto.withCloseAction
import maru.config.QbftConfig
import maru.config.ValidatorSignerConfig
import maru.config.ValidatorSignerType
import maru.consensus.ForksSchedule
import maru.crypto.LocalValidatorSigner
import maru.crypto.SecpCrypto

fun interface CustomValidatorSignerFactory {
  fun create(config: ValidatorSignerConfig): CloseableSigner<Secp256k1Signature>
}

object MissingCustomValidatorSignerFactory : CustomValidatorSignerFactory {
  override fun create(config: ValidatorSignerConfig): CloseableSigner<Secp256k1Signature> {
    require(config.type == ValidatorSignerType.CUSTOM) {
      "CustomValidatorSignerFactory is only used for custom validator signers"
    }
    throw IllegalArgumentException(
      "Custom validator signer '${config.name}' requires an external CustomValidatorSignerFactory",
    )
  }
}

internal class ValidatorSignerFactory(
  private val customSignerFactory: CustomValidatorSignerFactory,
) {
  fun create(
    qbftConfig: QbftConfig,
    beaconGenesisConfig: ForksSchedule,
    privateKey: ByteArray,
  ): CloseableSigner<Secp256k1Signature> {
    val signer =
      when (qbftConfig.validatorSigner.type) {
        ValidatorSignerType.LOCAL ->
          LocalValidatorSigner(
            SecpCrypto.privateKeyBytesWithoutPrefix(privateKey),
          ).withCloseAction()

        ValidatorSignerType.CUSTOM ->
          customSignerFactory.create(qbftConfig.validatorSigner)
      }

    try {
      ValidatorSetMembership.validate(
        qbftConfig = qbftConfig,
        beaconGenesisConfig = beaconGenesisConfig,
        validator = signer.toValidator(),
      )
      return signer
    } catch (error: Throwable) {
      try {
        signer.close()
      } catch (closeError: Throwable) {
        error.addSuppressed(closeError)
      }
      throw error
    }
  }
}

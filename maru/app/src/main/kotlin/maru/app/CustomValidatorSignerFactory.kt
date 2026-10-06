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
import maru.config.ValidatorSignerConfig
import maru.config.ValidatorSignerType

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

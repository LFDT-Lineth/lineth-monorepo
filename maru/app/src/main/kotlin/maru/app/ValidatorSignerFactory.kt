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
import maru.config.ValidatorSignerConfig
import maru.config.ValidatorSignerType
import java.util.concurrent.atomic.AtomicBoolean

fun interface ValidatorSignerFactory {
  fun create(config: ValidatorSignerConfig): ManagedValidatorSigner
}

class ManagedValidatorSigner(
  val signer: Signer<Secp256k1Signature>,
  private val closeAction: () -> Unit = {},
) : AutoCloseable {
  private val closed = AtomicBoolean()

  override fun close() {
    if (closed.compareAndSet(false, true)) {
      closeAction()
    }
  }
}

object DefaultValidatorSignerFactory : ValidatorSignerFactory {
  override fun create(config: ValidatorSignerConfig): ManagedValidatorSigner {
    require(config.type == ValidatorSignerType.CUSTOM) {
      "ValidatorSignerFactory is only used for custom validator signers"
    }
    throw IllegalArgumentException(
      "Custom validator signer '${config.name}' requires an external ValidatorSignerFactory",
    )
  }
}

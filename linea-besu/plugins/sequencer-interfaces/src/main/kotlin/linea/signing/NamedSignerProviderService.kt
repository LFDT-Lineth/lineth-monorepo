/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package linea.signing

import linea.crypto.Secp256k1Signature
import linea.crypto.Signer
import org.hyperledger.besu.plugin.services.BesuService

/**
 * Provides transaction signers from a separately packaged Besu plugin.
 *
 * Implementations own provider-specific configuration and signer lifecycle. A returned signer's
 * public key must be ready for synchronous access and encoded as the 64-byte unsigned secp256k1
 * coordinates `x || y`.
 */
fun interface NamedSignerProviderService : BesuService {
  /**
   * Resolves a signer by its provider-defined logical name.
   *
   * @param name logical signer name
   * @return initialized secp256k1 signer
   */
  operator fun get(name: String): Signer<Secp256k1Signature>
}

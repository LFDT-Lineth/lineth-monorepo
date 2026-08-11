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
import maru.core.Validator
import maru.crypto.decodeSecp256k1PublicKey
import org.hyperledger.besu.ethereum.core.Util

internal fun Signer<Secp256k1Signature>.toValidator(): Validator {
  val publicKey = decodeSecp256k1PublicKey(publicKey())
  return Validator(Util.publicKeyToAddress(publicKey).bytes.toArray())
}

/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package lineth.signing

import linea.crypto.Secp256k1Signature
import linea.crypto.Signer
import org.hyperledger.besu.plugin.services.BesuService

/**
 * A transaction signer supplied by a separately packaged Besu plugin.
 *
 * The providing plugin owns signer configuration and lifecycle. The public key must be ready for
 * synchronous access and encoded as the 64-byte unsigned secp256k1 coordinates `x || y`.
 */
interface SignerService : BesuService, Signer<Secp256k1Signature>

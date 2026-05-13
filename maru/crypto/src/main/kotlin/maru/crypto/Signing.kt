/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.crypto

import maru.core.Signer
import maru.extensions.toBytes32
import org.apache.tuweni.bytes.Bytes32
import org.hyperledger.besu.cryptoservices.NodeKey

object Signing {
  class ULongSigner(
    private val nodeKey: NodeKey,
  ) : Signer<ULong> {
    override fun sign(signee: ULong): ByteArray =
      nodeKey
        .sign(Bytes32.wrap(signee.toBytes32()))
        .encodedBytes()
        .toArray()
  }
}

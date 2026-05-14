/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.serialization.rlp

import maru.serialization.SerDe
import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.ethereum.rlp.RLP
import org.hyperledger.besu.ethereum.rlp.RLPInput
import org.hyperledger.besu.ethereum.rlp.RLPOutput

interface RLPSerDe<T> : SerDe<T> {
  fun writeTo(
    value: T,
    rlpOutput: RLPOutput,
  )

  fun readFrom(rlpInput: RLPInput): T

  override fun serialize(value: T): ByteArray = RLP.encode { rlpOutput -> this.writeTo(value, rlpOutput) }.toArray()

  override fun deserialize(bytes: ByteArray): T = this.readFrom(RLP.input(Bytes.wrap(bytes)))
}

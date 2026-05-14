/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.serialization.rlp

import maru.compression.MaruCompressor
import maru.serialization.compression.MaruSnappyFramedCompressor
import org.hyperledger.besu.ethereum.rlp.RLPInput
import org.hyperledger.besu.ethereum.rlp.RLPOutput

class MaruCompressorRLPSerDe<T>(
  private val serDe: RLPSerDe<T>,
  private val compressor: MaruCompressor = MaruSnappyFramedCompressor(),
) : RLPSerDe<T> {
  override fun serialize(value: T): ByteArray = compressor.compress(serDe.serialize(value))

  override fun deserialize(bytes: ByteArray): T = serDe.deserialize(compressor.decompress(bytes))

  override fun writeTo(
    value: T,
    rlpOutput: RLPOutput,
  ) = serDe.writeTo(value, rlpOutput)

  override fun readFrom(rlpInput: RLPInput): T = serDe.readFrom(rlpInput)
}

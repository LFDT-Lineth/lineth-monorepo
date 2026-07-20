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
import org.hyperledger.besu.ethereum.rlp.RLPOutput

/**
 * Write-only compressing RLP serializer: the [MaruCompressorRLPSerDe] equivalent for the write side only.
 * Wraps a function-free [RLPSerializer] so a compressed, fork-free static serializer can be exposed in
 * [RLPSerializers] without pulling in a header-containing deserializer. The compressed output is byte-identical
 * to [MaruCompressorRLPSerDe] over the same inner serializer.
 */
class MaruCompressorRLPSerializer<T>(
  private val serializer: RLPSerializer<T>,
  private val compressor: MaruCompressor = MaruSnappyFramedCompressor(),
) : RLPSerializer<T> {
  override fun serialize(value: T): ByteArray = compressor.compress(serializer.serialize(value))

  override fun writeTo(
    value: T,
    rlpOutput: RLPOutput,
  ) = serializer.writeTo(value, rlpOutput)
}

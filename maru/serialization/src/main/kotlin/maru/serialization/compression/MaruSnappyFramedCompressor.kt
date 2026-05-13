/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.serialization.compression

import io.libp2p.etc.types.readUvarint
import io.libp2p.etc.types.toByteArray
import io.libp2p.etc.types.toByteBuf
import io.netty.buffer.ByteBuf
import maru.compression.MaruCompressor
import maru.serialization.MAX_MESSAGE_SIZE
import org.apache.tuweni.bytes.Bytes
import tech.pegasys.teku.networking.eth2.rpc.core.RpcException.ChunkTooLongException
import tech.pegasys.teku.networking.eth2.rpc.core.RpcException.DecompressFailedException
import tech.pegasys.teku.networking.eth2.rpc.core.RpcException.ExtraDataAppendedException
import tech.pegasys.teku.networking.eth2.rpc.core.RpcException.PayloadTruncatedException
import tech.pegasys.teku.networking.eth2.rpc.core.encodings.ProtobufEncoder
import tech.pegasys.teku.networking.eth2.rpc.core.encodings.compression.Compressor
import tech.pegasys.teku.networking.eth2.rpc.core.encodings.compression.exceptions.CompressionException
import tech.pegasys.teku.networking.eth2.rpc.core.encodings.compression.exceptions.PayloadLargerThanExpectedException
import tech.pegasys.teku.networking.eth2.rpc.core.encodings.compression.exceptions.PayloadSmallerThanExpectedException
import tech.pegasys.teku.networking.eth2.rpc.core.encodings.compression.snappy.SnappyFramedCompressor
import java.util.Optional

class MaruSnappyFramedCompressor : MaruCompressor {
  private val compressor: Compressor = SnappyFramedCompressor()

  private fun readLengthPrefixHeader(input: ByteBuf): Long? {
    val length: Long = input.readUvarint()
    if (length < 0) {
      // wait for more byte to read length field
      return null
    }
    return length
  }

  private fun decompress(input: ByteBuf): ByteBuf? {
    if (!input.isReadable) {
      return null
    }
    val length: Long? = readLengthPrefixHeader(input)
    if (length != null) {
      if (length > MAX_MESSAGE_SIZE) {
        throw ChunkTooLongException()
      }
    } else {
      return null
    }

    val decompressor = compressor.createDecompressor(length.toInt())

    val ret: Optional<ByteBuf>
    try {
      ret = decompressor.decodeOneMessage(input)
    } catch (_: PayloadSmallerThanExpectedException) {
      throw PayloadTruncatedException()
    } catch (_: PayloadLargerThanExpectedException) {
      throw ExtraDataAppendedException()
    } catch (_: CompressionException) {
      throw DecompressFailedException()
    } finally {
      decompressor.complete()
      decompressor.close()
    }

    return if (ret.isPresent) {
      ret.get()
    } else if (length == 0L) {
      ByteArray(0).toByteBuf()
    } else {
      null
    }
  }

  override fun compress(payload: ByteArray): ByteArray {
    val header =
      ProtobufEncoder
        .encodeVarInt(payload.size)
    val compressedPayload: Bytes =
      compressor
        .compress(Bytes.wrap(payload))
    return Bytes
      .concatenate(header, compressedPayload)
      .toArray()
  }

  override fun decompress(payload: ByteArray): ByteArray {
    val compressedByteBuf = payload.toByteBuf()
    var decompressedByteBuf: ByteBuf? = null
    try {
      decompressedByteBuf = decompress(compressedByteBuf)

      if (decompressedByteBuf == null) {
        throw DecompressFailedException()
      }
      return decompressedByteBuf.toByteArray()
    } finally {
      compressedByteBuf.release()
      decompressedByteBuf?.release()
    }
  }
}

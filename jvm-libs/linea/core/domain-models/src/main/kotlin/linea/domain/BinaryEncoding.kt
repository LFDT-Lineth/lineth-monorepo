package linea.domain

import tech.pegasys.teku.infrastructure.async.SafeFuture

interface BinaryEncoder<T> {
  fun encode(block: T): ByteArray
  fun encode(blocks: List<T>): List<ByteArray> = blocks.map { encode(it) }
}

interface BinaryDecoder<T> {
  fun decode(block: ByteArray): T
  fun decode(blocks: List<ByteArray>): List<T> = blocks.map { decode(it) }
}

interface BinaryEncoderAsync<T> {
  fun encodeAsync(block: T): SafeFuture<ByteArray>
  fun encodeAsync(blocks: List<T>): SafeFuture<List<ByteArray>> =
    SafeFuture.collectAll(blocks.map { encodeAsync(it) }.stream())
}

interface BinaryDecoderAsync<T> {
  fun decodeAsync(block: ByteArray): SafeFuture<T>
  fun decodeAsync(blocks: List<ByteArray>): SafeFuture<List<T>> =
    SafeFuture.collectAll(blocks.map { decodeAsync(it) }.stream())
}

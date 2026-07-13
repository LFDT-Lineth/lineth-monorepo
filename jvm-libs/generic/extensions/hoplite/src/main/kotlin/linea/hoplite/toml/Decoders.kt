package linea.hoplite.toml

import com.sksamuel.hoplite.ConfigFailure
import com.sksamuel.hoplite.ConfigResult
import com.sksamuel.hoplite.DecoderContext
import com.sksamuel.hoplite.LongNode
import com.sksamuel.hoplite.Node
import com.sksamuel.hoplite.StringNode
import com.sksamuel.hoplite.decoder.Decoder
import com.sksamuel.hoplite.fp.invalid
import com.sksamuel.hoplite.fp.valid
import linea.kotlin.decodeHex
import kotlin.reflect.KType
import kotlin.time.Duration
import kotlin.time.Instant

class TomlByteArrayHexDecoder : Decoder<ByteArray> {
  override fun decode(node: Node, type: KType, context: DecoderContext): ConfigResult<ByteArray> {
    return when (node) {
      is StringNode ->
        runCatching {
          node.value.decodeHex()
        }.fold(
          { it.valid() },
          { ConfigFailure.DecodeError(node, type).invalid() },
        )

      else -> {
        ConfigFailure.DecodeError(node, type).invalid()
      }
    }
  }

  override fun supports(type: KType): Boolean {
    return type.classifier == ByteArray::class
  }
}

class TomlKotlinDurationDecoder : Decoder<Duration> {
  override fun decode(node: Node, type: KType, context: DecoderContext): ConfigResult<Duration> {
    return when (node) {
      is StringNode ->
        runCatching {
          Duration.parse(node.value)
        }.fold(
          { it.valid() },
          { ConfigFailure.DecodeError(node, type).invalid() },
        )

      else -> {
        ConfigFailure.DecodeError(node, type).invalid()
      }
    }
  }

  override fun supports(type: KType): Boolean {
    return type.classifier == Duration::class
  }
}

class TomlKotlinInstantDecoder : Decoder<Instant> {
  override fun decode(node: Node, type: KType, context: DecoderContext): ConfigResult<Instant> {
    return when (node) {
      is StringNode ->
        runCatching {
          Instant.parse(node.value)
        }.fold(
          { it.valid() },
          { ConfigFailure.DecodeError(node, type).invalid() },
        )
      is LongNode ->
        runCatching {
          Instant.fromEpochSeconds(node.value)
        }.fold(
          { it.valid() },
          { ConfigFailure.DecodeError(node, type).invalid() },
        )

      else -> {
        ConfigFailure.DecodeError(node, type).invalid()
      }
    }
  }

  override fun supports(type: KType): Boolean {
    return type.classifier == Instant::class
  }
}

open class TomlEnumDecoder<T : Enum<T>>(
  private val clazz: Class<T>,
  private val ignoreCase: Boolean = true,
) : Decoder<T> {
  override fun decode(node: Node, type: KType, context: DecoderContext): ConfigResult<T> {
    return when (node) {
      is StringNode ->
        runCatching {
          clazz.enumConstants.first { it.name.equals(node.value, ignoreCase = ignoreCase) }
        }.fold(
          { it.valid() },
          { ConfigFailure.DecodeError(node, type).invalid() },
        )

      else -> {
        ConfigFailure.DecodeError(node, type).invalid()
      }
    }
  }

  override fun supports(type: KType): Boolean {
    return type.classifier == clazz.kotlin
  }
}

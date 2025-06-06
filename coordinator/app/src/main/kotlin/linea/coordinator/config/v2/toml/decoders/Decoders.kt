package linea.coordinator.config.v2.toml.decoders

import com.sksamuel.hoplite.ConfigFailure
import com.sksamuel.hoplite.ConfigResult
import com.sksamuel.hoplite.DecoderContext
import com.sksamuel.hoplite.Node
import com.sksamuel.hoplite.StringNode
import com.sksamuel.hoplite.decoder.Decoder
import com.sksamuel.hoplite.fp.invalid
import com.sksamuel.hoplite.fp.valid
import linea.coordinator.config.v2.toml.SignerConfigToml
import linea.kotlin.decodeHex
import kotlin.reflect.KType
import kotlin.time.Duration

class TomlByteArrayHexDecoder : Decoder<ByteArray> {
  override fun decode(
    node: Node,
    type: KType,
    context: DecoderContext,
  ): ConfigResult<ByteArray> {
    return when (node) {
      is StringNode -> runCatching {
        node.value.decodeHex()
      }.fold(
        { it.valid() },
        { ConfigFailure.DecodeError(node, type).invalid() },
      )

      else -> { ConfigFailure.DecodeError(node, type).invalid() }
    }
  }

  override fun supports(type: KType): Boolean {
    return type.classifier == ByteArray::class
  }
}

class TomlKotlinDurationDecoder : Decoder<Duration> {
  override fun decode(
    node: Node,
    type: KType,
    context: DecoderContext,
  ): ConfigResult<Duration> {
    return when (node) {
      is StringNode -> runCatching {
        Duration.parse(node.value)
      }.fold(
        { it.valid() },
        { ConfigFailure.DecodeError(node, type).invalid() },
      )

      else -> { ConfigFailure.DecodeError(node, type).invalid() }
    }
  }

  override fun supports(type: KType): Boolean {
    return type.classifier == Duration::class
  }
}

class TomlSignerTypeDecoder : Decoder<SignerConfigToml.SignerType> {
  override fun decode(
    node: Node,
    type: KType,
    context: DecoderContext,
  ): ConfigResult<SignerConfigToml.SignerType> {
    return when (node) {
      is StringNode -> runCatching {
        SignerConfigToml.SignerType.valueOfIgnoreCase(node.value.lowercase())
      }.fold(
        { it.valid() },
        { ConfigFailure.DecodeError(node, type).invalid() },
      )

      else -> { ConfigFailure.DecodeError(node, type).invalid() }
    }
  }

  override fun supports(type: KType): Boolean {
    return type.classifier == SignerConfigToml.SignerType::class
  }
}

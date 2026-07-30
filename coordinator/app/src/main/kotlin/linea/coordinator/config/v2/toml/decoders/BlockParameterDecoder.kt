package linea.coordinator.config.v2.toml.decoders

import com.sksamuel.hoplite.ConfigFailure
import com.sksamuel.hoplite.ConfigResult
import com.sksamuel.hoplite.DecoderContext
import com.sksamuel.hoplite.LongNode
import com.sksamuel.hoplite.Node
import com.sksamuel.hoplite.StringNode
import com.sksamuel.hoplite.decoder.Decoder
import com.sksamuel.hoplite.fp.invalid
import com.sksamuel.hoplite.fp.valid
import linea.domain.BlockParameter
import linea.domain.toBlockParameter
import kotlin.reflect.KType

@Suppress("UNCHECKED_CAST")
abstract class AbstractBlockParameterDecoder<T : BlockParameter> : Decoder<T> {
  override fun decode(node: Node, type: KType, context: DecoderContext): ConfigResult<T> {
    return when (node) {
      is StringNode ->
        runCatching {
          node.value.toBlockParameter()
        }.fold(
          { (it as T).valid() },
          { ConfigFailure.DecodeError(node, type).invalid() },
        )

      is LongNode ->
        runCatching {
          BlockParameter.fromNumber(node.value)
        }.fold(
          { (it as T).valid() },
          { ConfigFailure.DecodeError(node, type).invalid() },
        )

      else -> ConfigFailure.DecodeError(node, type).invalid()
    }
  }
}

class BlockParameterTagDecoder : AbstractBlockParameterDecoder<BlockParameter.Tag>() {
  override fun supports(type: KType): Boolean = type.classifier == BlockParameter.Tag::class
}

class BlockParameterNumberDecoder : AbstractBlockParameterDecoder<BlockParameter.BlockNumber>() {
  override fun supports(type: KType): Boolean = type.classifier == BlockParameter.BlockNumber::class
}

class BlockParameterDecoder : AbstractBlockParameterDecoder<BlockParameter>() {
  override fun supports(type: KType): Boolean = type.classifier == BlockParameter::class
}

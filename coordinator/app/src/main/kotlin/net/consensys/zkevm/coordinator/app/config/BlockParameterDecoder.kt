package net.consensys.zkevm.coordinator.app.config

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
import kotlin.reflect.KType

class BlockParameterDecoder : Decoder<BlockParameter> {
  override fun supports(type: KType): Boolean = type.classifier == BlockParameter::class
  override fun decode(node: Node, type: KType, context: DecoderContext): ConfigResult<BlockParameter> {
    return when (node) {
      is StringNode -> runCatching {
        BlockParameter.parse(node.value)
      }.fold(
        { it.valid() },
        { ConfigFailure.DecodeError(node, type).invalid() }
      )

      is LongNode -> runCatching {
        BlockParameter.fromNumber(node.value)
      }.fold(
        { it.valid() },
        { ConfigFailure.DecodeError(node, type).invalid() }
      )

      else -> ConfigFailure.DecodeError(node, type).invalid()
    }
  }
}

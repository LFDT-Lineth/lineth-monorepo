package lineth.coordinator.config.v2.toml.decoders

import com.sksamuel.hoplite.ConfigFailure
import com.sksamuel.hoplite.ConfigResult
import com.sksamuel.hoplite.DecoderContext
import com.sksamuel.hoplite.Node
import com.sksamuel.hoplite.StringNode
import com.sksamuel.hoplite.decoder.Decoder
import com.sksamuel.hoplite.fp.invalid
import com.sksamuel.hoplite.fp.valid
import lineth.coordinator.config.v2.SignerConfig
import kotlin.reflect.KType

/** Any non-blank string is a valid signer type: unregistered names are rejected by the signer factory. */
class TomlSignerTypeDecoder : Decoder<SignerConfig.SignerType> {
  override fun decode(node: Node, type: KType, context: DecoderContext): ConfigResult<SignerConfig.SignerType> =
    when {
      node is StringNode && node.value.isNotBlank() -> SignerConfig.SignerType(node.value).valid()
      else -> ConfigFailure.DecodeError(node, type).invalid()
    }

  override fun supports(type: KType): Boolean = type.classifier == SignerConfig.SignerType::class
}

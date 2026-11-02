package linea.web3j.ethapi

import linea.ethapi.EthConfig
import linea.ethapi.EthConfigClient
import linea.ethapi.ForkConfig
import linea.web3j.requestAsync
import org.web3j.protocol.Web3jService
import org.web3j.protocol.core.Request
import org.web3j.protocol.core.Response
import tech.pegasys.teku.infrastructure.async.SafeFuture
import tools.jackson.core.JsonParser
import tools.jackson.databind.DeserializationContext
import tools.jackson.databind.JsonNode
import tools.jackson.databind.ValueDeserializer
import tools.jackson.databind.annotation.JsonDeserialize

/**
 * Web3j based implementation of [EthConfigClient] for the EIP-7910 `eth_config` JSON-RPC method.
 */
class Web3jEthConfigClient(
  private val web3jService: Web3jService,
) : EthConfigClient {
  override fun ethConfig(): SafeFuture<EthConfig> {
    return Request(
      "eth_config",
      emptyList<Any>(),
      web3jService,
      EthConfigResponse::class.java,
    ).requestAsync { response -> requireNotNull(response.result) { "eth_config returned no result" } }
  }
}

class EthConfigResponse : Response<EthConfig>() {
  @JsonDeserialize(using = ResponseDeserializer::class)
  override fun setResult(result: EthConfig?) {
    super.setResult(result)
  }

  class ResponseDeserializer : ValueDeserializer<EthConfig>() {
    override fun deserialize(jsonParser: JsonParser, deserializationContext: DeserializationContext): EthConfig {
      return EthConfigResponseParser.parse(jsonParser.readValueAsTree())
    }
  }
}

object EthConfigResponseParser {
  fun parse(json: JsonNode): EthConfig {
    return EthConfig(
      current = parseFork(requireNotNull(json.get("current")?.takeUnless { it.isNull }) { "missing field: current" }),
      next = json.get("next")?.takeUnless { it.isNull }?.let(::parseFork),
      last = json.get("last")?.takeUnless { it.isNull }?.let(::parseFork),
    )
  }

  private fun parseFork(json: JsonNode): ForkConfig {
    val activationTime = requireNotNull(json.get("activationTime")) { "missing field: activationTime" }
    val chainId = requireNotNull(json.get("chainId")) { "missing field: chainId" }.asString()
    require(chainId.startsWith("0x")) { "invalid chainId: $chainId" }
    val systemContracts = json.get("systemContracts")
      ?.takeUnless { it.isNull }
      ?.properties()
      ?.associate { (name, address) -> name to address.asString() }
      ?: emptyMap()
    return ForkConfig(
      activationTime = activationTime.asLong().toULong(),
      chainId = chainId.removePrefix("0x").toULong(16),
      systemContracts = systemContracts,
    )
  }
}

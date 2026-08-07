package lineth.web3j.ethapi

import lineth.domain.BlockParameter
import lineth.ethapi.ExecutionWitness
import lineth.ethapi.ExecutionWitnessClient
import lineth.web3j.requestAsync
import org.web3j.protocol.Web3jService
import org.web3j.protocol.core.Request
import org.web3j.protocol.core.Response
import tech.pegasys.teku.infrastructure.async.SafeFuture
import tools.jackson.core.JsonParser
import tools.jackson.core.JsonToken
import tools.jackson.databind.DeserializationContext
import tools.jackson.databind.JsonNode
import tools.jackson.databind.ValueDeserializer
import tools.jackson.databind.annotation.JsonDeserialize

/**
 * Web3j based implementation of [ExecutionWitnessClient] for the `debug_executionWitness` JSON-RPC method.
 */
class Web3jExecutionWitnessClient(
  private val web3jService: Web3jService,
) : ExecutionWitnessClient {

  override fun getExecutionWitness(block: BlockParameter): SafeFuture<ExecutionWitness?> {
    return Request(
      "debug_executionWitness",
      listOf(block.toDebugExecutionWitnessRpcParam()),
      web3jService,
      ExecutionWitnessResponse::class.java,
    ).requestAsync { response -> response.result }
  }
}

private fun BlockParameter.toDebugExecutionWitnessRpcParam(): String =
  when (this) {
    is BlockParameter.Tag -> tag
    is BlockParameter.BlockNumber -> number.toString()
    is BlockParameter.BlockHash -> hashHex
  }

class ExecutionWitnessResponse : Response<ExecutionWitness>() {
  @JsonDeserialize(using = ResponseDeserializer::class)
  override fun setResult(result: ExecutionWitness?) {
    super.setResult(result)
  }

  class ResponseDeserializer : ValueDeserializer<ExecutionWitness>() {
    override fun deserialize(
      jsonParser: JsonParser,
      deserializationContext: DeserializationContext,
    ): ExecutionWitness? {
      return if (jsonParser.currentToken() != JsonToken.VALUE_NULL) {
        val json: JsonNode = jsonParser.readValueAsTree()
        ExecutionWitnessResponseParser.parse(json)
      } else {
        null
      }
    }
  }
}

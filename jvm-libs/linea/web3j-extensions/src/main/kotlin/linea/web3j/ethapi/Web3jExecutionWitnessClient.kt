package linea.web3j.ethapi

import com.fasterxml.jackson.core.JsonParser
import com.fasterxml.jackson.core.JsonToken
import com.fasterxml.jackson.databind.DeserializationContext
import com.fasterxml.jackson.databind.JsonDeserializer
import com.fasterxml.jackson.databind.JsonNode
import com.fasterxml.jackson.databind.ObjectReader
import com.fasterxml.jackson.databind.annotation.JsonDeserialize
import linea.domain.BlockParameter
import linea.ethapi.ExecutionWitness
import linea.ethapi.ExecutionWitnessClient
import linea.ethapi.ExecutionWitnessClientException
import linea.ethapi.ExecutionWitnessError
import linea.web3j.requestAsync
import org.web3j.protocol.ObjectMapperFactory
import org.web3j.protocol.Web3jService
import org.web3j.protocol.core.Request
import org.web3j.protocol.core.Response
import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Web3j based implementation of [ExecutionWitnessClient] for the `debug_executionWitness` JSON-RPC method.
 */
class Web3jExecutionWitnessClient(
  private val web3jService: Web3jService,
) : ExecutionWitnessClient {

  override fun getExecutionWitness(block: BlockParameter): SafeFuture<ExecutionWitness> {
    return Request(
      "debug_executionWitness",
      listOf(block.toDebugExecutionWitnessRpcParam()),
      web3jService,
      ExecutionWitnessResponse::class.java,
    ).requestAsync { response ->
      response.result
        ?: throw ExecutionWitnessClientException(
          ExecutionWitnessError.NULL_RESULT,
          "debug_executionWitness returned null (witness unavailable for block)",
        )
    }
  }
}

private fun BlockParameter.toDebugExecutionWitnessRpcParam(): String =
  when (this) {
    is BlockParameter.Tag -> getTag()
    is BlockParameter.BlockNumber -> getNumber().toString()
    is BlockParameter.BlockHash -> getHash()
  }

class ExecutionWitnessResponse : Response<ExecutionWitness>() {
  @JsonDeserialize(using = ResponseDeserializer::class)
  override fun setResult(result: ExecutionWitness?) {
    super.setResult(result)
  }

  class ResponseDeserializer : JsonDeserializer<ExecutionWitness>() {
    private val objectReader: ObjectReader = ObjectMapperFactory.getObjectReader()

    override fun deserialize(
      jsonParser: JsonParser,
      deserializationContext: DeserializationContext,
    ): ExecutionWitness? {
      return if (jsonParser.currentToken != JsonToken.VALUE_NULL) {
        val json = objectReader.readValue(jsonParser, JsonNode::class.java)
        ExecutionWitnessResponseParser.parse(json)
      } else {
        null
      }
    }
  }
}

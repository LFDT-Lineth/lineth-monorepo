package net.consensys.linea.jsonrpc.client

import com.fasterxml.jackson.databind.JsonNode
import com.fasterxml.jackson.databind.node.JsonNodeType
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import io.vertx.core.Future
import io.vertx.core.json.JsonArray
import io.vertx.core.json.JsonObject
import net.consensys.linea.jsonrpc.JsonRpcErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcRequest
import net.consensys.linea.jsonrpc.JsonRpcSuccessResponse

fun toPrimitiveOrJacksonJsonNode(value: Any?): Any? = value

@Suppress("UNCHECKED_CAST")
fun toPrimitiveOrVertxJson(value: Any?): Any? {
  if (value == null) {
    return null
  }
  return when (value) {
    is String -> value
    is Number -> value
    is Boolean -> value
    is JsonNode -> {
      when (value.nodeType) {
        JsonNodeType.STRING, JsonNodeType.NUMBER, JsonNodeType.BOOLEAN, JsonNodeType.NULL ->
          value
            .toPrimitiveOrJsonNode()

        JsonNodeType.OBJECT -> JsonObject(objectMapper.convertValue(value, Map::class.java) as Map<String, Any?>)
        JsonNodeType.ARRAY -> JsonArray(objectMapper.convertValue(value, List::class.java) as List<Any?>)
        else -> throw IllegalArgumentException("Unsupported JsonNodeType: ${value.nodeType}")
      }
    }

    else -> throw IllegalArgumentException("Unsupported type: ${value::class.java}")
  }
}

interface JsonRpcClient {
  fun makeRequest(
    request: JsonRpcRequest,
    resultMapper: (Any?) -> Any? = ::toPrimitiveOrVertxJson, // to keep backward compatibility
  ): Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>
}

fun isResultOk(result: Result<Any?, Any?>): Boolean = result is Ok

interface JsonRpcClientWithRetries : JsonRpcClient {
  fun makeRequest(
    request: JsonRpcRequest,
    resultMapper: (Any?) -> Any? = ::toPrimitiveOrVertxJson, // to keep backward compatibility
    stopRetriesPredicate: (result: Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>) -> Boolean = ::isResultOk,
  ): Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>
}

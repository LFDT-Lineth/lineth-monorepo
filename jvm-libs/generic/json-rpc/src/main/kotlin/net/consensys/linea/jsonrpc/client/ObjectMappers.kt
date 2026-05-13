package net.consensys.linea.jsonrpc.client

import com.fasterxml.jackson.databind.JsonNode
import com.fasterxml.jackson.databind.node.JsonNodeType
import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import io.vertx.core.json.jackson.VertxModule
import linea.s11n.jackson.ethByteAsHexSerialisersModule

internal val objectMapper = jacksonObjectMapper()
  .registerModules(VertxModule())
  .registerModules(ethByteAsHexSerialisersModule)

fun JsonNodeType.isPrimitive(): Boolean {
  return when (this) {
    JsonNodeType.STRING, JsonNodeType.NUMBER, JsonNodeType.BOOLEAN, JsonNodeType.NULL -> true
    else -> false
  }
}

fun JsonNode.toPrimitiveOrJsonNode(): Any? {
  return if (this.nodeType.isPrimitive()) {
    objectMapper.convertValue(this, Any::class.java)
  } else {
    this
  }
}

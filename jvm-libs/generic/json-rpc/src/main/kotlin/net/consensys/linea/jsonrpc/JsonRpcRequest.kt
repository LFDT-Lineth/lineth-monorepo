package net.consensys.linea.jsonrpc

import com.fasterxml.jackson.annotation.JsonIgnore
import java.util.StringJoiner

interface JsonRpcRequest {
  val jsonrpc: String
  val id: Any
  val method: String
  val params: Any

  @get:JsonIgnore
  val isValid: Boolean
    get() {
      if ("2.0" != jsonrpc) {
        return false
      }
      return "" != method
    }
}

internal data class JsonRpcRequestData(
  override val jsonrpc: String,
  override val id: Any,
  override val method: String,
  override val params: Any,
) : JsonRpcRequest

data class JsonRpcRequestListParams(
  override val jsonrpc: String,
  override val id: Any,
  override val method: String,
  override val params: List<Any?>,
) : JsonRpcRequest {
  override fun toString(): String {
    return StringJoiner(", ", JsonRpcRequestListParams::class.java.simpleName + "[", "]")
      .add("jsonrpc='$jsonrpc'")
      .add("id='$id'")
      .add("method='$method'")
      .add("params=$params")
      .toString()
  }
}

data class JsonRpcRequestMapParams(
  override val jsonrpc: String,
  override val id: Any,
  override val method: String,
  override val params: Map<String, *>,
) : JsonRpcRequest {
  override fun toString(): String {
    return StringJoiner(", ", JsonRpcRequestMapParams::class.java.simpleName + "[", "]")
      .add("jsonrpc='$jsonrpc'")
      .add("id='$id'")
      .add("method='$method'")
      .add("params=$params")
      .toString()
  }
}

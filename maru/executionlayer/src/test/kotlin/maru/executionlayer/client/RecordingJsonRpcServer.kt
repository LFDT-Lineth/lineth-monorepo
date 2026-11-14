/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer.client

import com.fasterxml.jackson.databind.JsonNode
import com.fasterxml.jackson.databind.node.ObjectNode
import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import com.sun.net.httpserver.HttpServer
import java.net.InetSocketAddress
import java.net.URI
import java.net.URL
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors
import kotlin.time.Duration

/** JSON-RPC server that records requests and answers each method with a scripted `result` or `error`. */
class RecordingJsonRpcServer : AutoCloseable {
  data class RecordedRequest(val body: JsonNode, val authorization: String?) {
    val method: String get() = body["method"].asText()
  }

  private data class Reply(val result: JsonNode?, val error: JsonNode?, val delay: Duration)

  private val objectMapper = jacksonObjectMapper()
  private val replies = ConcurrentHashMap<String, Reply>()
  val requests = CopyOnWriteArrayList<RecordedRequest>()
  private val executor: ExecutorService = Executors.newCachedThreadPool()
  private val server: HttpServer =
    HttpServer.create(InetSocketAddress("127.0.0.1", 0), 0).apply {
      executor = this@RecordingJsonRpcServer.executor
      createContext("/") { exchange ->
        exchange.use {
          val body = objectMapper.readTree(exchange.requestBody.readBytes())
          val request = RecordedRequest(body, exchange.requestHeaders.getFirst("Authorization"))
          requests.add(request)
          val reply = replies[request.method] ?: Reply(result = null, error = null, delay = Duration.ZERO)
          Thread.sleep(reply.delay.inWholeMilliseconds)
          val response = objectMapper.createObjectNode().put("jsonrpc", "2.0").set<ObjectNode>("id", body["id"])
          if (reply.error != null) {
            response.set<JsonNode>(
              "error",
              reply.error,
            )
          } else {
            response.set("result", reply.result)
          }
          val bytes = objectMapper.writeValueAsBytes(response)
          exchange.responseHeaders.add("Content-Type", "application/json")
          exchange.sendResponseHeaders(200, bytes.size.toLong())
          exchange.responseBody.write(bytes)
        }
      }
      start()
    }

  val endpoint: URL = URI("http://127.0.0.1:${server.address.port}").toURL()

  fun replyResult(method: String, result: Any?, delay: Duration = Duration.ZERO) {
    replies[method] = Reply(objectMapper.valueToTree(result), error = null, delay = delay)
  }

  fun replyError(method: String, code: Int, message: String) {
    val error = objectMapper.createObjectNode().put("code", code).put("message", message)
    replies[method] = Reply(result = null, error = error, delay = Duration.ZERO)
  }

  override fun close() {
    server.stop(0)
    executor.shutdownNow()
  }
}

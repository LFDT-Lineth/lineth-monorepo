package net.consensys.linea.jsonrpc.client

import io.vertx.core.Vertx
import io.vertx.core.http.HttpServer
import io.vertx.core.http.HttpServerRequest
import io.vertx.core.http.HttpServerResponse
import io.vertx.core.json.JsonObject
import net.consensys.linea.async.get
import java.net.URI
import java.net.URL
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.CopyOnWriteArrayList
import kotlin.time.Duration

/**
 * Minimal JSON-RPC server for tests that abort in-flight requests (timeouts, resets).
 * WireMock is avoided there because client-side resets of its delayed responses interfere with other WireMock
 * instances running concurrently in the same JVM.
 */
internal class ScriptedJsonRpcServer(private val vertx: Vertx) {
  data class Reply(
    val delay: Duration = Duration.ZERO,
    val dribbleInterval: Duration? = null,
  )

  data class ReceivedRequest(val method: String, val headers: Map<String, String>)

  private val replies = ConcurrentHashMap<String, Reply>()
  private lateinit var server: HttpServer
  val receivedRequests = CopyOnWriteArrayList<ReceivedRequest>()

  fun start(): URL {
    server = vertx.createHttpServer().requestHandler(::handle).listen(0).get()
    return URI("http://localhost:${server.actualPort()}/").toURL()
  }

  fun stop() {
    server.close().get()
  }

  fun reply(method: String, reply: Reply) {
    replies[method] = reply
  }

  private fun handle(request: HttpServerRequest) {
    request.body().onSuccess { body ->
      val jsonRequest = JsonObject(body)
      val method = jsonRequest.getString("method")
      receivedRequests.add(ReceivedRequest(method, request.headers().associate { it.key to it.value }))
      val responseBody =
        JsonObject()
          .put("jsonrpc", "2.0")
          .put("id", jsonRequest.getValue("id"))
          .put("result", "OK")
          .encode()
      val reply = replies[method] ?: Reply()
      vertx.setTimer(reply.delay.inWholeMilliseconds.coerceAtLeast(1)) {
        respond(request.response(), responseBody, reply.dribbleInterval)
      }
    }
  }

  private fun respond(response: HttpServerResponse, body: String, dribbleInterval: Duration?) {
    if (response.closed()) return
    response.putHeader("Content-Type", "application/json")
    if (dribbleInterval == null) {
      response.end(body)
    } else {
      response.setChunked(true)
      dribble(response, body, 0, dribbleInterval)
    }
  }

  private fun dribble(response: HttpServerResponse, body: String, index: Int, interval: Duration) {
    if (response.closed()) return
    if (index == body.length) {
      response.end()
      return
    }
    response.write(body[index].toString())
    vertx.setTimer(interval.inWholeMilliseconds.coerceAtLeast(1)) { dribble(response, body, index + 1, interval) }
  }
}

/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package testutils

import io.javalin.Javalin
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import java.util.concurrent.CopyOnWriteArrayList

data class EngineApiCall(
  val method: String,
  val timestampMs: Long,
)

class RecordingEngineProxy(
  private val targetUrl: String,
) {
  companion object {
    private val METHOD_REGEX = Regex("\"method\"\\s*:\\s*\"([^\"]+)\"")
  }

  val calls: MutableList<EngineApiCall> = CopyOnWriteArrayList()
  private val httpClient = OkHttpClient()
  private val javalin =
    Javalin
      .create { config ->
        config.showJavalinBanner = false
      }.post("/*") { ctx ->
        val body = ctx.bodyAsBytes()
        val bodyStr = ctx.body()

        val method = METHOD_REGEX.find(bodyStr)?.groupValues?.get(1) ?: "unknown"
        if (method.startsWith("engine_")) {
          calls.add(EngineApiCall(method, System.currentTimeMillis()))
        }

        val proxyRequest =
          Request
            .Builder()
            .url(targetUrl)
            .post(body.toRequestBody("application/json".toMediaType()))
            .build()

        httpClient.newCall(proxyRequest).execute().use { response ->
          ctx.status(response.code)
          response.body?.bytes()?.let { ctx.result(it) }
        }
      }

  fun start() {
    javalin.start(0)
  }

  fun stop() {
    javalin.stop()
  }

  fun port(): Int = javalin.port()

  fun url(): String = "http://127.0.0.1:${port()}"

  fun clear() {
    calls.clear()
  }
}

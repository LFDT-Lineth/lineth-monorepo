/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package lineth.teku

import com.fasterxml.jackson.core.JsonParser
import com.fasterxml.jackson.databind.DeserializationFeature
import com.fasterxml.jackson.databind.ObjectMapper
import io.reactivex.Flowable
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.RequestBody.Companion.toRequestBody
import org.web3j.protocol.Web3jService
import org.web3j.protocol.core.BatchRequest
import org.web3j.protocol.core.BatchResponse
import org.web3j.protocol.core.Request
import org.web3j.protocol.core.Response
import org.web3j.protocol.exceptions.ClientConnectionException
import org.web3j.protocol.websocket.events.Notification
import org.web3j.utils.Async
import java.util.concurrent.CompletableFuture
import okhttp3.Request as OkHttpRequest

/**
 * web3j 6.0.0's [org.web3j.protocol.Service]/[org.web3j.protocol.http.HttpService] hardcode a Jackson 3
 * (`tools.jackson.databind`) ObjectMapper with no way to override it. Teku's engine-api schema classes
 * (ForkChoiceStateV1, PayloadAttributesV3, UInt64/Bytes wrappers, etc.) are annotated with classic Jackson 2
 * (`com.fasterxml.jackson.databind.annotation.JsonSerialize`) custom serializers, which Jackson 3 does not
 * recognize at all: it silently falls back to default bean-property reflection instead of raising an error,
 * producing garbage nested JSON (e.g. a `timestamp` field serialized as its internal boolean flags rather
 * than a hex string), which the execution client then rejects as invalid.
 *
 * This is a from-scratch [Web3jService] (not a [org.web3j.protocol.http.HttpService] subclass, since its
 * ObjectMapper field is private/final) that serializes with a classic Jackson 2 mapper over the same HTTP
 * transport, so the engine-api channel stays correct while the rest of the codebase keeps web3j 6.0.0.
 *
 * TODO: revert this workaround (delete this class, wire [TekuWeb3JClientFactory] back to a plain
 *  [org.web3j.protocol.http.HttpService]) once Teku ships a release whose engine-api schema classes
 *  (ForkChoiceStateV1, PayloadAttributesV3, UInt64/Bytes serializers, etc.) serialize correctly under
 *  web3j's Jackson 3 (`tools.jackson.databind`) ObjectMapper - i.e. once Teku migrates those annotations
 *  off classic Jackson 2 (`com.fasterxml.jackson.databind.annotation.JsonSerialize`), or otherwise adopts
 *  web3j >= 6.0.0 itself. As of Teku 26.7.0 (the latest available at the time of writing), Teku still pins
 *  web3j 4.14.0 internally and still uses classic Jackson 2 annotations, so this is still required.
 */
internal class Jackson2HttpService(
  private val url: String,
  private val httpClient: OkHttpClient,
) : Web3jService {
  internal val objectMapper: ObjectMapper = ObjectMapper()
    .configure(JsonParser.Feature.ALLOW_UNQUOTED_FIELD_NAMES, true)
    .configure(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES, false)
  private val jsonMediaType = "application/json; charset=utf-8".toMediaType()

  private fun performIO(payload: String): ByteArray {
    val httpRequest = OkHttpRequest.Builder()
      .url(url)
      .post(payload.toRequestBody(jsonMediaType))
      .build()
    httpClient.newCall(httpRequest).execute().use { response ->
      val bodyBytes = response.body?.bytes()
      if (!response.isSuccessful) {
        throw ClientConnectionException(response.code, bodyBytes?.toString(Charsets.UTF_8) ?: "N/A")
      }
      return bodyBytes ?: ByteArray(0)
    }
  }

  override fun <T : Response<*>> send(request: Request<*, *>, responseType: Class<T>): T {
    val payload = objectMapper.writeValueAsString(request)
    val responseBytes = performIO(payload)
    return objectMapper.readValue(responseBytes, responseType)
  }

  override fun <T : Response<*>> sendAsync(request: Request<*, *>, responseType: Class<T>): CompletableFuture<T> =
    Async.run { send(request, responseType) }

  override fun sendBatch(batchRequest: BatchRequest): BatchResponse {
    val requests = batchRequest.requests
    if (requests.isEmpty()) {
      return BatchResponse(emptyList(), emptyList())
    }
    val payload = objectMapper.writeValueAsString(requests)
    val responseBytes = performIO(payload)
    val nodes = objectMapper.readTree(responseBytes)
    val responses = requests.mapIndexed { i, request ->
      objectMapper.treeToValue(nodes.get(i), request.responseType) as Response<*>
    }
    return BatchResponse(requests, responses)
  }

  override fun sendBatchAsync(batchRequest: BatchRequest): CompletableFuture<BatchResponse> =
    Async.run { sendBatch(batchRequest) }

  override fun <T : Notification<*>> subscribe(
    request: Request<*, *>,
    unsubscribeMethod: String,
    responseType: Class<T>,
  ): Flowable<T> {
    throw UnsupportedOperationException("Service ${this::class.simpleName} does not support subscriptions")
  }

  override fun close() {}
}

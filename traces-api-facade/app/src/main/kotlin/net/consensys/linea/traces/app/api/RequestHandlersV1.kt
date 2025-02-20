package net.consensys.linea.traces.app.api

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import com.github.michaelbull.result.flatMap
import com.github.michaelbull.result.get
import com.github.michaelbull.result.map
import com.github.michaelbull.result.mapError
import io.vertx.core.Future
import io.vertx.core.json.JsonObject
import io.vertx.ext.auth.User
import linea.domain.BlockNumberAndHash
import linea.kotlin.decodeHex
import net.consensys.linea.TracesConflationServiceV1
import net.consensys.linea.TracesCountingServiceV1
import net.consensys.linea.TracesError
import net.consensys.linea.VersionedResult
import net.consensys.linea.async.toVertxFuture
import net.consensys.linea.jsonrpc.JsonRpcErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcRequest
import net.consensys.linea.jsonrpc.JsonRpcRequestHandler
import net.consensys.linea.jsonrpc.JsonRpcRequestMapParams
import net.consensys.linea.jsonrpc.JsonRpcSuccessResponse
import tech.pegasys.teku.infrastructure.async.SafeFuture

private fun parseBlockNumberAndHash(json: JsonObject) = BlockNumberAndHash(
  json.getString("blockNumber").toULong(),
  json.getString("blockHash").decodeHex()
)

internal fun validateParams(request: JsonRpcRequest): Result<JsonRpcRequestMapParams, JsonRpcErrorResponse> {
  if (request.params !is Map<*, *>) {
    return Err(
      JsonRpcErrorResponse.invalidParams(
        request.id,
        "params should be an object"
      )
    )
  }
  return try {
    val jsonRpcRequest = request as JsonRpcRequestMapParams
    if (jsonRpcRequest.params.isEmpty()) {
      Err(
        JsonRpcErrorResponse.invalidParams(
          request.id,
          "Parameters map is empty!"
        )
      )
    } else {
      Ok(request)
    }
  } catch (e: Exception) {
    Err(JsonRpcErrorResponse.invalidRequest())
  }
}

class TracesCounterRequestHandlerV1(
  private val tracesCountingService: TracesCountingServiceV1,
  private val validator: TracesSemanticVersionValidator
) :
  JsonRpcRequestHandler {

  override fun invoke(
    user: User?,
    request: JsonRpcRequest,
    requestJson: JsonObject
  ): Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>> {
    val (block, version) = try {
      val parsingResult = validateParams(request).flatMap { validatedRequest ->
        validator.validateExpectedVersion(
          validatedRequest.id,
          validatedRequest.params["expectedTracesApiVersion"].toString()
        ).map {
          val version =
            validatedRequest.params["rawExecutionTracesVersion"].toString()
          Pair(
            parseBlockNumberAndHash(JsonObject.mapFrom(validatedRequest.params["block"])),
            version
          )
        }
      }
      if (parsingResult is Err) {
        return Future.succeededFuture(parsingResult)
      } else {
        parsingResult.get()!!
      }
    } catch (e: Exception) {
      return Future.succeededFuture(
        Err(
          JsonRpcErrorResponse.invalidParams(
            request.id,
            e.message
          )
        )
      )
    }

    return tracesCountingService
      .getBlockTracesCounters(block, version)
      .thenApply { result ->
        result
          .map {
            val rpcResult =
              JsonObject()
                .put("tracesEngineVersion", it.version)
                .put("blockNumber", block.number.toString())
                .put("blockL1Size", it.result.blockL1Size.toString())
                .put(
                  "tracesCounters",
                  it.result.tracesCounters.entries().associate { it.first to it.second.toLong() }
                )
            JsonRpcSuccessResponse(request.id, rpcResult)
          }
          .mapError { error -> JsonRpcErrorResponse(request.id, jsonRpcError(error)) }
      }
      .toVertxFuture()
  }
}

abstract class AbstractTracesConflationRequestHandlerV1<T>(private val validator: TracesSemanticVersionValidator) :
  JsonRpcRequestHandler {

  abstract fun tracesContent(
    blocks: List<BlockNumberAndHash>,
    version: String
  ): SafeFuture<Result<T, TracesError>>

  override fun invoke(
    user: User?,
    request: JsonRpcRequest,
    requestJson: JsonObject
  ): Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>> {
    val (blocks: List<BlockNumberAndHash>, version: String) = try {
      val parsingResult = validateParams(request).flatMap { validatedRequest ->
        validator.validateExpectedVersion(
          validatedRequest.id,
          validatedRequest.params["expectedTracesApiVersion"].toString()
        ).map {
          val version = validatedRequest.params["rawExecutionTracesVersion"].toString()
          val blocks = validatedRequest.params["blocks"] as List<Any?>
          Pair(
            blocks.map { blockJson ->
              parseBlockNumberAndHash(
                JsonObject.mapFrom(blockJson)
              )
            },
            version
          )
        }
      }
      if (parsingResult is Err) {
        return Future.succeededFuture(parsingResult)
      } else {
        parsingResult.get()!!
      }
    } catch (e: Exception) {
      return Future.succeededFuture(
        Err(
          JsonRpcErrorResponse.invalidParams(
            request.id,
            e.message
          )
        )
      )
    }
    if (blocks.isEmpty()) {
      return Future.succeededFuture(
        Err(
          JsonRpcErrorResponse.invalidParams(
            request.id,
            "Empty list of blocks!"
          )
        )
      )
    }

    return tracesContent(blocks, version)
      .thenApply { result ->
        result
          .map { JsonRpcSuccessResponse(request.id, it) }
          .mapError { error -> JsonRpcErrorResponse(request.id, jsonRpcError(error)) }
      }
      .toVertxFuture()
  }
}

class GenerateConflatedTracesToFileRequestHandlerV1(
  private val service: TracesConflationServiceV1,
  validator: TracesSemanticVersionValidator
) :
  AbstractTracesConflationRequestHandlerV1<JsonObject>(validator) {
  override fun tracesContent(
    blocks: List<BlockNumberAndHash>,
    version: String
  ): SafeFuture<Result<JsonObject, TracesError>> {
    val blocksSorted = blocks.sortedBy { it.number }
    return service.generateConflatedTracesToFile(blocksSorted, version)
      .thenApply { result: Result<VersionedResult<String>, TracesError> ->
        result.map {
          JsonObject()
            .put("tracesEngineVersion", it.version)
            .put("startBlockNumber", blocksSorted.first().number.toString())
            .put("endBlockNumber", blocksSorted.last().number.toString())
            .put("conflatedTracesFileName", it.result)
        }
      }
  }
}

class GetConflatedTracesRequestHandlerV1(
  private val service: TracesConflationServiceV1,
  validator: TracesSemanticVersionValidator
) :
  AbstractTracesConflationRequestHandlerV1<JsonObject>(validator) {

  override fun tracesContent(
    blocks: List<BlockNumberAndHash>,
    version: String
  ): SafeFuture<Result<JsonObject, TracesError>> {
    return service.getConflatedTraces(blocks, version)
      .thenApply { result: Result<VersionedResult<JsonObject>, TracesError> ->
        result.map {
          JsonObject().put("tracesEngineVersion", it.version)
            .put("conflatedTraces", it.result)
        }
      }
  }
}

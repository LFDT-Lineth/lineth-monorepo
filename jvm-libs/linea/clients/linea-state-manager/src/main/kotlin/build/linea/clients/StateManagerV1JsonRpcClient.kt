package build.linea.clients

import com.fasterxml.jackson.databind.JsonNode
import com.fasterxml.jackson.databind.node.ArrayNode
import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import io.vertx.core.json.JsonObject
import linea.domain.BlockInterval
import linea.kotlin.decodeHex
import linea.kotlin.fromHexString
import net.consensys.linea.errors.ErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcErrorResponseException
import net.consensys.linea.jsonrpc.client.JsonRpcClientFactory
import net.consensys.linea.jsonrpc.client.JsonRpcV2Client
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.net.URI

class StateManagerV1JsonRpcClient(
  private val rpcClient: JsonRpcV2Client,
  private val zkStateManagerVersion: String,
  private val log: Logger = LogManager.getLogger(StateManagerV1JsonRpcClient::class.java),
) : StateManagerClientV1 {

  companion object {
    fun create(
      rpcClientFactory: JsonRpcClientFactory,
      endpoints: List<URI>,
      maxInflightRequestsPerClient: UInt,
      requestRetry: RequestRetryConfig,
      requestTimeout: Long? = null,
      zkStateManagerVersion: String,
      logger: Logger = LogManager.getLogger(StateManagerV1JsonRpcClient::class.java),
    ): StateManagerV1JsonRpcClient {
      return StateManagerV1JsonRpcClient(
        rpcClient = rpcClientFactory.createJsonRpcV2Client(
          endpoints = endpoints,
          maxInflightRequestsPerClient = maxInflightRequestsPerClient,
          retryConfig = requestRetry,
          requestTimeout = requestTimeout,
          log = logger,
          shallRetryRequestsClientBasePredicate = { it is Err },
        ),
        zkStateManagerVersion = zkStateManagerVersion,
      )
    }
  }

  override fun rollupGetHeadBlockNumber(): SafeFuture<ULong> {
    return rpcClient
      .makeRequest(
        method = "rollup_getZkEVMBlockNumber",
        params = emptyList<Unit>(),
        resultMapper = { ULong.fromHexString(it as String) },
      )
  }

  override fun rollupGetStateMerkleProof(blockInterval: BlockInterval): SafeFuture<GetZkEVMStateMerkleProofResponse> {
    val params = listOf(
      JsonObject.of(
        "startBlockNumber",
        blockInterval.startBlockNumber.toLong(),
        "endBlockNumber",
        blockInterval.endBlockNumber.toLong(),
        "zkStateManagerVersion",
        zkStateManagerVersion,
      ),
    )

    return rpcClient
      .makeRequest(
        method = "rollup_getZkEVMStateMerkleProofV0",
        params = params,
        resultMapper = ::parseZkEVMStateMerkleProofResponse,
      )
  }

  override fun rollupGetStateMerkleProofWithTypedError(
    blockInterval: BlockInterval,
  ): SafeFuture<Result<GetZkEVMStateMerkleProofResponse, ErrorResponse<StateManagerErrorType>>> {
    return rollupGetStateMerkleProof(blockInterval)
      .handleComposed { result, th ->
        if (th != null) {
          if (th is JsonRpcErrorResponseException) {
            SafeFuture.completedFuture(Err(mapErrorResponse(th)))
          } else {
            SafeFuture.failedFuture(th)
          }
        } else {
          SafeFuture.completedFuture(Ok(result))
        }
      }
  }

  private fun mapErrorResponse(
    jsonRpcErrorResponse: JsonRpcErrorResponseException,
  ): ErrorResponse<StateManagerErrorType> {
    val errorType =
      try {
        StateManagerErrorType.valueOf(
          jsonRpcErrorResponse.rpcErrorMessage.substringBefore('-').trim(),
        )
      } catch (_: Exception) {
        log.error(
          "State manager found unrecognised JSON-RPC response error: {}",
          jsonRpcErrorResponse.rpcErrorMessage,
        )
        StateManagerErrorType.UNKNOWN
      }

    return ErrorResponse(
      errorType,
      listOfNotNull(
        jsonRpcErrorResponse.rpcErrorMessage,
        jsonRpcErrorResponse.rpcErrorData?.toString(),
      )
        .joinToString(": "),
    )
  }

  private fun parseZkEVMStateMerkleProofResponse(result: Any?): GetZkEVMStateMerkleProofResponse {
    result as JsonNode
    return GetZkEVMStateMerkleProofResponse(
      zkStateManagerVersion = result.get("zkStateManagerVersion").asText(),
      zkStateMerkleProof = result.get("zkStateMerkleProof") as ArrayNode,
      zkParentStateRootHash = result.get("zkParentStateRootHash").asText().decodeHex(),
      zkEndStateRootHash = result.get("zkEndStateRootHash").asText().decodeHex(),
    )
  }
}

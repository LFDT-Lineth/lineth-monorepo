package linea.coordinator.clients.executionwitness

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Result
import com.github.michaelbull.result.fold
import io.vertx.core.Vertx
import linea.domain.BlockParameter
import linea.error.ErrorResponse
import linea.executionwitness.ExecutionWitness
import linea.executionwitness.ExecutionWitnessClient
import linea.executionwitness.ExecutionWitnessError
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.jsonrpc.JsonRpcRequestListParams
import net.consensys.linea.jsonrpc.client.JsonRpcClient
import net.consensys.linea.jsonrpc.client.JsonRpcRequestRetryer
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicInteger

class ExecutionWitnessJsonRpcClient(
  private val rpcClient: JsonRpcClient,
) : ExecutionWitnessClient {

  constructor(
    vertx: Vertx,
    rpcClient: JsonRpcClient,
    retryConfig: RequestRetryConfig,
    log: Logger = LogManager.getLogger(ExecutionWitnessJsonRpcClient::class.java),
  ) : this(
    JsonRpcRequestRetryer(
      vertx,
      rpcClient,
      config = JsonRpcRequestRetryer.Config(
        methodsToRetry = retryableMethods,
        requestRetry = retryConfig,
      ),
      log = log,
    ),
  )

  private val requestId = AtomicInteger(0)

  override fun getExecutionWitness(
    block: BlockParameter,
  ): SafeFuture<Result<ExecutionWitness, ErrorResponse<ExecutionWitnessError>>> {
    val jsonRequest = JsonRpcRequestListParams(
      jsonrpc = "2.0",
      id = requestId.incrementAndGet(),
      method = "debug_executionWitness",
      params = listOf(block.toDebugExecutionWitnessRpcParam()),
    )

    return rpcClient
      .makeRequest(jsonRequest)
      .toSafeFuture()
      .thenApply { responseResult ->
        responseResult.fold(
          { success -> ExecutionWitnessResponseParser.parseExecutionWitness(success) },
          { error -> Err(ExecutionWitnessResponseParser.mapRpcError(error)) },
        )
      }
  }

  companion object {
    internal val retryableMethods = setOf("debug_executionWitness")
  }
}

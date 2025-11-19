package linea.web3j.ethapi

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.domain.BlockParameter
import linea.domain.BlockParameter.Companion.toBlockParameter
import linea.domain.RetryConfig
import linea.ethapi.EthApiClient
import linea.jsonrpc.TestingJsonRpcServer
import net.consensys.linea.jsonrpc.JsonRpcErrorResponseException
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import kotlin.time.Duration.Companion.milliseconds

@ExtendWith(VertxExtension::class)
class Web3jEthApiClientWithRetriesTest {
  private lateinit var jsonRpcServer: TestingJsonRpcServer
  private lateinit var client: EthApiClient

  @BeforeEach
  fun setup(vertx: Vertx) {
    jsonRpcServer = TestingJsonRpcServer(
      vertx = vertx,
      recordRequestsResponses = true,
    )
    client = createEthApiClient(
      rpcUrl = jsonRpcServer.httpEndpoint.toString(),
      requestRetryConfig = RetryConfig(maxRetries = 2u, backoffDelay = 10.milliseconds),
      vertx = vertx,
    )
  }

  @Test
  fun `should retry until retries elapsed`() {
    jsonRpcServer.handle("eth_getBlockByNumber") { request ->
      throw JsonRpcErrorResponseException(rpcErrorCode = -123, rpcErrorMessage = "Internal error")
    }
    runCatching { client.ethGetBlockByNumberTxHashes(BlockParameter.Tag.FINALIZED).get() }
    assertThat(jsonRpcServer.recordedRequests()).hasSize(3)
  }

  @Test
  fun `should stop retry when predicate is true`(vertx: Vertx) {
    jsonRpcServer.handle("eth_getBlockByNumber") { request ->
      throw JsonRpcErrorResponseException(rpcErrorCode = -39001, rpcErrorMessage = "Unknown block")
    }
    val client = createEthApiClient(
      rpcUrl = jsonRpcServer.httpEndpoint.toString(),
      requestRetryConfig = RetryConfig(maxRetries = 2u, backoffDelay = 10.milliseconds),
      vertx = vertx,
      stopRetriesOnErrorPredicate = { th ->
        if (th.cause is linea.error.JsonRpcErrorResponseException) {
          val rpcError = th.cause as linea.error.JsonRpcErrorResponseException
          rpcError.method == "eth_getBlockByNumber" && rpcError.rpcErrorCode == -39001
        } else {
          false
        }
      },
    )
    runCatching { client.ethGetBlockByNumberTxHashes(BlockParameter.Tag.FINALIZED).get() }
    assertThat(jsonRpcServer.recordedRequests()).hasSize(1)
  }

  @Test
  fun `should stop retry when getBlockByNumber FINALIED or SAFE is not found`() {
    jsonRpcServer.handle("eth_getBlockByNumber") { request ->
      throw JsonRpcErrorResponseException(rpcErrorCode = -39001, rpcErrorMessage = "Unknown block")
    }
    runCatching { client.ethGetBlockByNumberTxHashes(BlockParameter.Tag.FINALIZED).get() }
    assertThat(jsonRpcServer.recordedRequests()).hasSize(1)
    jsonRpcServer.cleanRecordedRequests()

    runCatching { client.ethGetBlockByNumberFullTxs(BlockParameter.Tag.FINALIZED).get() }
    assertThat(jsonRpcServer.recordedRequests()).hasSize(1)
    jsonRpcServer.cleanRecordedRequests()

    runCatching { client.ethGetBlockByNumberTxHashes(BlockParameter.Tag.SAFE).get() }
    assertThat(jsonRpcServer.recordedRequests()).hasSize(1)
    jsonRpcServer.cleanRecordedRequests()

    runCatching { client.ethGetBlockByNumberFullTxs(BlockParameter.Tag.SAFE).get() }
    assertThat(jsonRpcServer.recordedRequests()).hasSize(1)

    jsonRpcServer.cleanRecordedRequests()
    runCatching { client.ethGetBlockByNumberFullTxs(BlockParameter.Tag.LATEST).get() }
    assertThat(jsonRpcServer.recordedRequests()).hasSize(3)

    jsonRpcServer.cleanRecordedRequests()
    runCatching { client.ethGetBlockByNumberFullTxs(2.toBlockParameter()).get() }
    assertThat(jsonRpcServer.recordedRequests()).hasSize(3)
  }
}

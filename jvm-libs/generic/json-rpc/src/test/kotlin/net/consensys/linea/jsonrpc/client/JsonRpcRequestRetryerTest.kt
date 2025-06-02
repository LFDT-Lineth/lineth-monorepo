package net.consensys.linea.jsonrpc.client

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import io.vertx.core.Future
import io.vertx.core.Vertx
import net.consensys.linea.async.AsyncRetryer
import net.consensys.linea.async.RetriedExecutionException
import net.consensys.linea.async.get
import net.consensys.linea.jsonrpc.JsonRpcError
import net.consensys.linea.jsonrpc.JsonRpcErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcRequestListParams
import net.consensys.linea.jsonrpc.JsonRpcSuccessResponse
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.mockito.kotlin.any
import org.mockito.kotlin.anyOrNull
import org.mockito.kotlin.doReturn
import org.mockito.kotlin.eq
import org.mockito.kotlin.mock
import org.mockito.kotlin.never
import org.mockito.kotlin.spy
import org.mockito.kotlin.times
import org.mockito.kotlin.verify
import java.net.SocketException
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

class JsonRpcRequestRetryerTest {

  private lateinit var delegate: JsonRpcClient
  private lateinit var retryer: AsyncRetryer<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>
  private val ethBlockNumberRequest = JsonRpcRequestListParams(
    jsonrpc = "2.0",
    id = "1",
    method = "eth_blockNumber",
    params = listOf("0x1"),
  )
  private val networkError1 = SocketException("Forced network error 1")
  private val networkError2 = SocketException("Forced network error 2")
  private val maxRetries = 10
  private val config = JsonRpcRequestRetryer.Config(
    methodsToRetry = emptySet(),
    requestRetry = RequestRetryConfig(
      maxRetries = maxRetries.toUInt(),
      timeout = 20.seconds,
      backoffDelay = 10.milliseconds,
      failuresWarningThreshold = 2u,
    ),
  )
  private lateinit var vertx: Vertx

  @BeforeEach
  fun beforeEach() {
    vertx = Vertx.vertx()
    retryer = AsyncRetryer.retryer(
      vertx,
      backoffDelay = 10.milliseconds,
      maxRetries = maxRetries,
      timeout = 20.seconds,
      initialDelay = null,
    )
    delegate = mock() {
      on { makeRequest(any(), anyOrNull()) }
        .doReturn(Future.failedFuture(networkError1))
        .doReturn(Future.succeededFuture(Err(JsonRpcErrorResponse("1", JsonRpcError.internalError("Forced error 2")))))
        .doReturn(Future.succeededFuture(Ok(JsonRpcSuccessResponse("1", "0x1"))))
        .doReturn(Future.succeededFuture(Ok(JsonRpcSuccessResponse("1", "0x2"))))
        .doReturn(Future.succeededFuture(Ok(JsonRpcSuccessResponse("1", "0x3"))))
    }
  }

  @AfterEach
  fun afterEach() {
    vertx.close()
  }

  @Test
  fun `should retry request when in retries list until success is returned`() {
    val methodsToRetry = setOf(ethBlockNumberRequest.method)
    val requestRetrier = JsonRpcRequestRetryer(vertx, delegate, config.copy(methodsToRetry = methodsToRetry))

    val result = requestRetrier.makeRequest(ethBlockNumberRequest).get()
    assertThat(result).isEqualTo(Ok(JsonRpcSuccessResponse("1", "0x1")))

    verify(delegate, times(3)).makeRequest(eq(ethBlockNumberRequest), anyOrNull())
  }

  @Test
  fun `should NOT retry request when not in retries list`() {
    val methodsToRetry = emptySet<String>()
    val requestRetrier = JsonRpcRequestRetryer(vertx, delegate, config.copy(methodsToRetry = methodsToRetry))

    val error = assertThrows<Exception> { requestRetrier.makeRequest(ethBlockNumberRequest).get() }
    assertThat(error.cause).isEqualTo(networkError1)

    verify(delegate, times(1)).makeRequest(eq(ethBlockNumberRequest), anyOrNull())
  }

  @Test
  fun `should retry request until stop condition is satisfied`() {
    val methodsToRetry = setOf(ethBlockNumberRequest.method)
    val requestRetrier = JsonRpcRequestRetryer(vertx, delegate, config.copy(methodsToRetry = methodsToRetry))

    val result = requestRetrier.makeRequest(
      ethBlockNumberRequest,
      stopRetriesPredicate = { result -> result is Ok && result.value.result == "0x2" },
    ).get()

    assertThat(result).isEqualTo(Ok(JsonRpcSuccessResponse("1", "0x2")))

    verify(delegate, times(4)).makeRequest(eq(ethBlockNumberRequest), anyOrNull())
  }

  @Test
  fun `when max retries are elapsed shall return last error - promise rejected`() {
    val methodsToRetry = setOf(ethBlockNumberRequest.method)
    val alwaysDownEndpoint = mock<JsonRpcClient>() {
      on { makeRequest(any(), anyOrNull()) }
        .doReturn(Future.failedFuture(networkError1))
        .doReturn(Future.failedFuture(networkError2))
    }
    val requestRetryer =
      JsonRpcRequestRetryer(vertx, alwaysDownEndpoint, config.copy(methodsToRetry = methodsToRetry))

    val error = assertThrows<Exception> { requestRetryer.makeRequest(ethBlockNumberRequest).get() }
    assertThat(error.cause).isEqualTo(networkError2)

    verify(alwaysDownEndpoint, times(maxRetries + 1)).makeRequest(eq(ethBlockNumberRequest), anyOrNull())
  }

  @Test
  fun `when max retries are elapsed shall return last error - JsonRpcErrorResponse`() {
    val methodsToRetry = setOf(ethBlockNumberRequest.method)
    val alwaysDownEndpoint = mock<JsonRpcClient>() {
      on { makeRequest(any(), anyOrNull()) }
        .doReturn(Future.succeededFuture(Err(JsonRpcErrorResponse("1", JsonRpcError.internalError("Forced error 1")))))
        .doReturn(Future.succeededFuture(Err(JsonRpcErrorResponse("1", JsonRpcError.internalError("Forced error 2")))))
    }
    val requestRetryer =
      JsonRpcRequestRetryer(vertx, alwaysDownEndpoint, config.copy(methodsToRetry = methodsToRetry))

    val result = requestRetryer.makeRequest(ethBlockNumberRequest).get()
    assertThat(result).isEqualTo(Err(JsonRpcErrorResponse("1", JsonRpcError.internalError("Forced error 2"))))

    verify(alwaysDownEndpoint, times(maxRetries + 1)).makeRequest(eq(ethBlockNumberRequest), anyOrNull())
  }

  @Test
  fun `when max retries are elapsed and stop condition not met shall return error`() {
    val methodsToRetry = setOf(ethBlockNumberRequest.method)
    val requestRetrier = JsonRpcRequestRetryer(vertx, delegate, config.copy(methodsToRetry = methodsToRetry))

    val error = assertThrows<Exception> {
      requestRetrier.makeRequest(
        ethBlockNumberRequest,
        stopRetriesPredicate = { result -> result is Ok && result.value.result == "0x100" },
      )
        .get()
    }
    assertThat(error.cause).isInstanceOf(RetriedExecutionException::class.java)

    verify(delegate, times(maxRetries + 1)).makeRequest(eq(ethBlockNumberRequest), anyOrNull())
  }

  @Test
  fun `should log warning message every threshold failures`() {
    val methodsToRetry = setOf(ethBlockNumberRequest.method)
    val alwaysDownEndpoint = mock<JsonRpcClient>() {
      on { makeRequest(any(), anyOrNull()) }.doReturn(Future.failedFuture(networkError1))
    }
    val log: Logger = spy(LogManager.getLogger("unit-test-logger"))
    val requestRetryer =
      JsonRpcRequestRetryer(
        vertx,
        alwaysDownEndpoint,
        config.copy(
          methodsToRetry = methodsToRetry,
          requestRetry = config.requestRetry.copy(failuresWarningThreshold = 2u),
        ),
        log = log,
        failuresLogLevel = Level.INFO,
      )

    val error = assertThrows<Exception> { requestRetryer.makeRequest(ethBlockNumberRequest).get() }
    assertThat(error.cause).isEqualTo(networkError1)

    verify(log).log(
      eq(Level.INFO),
      eq("Request '{}' already retried {} times. lastError={}"),
      eq("""{"jsonrpc":"2.0","id":"1","method":"eth_blockNumber","params":["0x1"]}"""),
      eq(2),
      eq(networkError1),
    )
    verify(log).log(
      eq(Level.INFO),
      eq("Request '{}' already retried {} times. lastError={}"),
      eq("""{"jsonrpc":"2.0","id":"1","method":"eth_blockNumber","params":["0x1"]}"""),
      eq(4),
      eq(networkError1),
    )
  }

  @Test
  fun `should not log warning message when retriesWarningThreshold=0`() {
    val methodsToRetry = setOf(ethBlockNumberRequest.method)
    val alwaysDownEndpoint = mock<JsonRpcClient>() {
      on { makeRequest(any(), anyOrNull()) }.doReturn(Future.failedFuture(networkError1))
    }
    val log: Logger = spy(LogManager.getLogger("unit-test-logger"))
    val requestRetryer =
      JsonRpcRequestRetryer(
        vertx,
        alwaysDownEndpoint,
        config.copy(
          methodsToRetry = methodsToRetry,
          requestRetry = config.requestRetry.copy(failuresWarningThreshold = 0u),
        ),
        log = log,
        failuresLogLevel = Level.INFO,
      )

    val error = assertThrows<Exception> { requestRetryer.makeRequest(ethBlockNumberRequest).get() }
    assertThat(error.cause).isEqualTo(networkError1)

    verify(log, never()).log(
      eq(Level.INFO),
      any<String>(),
      any<String>(),
      any(),
      eq(networkError1),
    )
  }
}

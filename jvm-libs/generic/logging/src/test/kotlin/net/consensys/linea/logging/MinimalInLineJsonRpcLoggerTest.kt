package net.consensys.linea.logging

import org.apache.logging.log4j.Level
import org.apache.logging.log4j.Logger
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.Mockito.mock
import org.mockito.kotlin.any
import org.mockito.kotlin.anyOrNull
import org.mockito.kotlin.eq
import org.mockito.kotlin.never
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever

class MinimalInLineJsonRpcLoggerTest {
  private lateinit var logger: Logger
  private lateinit var minimalInLineJsonRpcLogger: MinimalInLineJsonRpcLogger
  private val jsonRequestBody =
    """{"jsonrpc":"2.0","id":"53","method":"eth_getBlockByNumber","params":["latest", true]}"""
  private val jsonSuccessResponse = """{"jsonrpc":"2.0","id":"53","result":"0x35"}"""
  private val jsonErrorResponse = """{"jsonrpc":"2.0","id":89,"error":{"code":-32001,"message":"Nonce too low"}}"""

  @BeforeEach
  fun setUp() {
    logger = mock()
    minimalInLineJsonRpcLogger = MinimalInLineJsonRpcLogger(
      logger,
      requestResponseLogLevel = Level.DEBUG,
      failuresLogLevel = Level.WARN
    )
    whenever(logger.level).thenReturn(Level.INFO)
  }

  @Test
  fun `logRequest logs request with correct level and format`() {
    minimalInLineJsonRpcLogger.logRequest("testEndpoint", jsonRequestBody)

    verify(logger).log(eq(Level.DEBUG), eq("--> {} {}"), eq("testEndpoint"), eq(jsonRequestBody))
  }

  @Test
  fun `logRequest logs request with correct level and format - failure`() {
    val error = RuntimeException("Http client error")
    minimalInLineJsonRpcLogger.logRequest("testEndpoint", jsonRequestBody, error)

    verify(logger).log(
      eq(Level.DEBUG),
      eq("--> {} {} failed with error={}"),
      eq("testEndpoint"),
      eq(jsonRequestBody),
      eq("Http client error"),
      eq(error)
    )
  }

  @Test
  fun `logResponse logs response without error correctly`() {
    minimalInLineJsonRpcLogger.logResponse("testEndpoint", 200, jsonRequestBody, jsonSuccessResponse, null)

    verify(logger).log(
      eq(Level.DEBUG),
      eq("<-- {} {} {}"),
      eq("testEndpoint"),
      eq(200),
      eq(jsonSuccessResponse)
    )
  }

  @Test
  fun `logResponse logs response and request with error correctly`() {
    val exception = RuntimeException("Test exception")
    minimalInLineJsonRpcLogger.logResponse("testEndpoint", 500, jsonRequestBody, jsonErrorResponse, exception)

    verify(logger).log(eq(Level.WARN), eq("--> {} {}"), eq("testEndpoint"), eq(jsonRequestBody))
    verify(logger).log(
      eq(Level.WARN),
      eq("<-- {} {} {} failed with error={}"),
      eq("testEndpoint"),
      eq(500),
      eq(jsonErrorResponse),
      eq("Test exception")
    )
  }

  @Test
  fun `logResponse logs response and response only once when in debug`() {
    val exception = RuntimeException("Test exception")
    minimalInLineJsonRpcLogger = MinimalInLineJsonRpcLogger(
      logger,
      requestResponseLogLevel = Level.DEBUG,
      failuresLogLevel = Level.WARN
    )
    whenever(logger.level).thenReturn(Level.DEBUG)
    minimalInLineJsonRpcLogger.logResponse("testEndpoint", 500, jsonRequestBody, jsonErrorResponse, exception)

    verify(logger, never()).log(
      any(),
      eq("--> {} {}"),
      eq("testEndpoint"),
      eq(jsonRequestBody),
      anyOrNull(),
      anyOrNull()
    )
    verify(logger).log(
      eq(Level.WARN),
      eq("<-- {} {} {} failed with error={}"),
      eq("testEndpoint"),
      eq(500),
      eq(jsonErrorResponse),
      eq("Test exception")
    )
  }

  @Test
  fun `isJsonRpcError returns true for non-200 status codes`() {
    val isError = minimalInLineJsonRpcLogger.isJsonRpcError(500, """{"error":"Internal Server Error"}""")

    assertThat(isError).isEqualTo(true)
  }

  @Test
  fun `isJsonRpcError returns true for 200 status code with error in body`() {
    val isError = minimalInLineJsonRpcLogger.isJsonRpcError(200, """{"error":"Some error"}""")

    assertThat(isError).isEqualTo(true)
  }

  @Test
  fun `isJsonRpcError returns false for 200 status code without error in body`() {
    val isError = minimalInLineJsonRpcLogger.isJsonRpcError(200, """{"result":"success"}""")

    assert(!isError)
  }
}

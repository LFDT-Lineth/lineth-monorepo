package net.consensys.linea.jsonrpc.client

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import io.vertx.core.Future
import net.consensys.linea.async.get
import net.consensys.linea.jsonrpc.JsonRpcErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcRequestListParams
import net.consensys.linea.jsonrpc.JsonRpcSuccessResponse
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.kotlin.anyOrNull
import org.mockito.kotlin.eq
import org.mockito.kotlin.mock
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever

class JsonRpcRequestFanOutTest {
  private lateinit var rpcClient1: JsonRpcClient
  private lateinit var rpcClient2: JsonRpcClient
  private lateinit var rpcClient3: JsonRpcClient
  private lateinit var rpcClientFanOut: JsonRpcRequestFanOut
  private val request = JsonRpcRequestListParams("2.0", "1", "miner_setMinGasPrice", listOf("0x1"))

  @BeforeEach
  fun beforeEach() {
    rpcClient1 = mock {}
    rpcClient2 = mock {}
    rpcClient3 = mock {}
    rpcClientFanOut = JsonRpcRequestFanOut(listOf(rpcClient1, rpcClient2, rpcClient3))
  }

  @Test
  fun `fanoutRequests - returns list with responses`() {
    whenever(rpcClient1.makeRequest(eq(request), anyOrNull()))
      .thenReturn(Future.succeededFuture(Ok(JsonRpcSuccessResponse("1", 1))))
    whenever(rpcClient2.makeRequest(eq(request), anyOrNull()))
      .thenReturn(Future.succeededFuture(Ok(JsonRpcSuccessResponse("1", 2))))
    whenever(rpcClient3.makeRequest(eq(request), anyOrNull()))
      .thenReturn(Future.succeededFuture(Ok(JsonRpcSuccessResponse("1", 3))))

    assertThat(rpcClientFanOut.fanoutRequest(request).get()).isEqualTo(
      listOf(
        Ok(JsonRpcSuccessResponse("1", 1)),
        Ok(JsonRpcSuccessResponse("1", 2)),
        Ok(JsonRpcSuccessResponse("1", 3)),
      ),
    )
    verify(rpcClient1).makeRequest(eq(request), anyOrNull())
    verify(rpcClient2).makeRequest(eq(request), anyOrNull())
    verify(rpcClient3).makeRequest(eq(request), anyOrNull())
  }

  @Test
  fun `makeRequest - when all responses are successful should return the first successful response`() {
    whenever(rpcClient1.makeRequest(eq(request), anyOrNull()))
      .thenReturn(Future.succeededFuture(Ok(JsonRpcSuccessResponse("1", 1))))
    whenever(rpcClient2.makeRequest(eq(request), anyOrNull()))
      .thenReturn(Future.succeededFuture(Ok(JsonRpcSuccessResponse("1", 2))))
    whenever(rpcClient3.makeRequest(eq(request), anyOrNull()))
      .thenReturn(Future.succeededFuture(Ok(JsonRpcSuccessResponse("1", 3))))

    assertThat(rpcClientFanOut.makeRequest(request).get()).isEqualTo(Ok(JsonRpcSuccessResponse("1", 1)))
    verify(rpcClient1).makeRequest(eq(request), anyOrNull())
    verify(rpcClient2).makeRequest(eq(request), anyOrNull())
    verify(rpcClient3).makeRequest(eq(request), anyOrNull())
  }

  @Test
  fun `makeRequest - when one response is error, should return the first error response`() {
    whenever(rpcClient1.makeRequest(eq(request), anyOrNull()))
      .thenReturn(Future.succeededFuture(Ok(JsonRpcSuccessResponse("1", 1))))
    whenever(rpcClient2.makeRequest(eq(request), anyOrNull()))
      .thenReturn(Future.succeededFuture(Err(JsonRpcErrorResponse.internalError("1", "2"))))
    whenever(rpcClient3.makeRequest(eq(request), anyOrNull()))
      .thenReturn(Future.succeededFuture(Err(JsonRpcErrorResponse.internalError("1", "3"))))

    assertThat(rpcClientFanOut.makeRequest(request).get()).isEqualTo(Err(JsonRpcErrorResponse.internalError("1", "2")))
    verify(rpcClient1).makeRequest(eq(request), anyOrNull())
    verify(rpcClient2).makeRequest(eq(request), anyOrNull())
    verify(rpcClient3).makeRequest(eq(request), anyOrNull())
  }
}

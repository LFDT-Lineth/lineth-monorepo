package net.consensys.linea.jsonrpc.client

import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import com.github.michaelbull.result.unwrap
import io.vertx.core.Future
import io.vertx.core.Promise
import net.consensys.linea.async.get
import net.consensys.linea.jsonrpc.JsonRpcErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcRequest
import net.consensys.linea.jsonrpc.JsonRpcRequestListParams
import net.consensys.linea.jsonrpc.JsonRpcSuccessResponse
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.kotlin.any
import org.mockito.kotlin.mock
import org.mockito.kotlin.times
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import org.mockito.stubbing.Answer
import java.lang.Exception
import java.util.concurrent.CountDownLatch
import java.util.concurrent.CyclicBarrier
import java.util.concurrent.Executors
import java.util.concurrent.ThreadLocalRandom
import java.util.concurrent.atomic.AtomicInteger
import kotlin.concurrent.timer

class LoadBalancingJsonRpcClientTest {
  private lateinit var rpcClient2: JsonRpcClient
  private lateinit var rpcClient1: JsonRpcClient
  private lateinit var loadBalancer: LoadBalancingJsonRpcClient
  private val maxInflightRequestsPerClient = 2u
  private val requestId: AtomicInteger = AtomicInteger(0)
  private fun rpcRequest(
    method: String = "eth_blockNumber",
    params: List<Any> = emptyList(),
  ): JsonRpcRequestListParams = JsonRpcRequestListParams("2.0", requestId.incrementAndGet(), method, params)

  @BeforeEach
  fun beforeEach() {
    rpcClient1 = mock()
    rpcClient2 = mock()
    loadBalancer =
      LoadBalancingJsonRpcClient.create(listOf(rpcClient1, rpcClient2), maxInflightRequestsPerClient)
  }

  @AfterEach fun afterEach() {}

  @Test
  fun uses_available_client() {
    rpcClient1.replyWithDelay(10, result(1, "client-1-response"))
    val future = loadBalancer.makeRequest(rpcRequest())
    assertThat(future.get()).isEqualTo(result(1, "client-1-response"))
  }

  @Test
  fun uses_next_free_client_fore_reusing_same_one() {
    rpcClient1.replyWithDelay(100, result(1, "client-1-result"))
    rpcClient2.replyWithDelay(10, result(1, "client-2-result"))
    val future1 = loadBalancer.makeRequest(rpcRequest())
    val future2 = loadBalancer.makeRequest(rpcRequest())
    assertThat(future1.get()).isEqualTo(result(1, "client-1-result"))
    assertThat(future2.get()).isEqualTo(result(1, "client-2-result"))
  }

  @Test
  fun does_not_go_over_inflight_limit() {
    val c1P1 = Promise.promise<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>()
    val c1P2 = Promise.promise<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>()
    val c1P3 = Promise.promise<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>()
    val c2P1 = Promise.promise<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>()
    val c2P2 = Promise.promise<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>()
    val c2P3 = Promise.promise<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>()
    whenever(rpcClient1.makeRequest(any(), any()))
      .thenReturn(c1P1.future())
      .thenReturn(c1P2.future())
      .thenReturn(c1P3.future())
    whenever(rpcClient2.makeRequest(any(), any()))
      .thenReturn(c2P1.future())
      .thenReturn(c2P2.future())
      .thenReturn(c2P3.future())

    val f1 = loadBalancer.makeRequest(rpcRequest())
    val f2 = loadBalancer.makeRequest(rpcRequest())
    val f3 = loadBalancer.makeRequest(rpcRequest())
    val f4 = loadBalancer.makeRequest(rpcRequest())
    loadBalancer.makeRequest(rpcRequest())
    loadBalancer.makeRequest(rpcRequest())
    verify(rpcClient1, times(2)).makeRequest(any(), any())
    verify(rpcClient2, times(2)).makeRequest(any(), any())

    // resolve client 1, 1st inflight request
    c1P1.complete(result(1, "client-1-result-1"))
    verify(rpcClient1, times(3)).makeRequest(any(), any())
    assertThat(f1.get()).isEqualTo(result(1, "client-1-result-1"))

    // resolve client 2, 1st inflight request
    c2P1.complete(result(2, "client-2-result-1"))
    verify(rpcClient2, times(3)).makeRequest(any(), any())
    assertThat(f2.get()).isEqualTo(result(2, "client-2-result-1"))

    // resolve client 1, 2nd inflight request
    c1P2.complete(result(3, "client-1-result-2"))
    verify(rpcClient1, times(3)).makeRequest(any(), any())
    assertThat(f3.get()).isEqualTo(result(3, "client-1-result-2"))

    // resolve client 2, 2nd inflight request
    c2P2.complete(result(4, "client-2-result-2"))
    verify(rpcClient1, times(3)).makeRequest(any(), any())
    assertThat(f4.get()).isEqualTo(result(4, "client-2-result-2"))
  }

  @Test
  fun threadSafe_MoreClientsThanRequestingThreads() {
    val numberOfRpcClients = 100
    val maxInflightRequestsPerRpcClient = 5u
    val numberOfThreads = 20
    val numberOfRequestPerThread = 2
    testThreadSafe(
      numberOfRpcClients,
      maxInflightRequestsPerRpcClient,
      numberOfThreads,
      numberOfRequestPerThread,
    )
  }

  @Test
  fun threadSafe_HigherInflightLimitThanRequestingThreads() {
    val numberOfRpcClients = 5
    val maxInflightRequestsPerRpcClient = 50u
    val numberOfThreads = 10
    val numberOfRequestPerThread = 10
    testThreadSafe(
      numberOfRpcClients,
      maxInflightRequestsPerRpcClient,
      numberOfThreads,
      numberOfRequestPerThread,
    )
  }

  @Test
  fun threadSafe_LowerInflightLimitThanRequestingThreads() {
    val numberOfRpcClients = 5
    val maxInflightRequestsPerRpcClient = 3u
    val numberOfThreads = 10
    val numberOfRequestPerThread = 20
    testThreadSafe(
      numberOfRpcClients,
      maxInflightRequestsPerRpcClient,
      numberOfThreads,
      numberOfRequestPerThread,
    )
  }

  private fun testThreadSafe(
    numberOfRpcClients: Int,
    maxInflightRequestsPerRpcClient: UInt,
    numberOfThreads: Int,
    numberOfRequestPerThread: Int,
  ) {
    val executor = Executors.newCachedThreadPool()
    val producersStartBarrier = CyclicBarrier(numberOfThreads + 1)
    val totalRequestsToWait = numberOfThreads * numberOfRequestPerThread
    val receivedResponsesLatch = CountDownLatch(totalRequestsToWait)

    val rpcClients = (1..numberOfRpcClients).map { FakeJsonRpcClient(it) }
    val loadBalancer = LoadBalancingJsonRpcClient.create(rpcClients, maxInflightRequestsPerRpcClient)
    val requestProducers =
      (1..numberOfThreads).map {
        RequestProducer(
          it,
          loadBalancer,
          numberOfRequestPerThread,
          producersStartBarrier,
          receivedResponsesLatch,
        )
      }
    for (t in 1..numberOfThreads) {
      executor.execute(requestProducers[t - 1])
    }
    producersStartBarrier.await() // wait for all threads to make the requests;
    receivedResponsesLatch.await() // wait all threads to receive the responses;
    assertThat(loadBalancer.inflightRequestsCount()).isEqualTo(0L)
    assertThat(loadBalancer.queuedRequests()).isEmpty()
    requestProducers.forEach { requestProducer ->
      assertThat(requestProducer.responsesHandled()).isEqualTo(numberOfRequestPerThread)
    }
  }

  private class RequestProducer(
    val id: Int,
    val loadBalancer: LoadBalancingJsonRpcClient,
    val numberOfRequests: Int,
    val startBarrier: CyclicBarrier,
    val responsesReceivedLatch: CountDownLatch,
  ) : Runnable {
    private var responsesHandledAtomic = AtomicInteger(0)
    fun responsesHandled(): Int = responsesHandledAtomic.get()
    override fun run() {
      val threadRandom = ThreadLocalRandom.current()
      startBarrier.await()
      for (req in 1..numberOfRequests) {
        val responseDelay = threadRandom.nextLong(5, 100)
        val shallFail: Boolean = threadRandom.nextBoolean()
        val futureHandlerShallThrow: Boolean = threadRandom.nextInt(1, 4) > 2 // only 1/3 of errors
        val requestId = "${id}_$req"
        loadBalancer
          .makeRequest(
            JsonRpcRequestListParams("2.0", requestId, "sleepMs", listOf(responseDelay, shallFail)),
          )
          .map {
            if (futureHandlerShallThrow) {
              throw Error("Bad handler")
            }
            it
          }
          .onComplete { asyncResult ->
            responsesHandledAtomic.incrementAndGet()
            responsesReceivedLatch.countDown()
            assertThat(asyncResult.failed()).isEqualTo(shallFail || futureHandlerShallThrow)
            asyncResult.result()?.let { assertThat(it.unwrap().id).isEqualTo(requestId) }
          }
      }
    }
  }

  private class FakeJsonRpcClient(val id: Int) : JsonRpcClient {
    override fun makeRequest(
      request: JsonRpcRequest,
      resultMapper: (Any?) -> Any?,
    ): Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>> {
      val promise = Promise.promise<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>()
      val paramsList = request.params as List<*>
      val delayInMilliseconds = paramsList[0] as Long
      val shallFail = paramsList[1] as Boolean

      timer("rcp-client-$id", false, delayInMilliseconds, 100L) {
        if (!promise.future().isComplete) {
          if (shallFail) {
            promise.fail(Exception("Connection timeout"))
          } else {
            promise.complete(Ok(JsonRpcSuccessResponse(request.id, "wokeUp")))
          }
        }
        this.cancel()
      }
      return promise.future()
    }

    override fun toString(): String = "FAKE_CLIENT_$id"
  }

  private fun JsonRpcClient.replyWithDelay(
    delayInMilliseconds: Long,
    result: Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>,
  ): Promise<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>> {
    val promise = Promise.promise<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>()
    val answer: Answer<Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>> = Answer {
      timer("rcp-client", false, delayInMilliseconds, 100L) {
        if (!promise.future().isComplete) {
          promise.complete(result)
        }
        this.cancel()
      }

      promise.future() as Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>
    }
    whenever(this.makeRequest(any(), any())).thenAnswer(answer)
    return promise
  }

  private fun result(id: Any, result: Any?): Result<JsonRpcSuccessResponse, JsonRpcErrorResponse> =
    Ok(JsonRpcSuccessResponse(id, result))
}

package net.consensys.linea.jsonrpc.client

import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import com.github.michaelbull.result.unwrap
import io.vertx.core.Future
import io.vertx.core.Promise
import net.consensys.linea.async.get
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.jsonrpc.JsonRpcErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcRequest
import net.consensys.linea.jsonrpc.JsonRpcRequestListParams
import net.consensys.linea.jsonrpc.JsonRpcSuccessResponse
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.RepeatedTest
import org.junit.jupiter.api.RepetitionInfo
import org.junit.jupiter.api.Test
import org.mockito.kotlin.any
import org.mockito.kotlin.mock
import org.mockito.kotlin.times
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import org.mockito.stubbing.Answer
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.CountDownLatch
import java.util.concurrent.CyclicBarrier
import java.util.concurrent.Executors
import java.util.concurrent.ThreadLocalRandom
import java.util.concurrent.atomic.AtomicInteger
import kotlin.concurrent.atomics.AtomicLong
import kotlin.concurrent.atomics.ExperimentalAtomicApi
import kotlin.concurrent.atomics.incrementAndFetch
import kotlin.concurrent.timer
import kotlin.math.log
import kotlin.time.Duration.Companion.milliseconds

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

  /**
   * {"jsonrpc":"2.0","id":34923,"method":"linea_getBlockTracesCountersV2","params":[{"blockNumber":22644557,"expectedTracesEngineVersion":"beta-v2.1-rc16.2"}]}
   */
  private fun tracesCountersRequest(blockNumber: Int): JsonRpcRequestListParams = rpcRequest(
    "linea_getBlockTracesCountersV2",
    listOf(mapOf("blockNumber" to blockNumber)),
  )

  @BeforeEach
  fun beforeEach() {
    rpcClient1 = mock()
    rpcClient2 = mock()
    loadBalancer =
      LoadBalancingJsonRpcClient.create(listOf(rpcClient1, rpcClient2), maxInflightRequestsPerClient)
  }

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

  @RepeatedTest(2)
  @Suppress("UNCHECKED_CAST")
  fun should_fire_request_by_priority(repetitionInfo: RepetitionInfo) {
    val requests = (1..20).map { tracesCountersRequest(blockNumber = it) }
    val requestsCallOrder = requests.reversed()

    val requestsReceivedByClients = CopyOnWriteArrayList<JsonRpcRequest>()
    val requestHandler = { request: JsonRpcRequest ->
      JsonRpcSuccessResponse(request.id, "success")
    }
    val client1 = FakeJsonRpcHandler(
      requestHandled = requestsReceivedByClients,
      defaultResponseDelay = 10.milliseconds,
      defaultResponseSupplier = requestHandler,
    ).apply {
      // 1st N requests should have high delay, otherwise it's replies arrive before
      // they all request are sent LoadBalancer queue
      requestsCallOrder.take(4).forEach { onRequest(it, 300.milliseconds, requestHandler) }
    }
    val client2 = FakeJsonRpcHandler(
      requestHandled = requestsReceivedByClients,
      defaultResponseDelay = 20.milliseconds,
      defaultResponseSupplier = requestHandler,
    ).apply {
      requestsCallOrder.take(4).forEach { onRequest(it, 300.milliseconds, requestHandler) }
    }
    // Custom logger to help debugging
    val log = LogManager.getLogger(
      "net.consensys.linea.jsonrpc.LoadBalancingJsonRpcClient-${repetitionInfo.currentRepetition}-",
    )
    loadBalancer = LoadBalancingJsonRpcClient.create(
      rpcClients = listOf(client1, client2),
      requestLimitPerEndpoint = 1u,
      requestPriorityComparator = { o1, o2 ->
        val bn1 = (o1.params as List<Map<String, Int>>).first()["blockNumber"]!!
        val bn2 = (o2.params as List<Map<String, Int>>).first()["blockNumber"]!!
        bn1.compareTo(bn2)
      },
      log = log,
    )

    // send requests in reverse order
    requests.reversed()
      .map { loadBalancer.makeRequest(it).toSafeFuture() }
      // .also { log.trace("before collection") }
      .let { SafeFuture.collectAll(it.stream()).get() }

    // assert that queued requests were fired in correct priority
    // Expected order:
    // 20 -- fired right away to client1, que is empty
    // 19 -- fired right away to client2, que is empty
    // 18..17 -- may be queued or fired, depends on client1/client2 response time and thread scheduling
    // 1,2,3,...17 were queued and fired regarding priority
    assertThat(requestsReceivedByClients.take(2)).isEqualTo(requests.takeLast(2).reversed())
    assertThat(requestsReceivedByClients.drop(4))
      .containsSubsequence(requests.drop(4).dropLast(4))
  }

  @Test
  fun should_fire_requests_by_fifo_when_comparator_throws() {
    val requestsReceivedByClients = CopyOnWriteArrayList<JsonRpcRequest>()
    val requestHandler = { request: JsonRpcRequest ->
      JsonRpcSuccessResponse(request.id, "success")
    }
    val client1 = FakeJsonRpcHandler(
      requestHandled = requestsReceivedByClients,
      defaultResponseDelay = 10.milliseconds,
      defaultResponseSupplier = requestHandler,
    )
    val client2 = FakeJsonRpcHandler(
      requestHandled = requestsReceivedByClients,
      defaultResponseDelay = 10.milliseconds,
      defaultResponseSupplier = requestHandler,
    )

    loadBalancer = LoadBalancingJsonRpcClient.create(
      rpcClients = listOf(client1, client2),
      requestLimitPerEndpoint = 1u,
      requestPriorityComparator = object : Comparator<JsonRpcRequest> {
        override fun compare(o1: JsonRpcRequest, o2: JsonRpcRequest): Int {
          throw RuntimeException("Ups, cant parse this request")
        }
      },
    )
    val requests = (1..10).map {
      tracesCountersRequest(blockNumber = it)
    }
    // send requests in reverse order
    requests.reversed()
      .map { loadBalancer.makeRequest(it).toSafeFuture() }
      .let { SafeFuture.collectAll(it.stream()).get() }

    // assert that queued requests were fired in correct priority
    // Expected order: 1..10
    assertThat(requestsReceivedByClients).isEqualTo(requests.reversed())
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

  @Test
  fun `verify that if first node in list does not becomes the sticky client`() {
    val numberOfRpcClients = 5
    val maxInflightRequestsPerRpcClient = 3u
    val numberOfThreads = 10
    val numberOfRequestPerThread = 20
    val alwaysFailFakeJsonRpcClient = AlwaysFailFakeJsonRpcClient(1)
    val fakeJsonRpcClients = (2..numberOfRpcClients).map { FakeJsonRpcClient(it) }
    val rpcClients: List<JsonRpcClient> = listOf(alwaysFailFakeJsonRpcClient) + fakeJsonRpcClients
    testThreadSafe(
      numberOfRpcClients,
      maxInflightRequestsPerRpcClient,
      numberOfThreads,
      numberOfRequestPerThread,
      rpcClients,
    )
    val totalRequests = numberOfThreads * numberOfRequestPerThread
    assertThat(alwaysFailFakeJsonRpcClient.getRequestsHandled() + fakeJsonRpcClients.sumOf { it.getRequestsHandled() })
      .isEqualTo(totalRequests.toLong())
    // assert that the first client did not handle all requests
    assertThat(alwaysFailFakeJsonRpcClient.getRequestsHandled()).isLessThan(totalRequests.toLong())
    fakeJsonRpcClients.forEach { assertThat(it.getRequestsHandled()).isGreaterThan(0L) }
  }

  private fun testThreadSafe(
    numberOfRpcClients: Int,
    maxInflightRequestsPerRpcClient: UInt,
    numberOfThreads: Int,
    numberOfRequestPerThread: Int,
    rpcClients: List<JsonRpcClient> = (1..numberOfRpcClients).map { FakeJsonRpcClient(it) },
  ) {
    val executor = Executors.newCachedThreadPool()
    val producersStartBarrier = CyclicBarrier(numberOfThreads + 1)
    val totalRequestsToWait = numberOfThreads * numberOfRequestPerThread
    val receivedResponsesLatch = CountDownLatch(totalRequestsToWait)

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
    assertThat(loadBalancer.queuedRequestsCount()).isEqualTo(0)
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
            if (asyncResult.failed()) {
              val errorMessage = asyncResult.cause()?.message
              if ("ALWAYS_FAILING_FAKE_CLIENT" != errorMessage) {
                assertThat(shallFail || futureHandlerShallThrow).isEqualTo(true)
              }
            }
            asyncResult.result()?.let { assertThat(it.unwrap().id).isEqualTo(requestId) }
          }
      }
    }
  }

  private abstract class BaseFakeJsonRpcClient : JsonRpcClient {
    abstract fun getRequestsHandled(): Long
  }

  @OptIn(ExperimentalAtomicApi::class)
  private class AlwaysFailFakeJsonRpcClient(val id: Int) : BaseFakeJsonRpcClient() {
    private val requestsHandled = AtomicLong(0)
    override fun makeRequest(
      request: JsonRpcRequest,
      resultMapper: (Any?) -> Any?,
    ): Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>> {
      val promise = Promise.promise<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>()
      requestsHandled.incrementAndFetch()
      promise.fail(Exception("ALWAYS_FAILING_FAKE_CLIENT"))
      return promise.future()
    }

    override fun toString(): String = "ALWAYS_FAILING_FAKE_CLIENT_$id"
    override fun getRequestsHandled(): Long {
      return requestsHandled.load()
    }
  }

  @OptIn(ExperimentalAtomicApi::class)
  private class FakeJsonRpcClient(val id: Int) : BaseFakeJsonRpcClient() {
    private val requestsHandled = AtomicLong(0)

    override fun makeRequest(
      request: JsonRpcRequest,
      resultMapper: (Any?) -> Any?,
    ): Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>> {
      val promise = Promise.promise<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>()
      val paramsList = request.params as List<*>
      val delayInMilliseconds = paramsList[0] as Long
      val shallFail = paramsList[1] as Boolean

      requestsHandled.incrementAndFetch()

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
    override fun getRequestsHandled(): Long {
      return requestsHandled.load()
    }
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

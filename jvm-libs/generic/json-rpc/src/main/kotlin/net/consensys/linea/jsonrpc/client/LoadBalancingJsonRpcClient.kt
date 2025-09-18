package net.consensys.linea.jsonrpc.client

import com.github.michaelbull.result.Result
import io.vertx.core.AsyncResult
import io.vertx.core.Future
import io.vertx.core.Handler
import io.vertx.core.Promise
import net.consensys.linea.jsonrpc.JsonRpcErrorResponse
import net.consensys.linea.jsonrpc.JsonRpcRequest
import net.consensys.linea.jsonrpc.JsonRpcSuccessResponse
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import java.util.LinkedList
import java.util.Queue
import java.util.concurrent.ConcurrentLinkedQueue
import java.util.concurrent.locks.ReadWriteLock
import java.util.concurrent.locks.ReentrantReadWriteLock
import kotlin.concurrent.withLock

/**
 * Implements a Client Side LoadBalancer on round-robin for each JsonRpcClient. Each JsonRpcClient
 * can have up to maxInflightRequestsPerClient requests in progress. After all rpc clients reach
 * this limit request will queue and served in a LIFO order.
 *
 * It's expected that each JsonRpcClient represents a different upstream Endpoint e.g
 * prover1.linea.io:8080 and prover2.linea.io:8081
 */
class LoadBalancingJsonRpcClient
private constructor(
  rpcClients: List<JsonRpcClient>,
  private val maxInflightRequestsPerClient: UInt,
  private val requestPriorityComparator: Comparator<JsonRpcRequest>? = null,
  private val log: Logger = LogManager.getLogger(LoadBalancingJsonRpcClient::class.java),
) : JsonRpcClient {

  companion object {
    private val loadBalancingJsonRpcClients: ConcurrentLinkedQueue<LoadBalancingJsonRpcClient> = ConcurrentLinkedQueue()

    fun create(
      rpcClients: List<JsonRpcClient>,
      requestLimitPerEndpoint: UInt,
      requestPriorityComparator: Comparator<JsonRpcRequest>? = null,
      log: Logger = LogManager.getLogger(LoadBalancingJsonRpcClient::class.java),
    ): LoadBalancingJsonRpcClient {
      val loadBalancingJsonRpcClient = LoadBalancingJsonRpcClient(
        rpcClients = rpcClients,
        maxInflightRequestsPerClient = requestLimitPerEndpoint,
        requestPriorityComparator = requestPriorityComparator,
        log = log,
      )
      loadBalancingJsonRpcClients.add(loadBalancingJsonRpcClient)
      return loadBalancingJsonRpcClient
    }

    fun stop() {
      loadBalancingJsonRpcClients.forEach {
        it.close()
      }
    }
  }

  private data class RpcClientContext(
    val rpcClient: JsonRpcClient,
    var inflightRequests: UInt,
    var totalRequests: ULong,
  )
  internal data class RpcRequestContext(
    val request: JsonRpcRequest,
    val promise: Promise<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>,
    val resultMapper: (Any?) -> Any?,
  )

  private val clientsPool: List<RpcClientContext> = rpcClients.map { RpcClientContext(it, 0u, 0uL) }
  private val waitingQueue: Queue<RpcRequestContext> =
    if (requestPriorityComparator != null) {
      PriorityQueueWithFIFOFallback<RpcRequestContext>(comparator = { o1, o2 ->
        requestPriorityComparator.compare(o1.request, o2.request)
      })
    } else {
      LinkedList<RpcRequestContext>()
    }
  private val readWriteLock: ReadWriteLock = ReentrantReadWriteLock()

  internal fun queuedRequestsCount(): Int {
    readWriteLock.readLock().withLock {
      return waitingQueue.size
    }
  }

  fun inflightRequestsCount(): Long {
    return clientsPool.fold(0L) { acc, it -> acc + it.inflightRequests.toLong() }
  }

  override fun makeRequest(
    request: JsonRpcRequest,
    resultMapper: (Any?) -> Any?,
  ): Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>> {
    val result = enqueueRequest(request, resultMapper)
    serveNextWaitingInTheQueue()
    return result
  }

  private fun serveNextWaitingInTheQueue() {
    readWriteLock.readLock().withLock {
      if (waitingQueue.isEmpty()) return
    }

    readWriteLock.writeLock().withLock {
      val availableClients = clientsPool.filter {
        it.inflightRequests < maxInflightRequestsPerClient
      }
      if (availableClients.isEmpty()) {
        log.trace("All clients are busy, waitingQueue.size={}", waitingQueue.size)
      } else {
        val client = availableClients.minBy { it.totalRequests }
        // fetch waiting request from the queue
        waitingQueue.poll()?.let { request ->
          client.totalRequests++
          client.inflightRequests++
          log.trace("making request={}", request.request)
          // firing request inside the lock to guarantee order
          // otherwise thread scheduling may make them fire out of order,
          // does not matter much in prod, but results in flaky tests...
          client.rpcClient
            .makeRequest(request.request, request.resultMapper)
            .onComplete(requestResultHandler(client, request))
        }
      }
    }
  }

  private fun enqueueRequest(
    request: JsonRpcRequest,
    resultMapper: (Any?) -> Any?,
  ): Future<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>> {
    val resultPromise: Promise<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>> =
      Promise.promise()
    val requestContext = RpcRequestContext(request, resultPromise, resultMapper)
    log.trace("enqueuing request={}", request)
    readWriteLock.writeLock().withLock {
      waitingQueue.add(requestContext)
    }
    return resultPromise.future()
  }

  private fun requestResultHandler(
    rpcClientContext: RpcClientContext,
    queuedRequest: RpcRequestContext,
  ): Handler<AsyncResult<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>> {
    return Handler<AsyncResult<Result<JsonRpcSuccessResponse, JsonRpcErrorResponse>>> { asyncResult ->
      readWriteLock.writeLock().withLock {
        rpcClientContext.inflightRequests--
      }
      try {
        queuedRequest.promise.handle(asyncResult)
      } catch (e: Exception) {
        log.error("Response handler threw error:", e)
      } finally {
        serveNextWaitingInTheQueue()
      }
    }
  }

  fun close() {
    readWriteLock.writeLock().withLock(waitingQueue::clear)
  }
}

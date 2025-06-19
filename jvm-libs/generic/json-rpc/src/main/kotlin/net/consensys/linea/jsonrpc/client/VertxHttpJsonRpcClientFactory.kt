package net.consensys.linea.jsonrpc.client

import com.fasterxml.jackson.databind.ObjectMapper
import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Result
import io.vertx.core.Vertx
import io.vertx.core.http.HttpClientOptions
import io.vertx.core.http.HttpVersion
import net.consensys.linea.metrics.MetricsFacade
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import java.net.URI
import java.net.URL
import java.util.function.Predicate
import java.util.function.Supplier

interface JsonRpcClientFactory {
  /**
   * Creates a JSON-RPC V2 Spec client.
   * If multiple endpoints are provided, a load balancing client will be created with round-robin strategy.
   */
  fun createJsonRpcV2Client(
    endpoints: List<URI>,
    maxInflightRequestsPerClient: UInt? = null,
    retryConfig: RequestRetryConfig,
    httpVersion: HttpVersion? = null,
    requestObjectMapper: ObjectMapper = objectMapper,
    responseObjectMapper: ObjectMapper = objectMapper,
    requestTimeout: Long? = null,
    shallRetryRequestsClientBasePredicate: Predicate<Result<Any?, Throwable>> = Predicate { it is Err },
    log: Logger = LogManager.getLogger(VertxHttpJsonRpcClient::class.java),
    requestResponseLogLevel: Level = Level.TRACE,
    failuresLogLevel: Level = Level.DEBUG,
  ): JsonRpcV2Client
}

class VertxHttpJsonRpcClientFactory(
  private val vertx: Vertx,
  private val metricsFacade: MetricsFacade,
  private val requestResponseLogLevel: Level = Level.TRACE,
  private val failuresLogLevel: Level = Level.DEBUG,
  private val requestIdSupplier: Supplier<Any> = SequentialIdSupplier.singleton,
) : JsonRpcClientFactory {
  fun create(
    endpoint: URL,
    maxPoolSize: Int? = null,
    httpVersion: HttpVersion? = null,
    requestObjectMapper: ObjectMapper = objectMapper,
    responseObjectMapper: ObjectMapper = objectMapper,
    requestTimeout: Long? = null,
    log: Logger = LogManager.getLogger(VertxHttpJsonRpcClient::class.java),
    requestResponseLogLevel: Level = this.requestResponseLogLevel,
    failuresLogLevel: Level = this.failuresLogLevel,
  ): VertxHttpJsonRpcClient {
    val clientOptions =
      HttpClientOptions()
        .setKeepAlive(true)
        .setDefaultHost(endpoint.host)
        .setDefaultPort(endpoint.port)
    maxPoolSize?.let(clientOptions::setMaxPoolSize)
    httpVersion?.let(clientOptions::setProtocolVersion)
    val httpClient = vertx.createHttpClient(clientOptions)
    return VertxHttpJsonRpcClient(
      httpClient,
      endpoint,
      metricsFacade,
      log = log,
      requestParamsObjectMapper = requestObjectMapper,
      responseObjectMapper = responseObjectMapper,
      requestTimeout = requestTimeout,
      requestResponseLogLevel = requestResponseLogLevel,
      failuresLogLevel = failuresLogLevel,
    )
  }

  fun createWithLoadBalancing(
    endpoints: Set<URL>,
    maxInflightRequestsPerClient: UInt,
    httpVersion: HttpVersion? = null,
    requestObjectMapper: ObjectMapper = objectMapper,
    responseObjectMapper: ObjectMapper = objectMapper,
    requestTimeout: Long? = null,
    log: Logger = LogManager.getLogger(VertxHttpJsonRpcClient::class.java),
    requestResponseLogLevel: Level = this.requestResponseLogLevel,
    failuresLogLevel: Level = this.failuresLogLevel,
  ): JsonRpcClient {
    return LoadBalancingJsonRpcClient.create(
      endpoints.map { endpoint ->
        create(
          endpoint = endpoint,
          maxPoolSize = maxInflightRequestsPerClient.toInt(),
          httpVersion = httpVersion,
          requestObjectMapper = requestObjectMapper,
          responseObjectMapper = responseObjectMapper,
          requestTimeout = requestTimeout,
          log = log,
          requestResponseLogLevel = requestResponseLogLevel,
          failuresLogLevel = failuresLogLevel,
        )
      },
      maxInflightRequestsPerClient,
    )
  }

  fun createWithRetries(
    endpoint: URL,
    maxPoolSize: Int? = null,
    retryConfig: RequestRetryConfig,
    methodsToRetry: Set<String>,
    httpVersion: HttpVersion? = null,
    requestObjectMapper: ObjectMapper = objectMapper,
    responseObjectMapper: ObjectMapper = objectMapper,
    requestTimeout: Long? = null,
    log: Logger = LogManager.getLogger(VertxHttpJsonRpcClient::class.java),
    requestResponseLogLevel: Level = this.requestResponseLogLevel,
    failuresLogLevel: Level = this.failuresLogLevel,
  ): JsonRpcClient {
    val rpcClient = create(
      endpoint = endpoint,
      maxPoolSize = maxPoolSize,
      httpVersion = httpVersion,
      requestObjectMapper = requestObjectMapper,
      responseObjectMapper = responseObjectMapper,
      requestTimeout = requestTimeout,
      log = log,
      requestResponseLogLevel = requestResponseLogLevel,
      failuresLogLevel = failuresLogLevel,
    )

    return JsonRpcRequestRetryer(
      this.vertx,
      rpcClient,
      config = JsonRpcRequestRetryer.Config(
        methodsToRetry = methodsToRetry,
        requestRetry = retryConfig,
      ),
      log = log,
    )
  }

  fun createWithLoadBalancingAndRetries(
    vertx: Vertx,
    endpoints: Set<URL>,
    maxInflightRequestsPerClient: UInt,
    retryConfig: RequestRetryConfig,
    methodsToRetry: Set<String>,
    httpVersion: HttpVersion? = null,
    requestObjectMapper: ObjectMapper = objectMapper,
    responseObjectMapper: ObjectMapper = objectMapper,
    requestTimeout: Long? = null,
    log: Logger = LogManager.getLogger(VertxHttpJsonRpcClient::class.java),
    requestResponseLogLevel: Level = this.requestResponseLogLevel,
    failuresLogLevel: Level = this.failuresLogLevel,
  ): JsonRpcClient {
    val loadBalancingClient = createWithLoadBalancing(
      endpoints = endpoints,
      maxInflightRequestsPerClient = maxInflightRequestsPerClient,
      httpVersion = httpVersion,
      requestObjectMapper = requestObjectMapper,
      responseObjectMapper = responseObjectMapper,
      requestTimeout = requestTimeout,
      log = log,
      requestResponseLogLevel = requestResponseLogLevel,
      failuresLogLevel = failuresLogLevel,
    )

    return JsonRpcRequestRetryer(
      vertx,
      loadBalancingClient,
      config = JsonRpcRequestRetryer.Config(
        methodsToRetry = methodsToRetry,
        requestRetry = retryConfig,
      ),
      requestObjectMapper = requestObjectMapper,
      failuresLogLevel = failuresLogLevel,
      log = log,
    )
  }

  override fun createJsonRpcV2Client(
    endpoints: List<URI>,
    maxInflightRequestsPerClient: UInt?,
    retryConfig: RequestRetryConfig,
    httpVersion: HttpVersion?,
    requestObjectMapper: ObjectMapper,
    responseObjectMapper: ObjectMapper,
    requestTimeout: Long?,
    shallRetryRequestsClientBasePredicate: Predicate<Result<Any?, Throwable>>,
    log: Logger,
    requestResponseLogLevel: Level,
    failuresLogLevel: Level,
  ): JsonRpcV2Client {
    assert(endpoints.isNotEmpty()) { "endpoints set is empty " }
    assert(endpoints.size == endpoints.toSet().size) {
      "endpoints set contains duplicates: $endpoints"
    }

    // create base client
    return if (maxInflightRequestsPerClient != null || endpoints.size > 1) {
      createWithLoadBalancing(
        endpoints = endpoints.map { it.toURL() }.toSet(),
        maxInflightRequestsPerClient = maxInflightRequestsPerClient!!,
        httpVersion = httpVersion,
        requestObjectMapper = requestObjectMapper,
        responseObjectMapper = responseObjectMapper,
        requestTimeout = requestTimeout,
        log = log,
        requestResponseLogLevel = requestResponseLogLevel,
        failuresLogLevel = failuresLogLevel,
      )
    } else {
      create(
        endpoint = endpoints.first().toURL(),
        httpVersion = httpVersion,
        requestObjectMapper = requestObjectMapper,
        responseObjectMapper = responseObjectMapper,
        requestTimeout = requestTimeout,
        log = log,
        requestResponseLogLevel = requestResponseLogLevel,
        failuresLogLevel = failuresLogLevel,
      )
    }.let {
      // Wrap the client with a retryer
      JsonRpcRequestRetryerV2(
        vertx = vertx,
        delegate = it,
        requestRetry = retryConfig,
        requestObjectMapper = requestObjectMapper,
        shallRetryRequestsClientBasePredicate = shallRetryRequestsClientBasePredicate,
        failuresLogLevel = failuresLogLevel,
        log = log,
      )
    }.let {
      // Wrap the client with a v2 client helper
      JsonRpcV2ClientImpl(
        delegate = it,
        idSupplier = requestIdSupplier,
      )
    }
  }
}

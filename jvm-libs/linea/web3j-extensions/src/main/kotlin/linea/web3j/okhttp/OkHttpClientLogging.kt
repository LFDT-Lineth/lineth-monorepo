package linea.web3j.okhttp

import net.consensys.linea.logging.JsonRpcRequestResponseLogger
import net.consensys.linea.logging.MinimalInLineJsonRpcLogger
import net.consensys.linea.logging.maskEndpointPath
import okhttp3.Interceptor
import okhttp3.OkHttpClient
import okhttp3.Response
import okio.Buffer
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.protocol.http.HttpService

class OkHttpMinimalJsonRpcLoggerInterceptor(
  val logger: JsonRpcRequestResponseLogger = MinimalInLineJsonRpcLogger(LogManager.getLogger(HttpService::class.java))
) : Interceptor {
  override fun intercept(chain: Interceptor.Chain): Response {
    val request = chain.request()
    val endpoint = request.url.toString()
    val requestBody = request.body?.let {
      val buffer = Buffer()
      it.writeTo(buffer)
      buffer.readUtf8()
    } ?: ""

    logger.logRequest(endpoint, requestBody)

    return kotlin.runCatching {
      chain.proceed(request)
    }.onSuccess { response ->
      val responseBody = response.body?.source()
        ?.apply { request(Long.MAX_VALUE) }
        ?.buffer?.clone()?.readUtf8()
        ?: ""
      logger.logResponse(
        endpoint = endpoint,
        responseStatusCode = response.code,
        requestBody = requestBody,
        responseBody = responseBody
      )
    }.onFailure { e ->
      logger.logResponse(
        endpoint = endpoint,
        responseStatusCode = null,
        requestBody = requestBody,
        responseBody = "",
        failureCause = e
      )
    }
      .getOrThrow()
  }
}

fun okHttpClientBuilder(
  logger: Logger = LogManager.getLogger(HttpService::class.java), // use same class to keep backward compatibility
  // we make a lot of eth_call request that fail by design, having DEBUG/WARN level is too noisy
  // ideally we should manage methods individually, but don't have time for that now
  requestResponseLogLevel: Level = Level.TRACE,
  failuresLogLevel: Level = Level.DEBUG
): OkHttpClient.Builder {
  val httpClientBuilder = OkHttpClient.Builder()
  httpClientBuilder.addInterceptor(
    OkHttpMinimalJsonRpcLoggerInterceptor(
      MinimalInLineJsonRpcLogger(
        logger,
        requestResponseLogLevel = requestResponseLogLevel,
        failuresLogLevel = failuresLogLevel,
        maskEndpoint = ::maskEndpointPath
      )
    )
  )
  return httpClientBuilder
}

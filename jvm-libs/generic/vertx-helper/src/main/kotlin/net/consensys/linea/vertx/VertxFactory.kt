package net.consensys.linea.vertx

import io.vertx.core.Vertx
import io.vertx.core.VertxOptions
import io.vertx.core.json.JsonObject
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

object VertxFactory {
  fun createVertxWithJsonConfigs(configs: JsonObject): Vertx {
    return Vertx.vertx(VertxOptions(configs))
  }

  fun createVertx(
    maxEventLoopExecuteTime: Duration? = 5.seconds,
    maxWorkerExecuteTime: Duration? = 30.seconds,
    blockedThreadCheckInterval: Duration? = 5.seconds,
    warningExceptionTime: Duration? = 60.seconds,
    jvmMetricsEnabled: Boolean = true,
    prometheusMetricsEnabled: Boolean = true,
    preferNativeTransport: Boolean = true
  ): Vertx {
    val configs = JsonObject()
    maxEventLoopExecuteTime?.let {
      configs.put("maxEventLoopExecuteTime", it.inWholeMilliseconds)
      configs.put("maxEventLoopExecuteTimeUnit", "MILLISECONDS")
    }
    maxWorkerExecuteTime?.let {
      configs.put("maxWorkerExecuteTime", it.inWholeMilliseconds)
      configs.put("maxWorkerExecuteTimeTimeUnit", "MILLISECONDS")
    }
    blockedThreadCheckInterval?.let {
      configs.put("blockedThreadCheckInterval", it.inWholeMilliseconds)
      configs.put("blockedThreadCheckIntervalUnit", "MILLISECONDS")
    }
    warningExceptionTime?.let {
      configs.put("warningExceptionTime", it.inWholeMilliseconds)
      configs.put("warningExceptionTimeUnit", "MILLISECONDS")
    }
    configs.put("preferNativeTransport", preferNativeTransport)

    if (jvmMetricsEnabled || prometheusMetricsEnabled) {
      val metricsOptions = JsonObject()
      metricsOptions.put("enabled", true)
      metricsOptions.put("jvmMetricsEnabled", jvmMetricsEnabled)
      if (prometheusMetricsEnabled) {
        val prometheusOptions = JsonObject()
        prometheusOptions.put("enabled", true)
        prometheusOptions.put("publishQuantiles", true)
        metricsOptions.put("prometheusOptions", prometheusOptions)
      }
      configs.put("metricsOptions", metricsOptions)
    }
    return createVertxWithJsonConfigs(configs)
  }
}

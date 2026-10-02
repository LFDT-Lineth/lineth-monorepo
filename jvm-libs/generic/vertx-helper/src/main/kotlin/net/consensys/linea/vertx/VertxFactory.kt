package net.consensys.linea.vertx

import io.vertx.core.Vertx
import io.vertx.core.VertxOptions
import io.vertx.micrometer.MicrometerMetricsOptions
import io.vertx.micrometer.VertxPrometheusOptions
import java.util.concurrent.TimeUnit
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

object VertxFactory {
  fun createVertx(
    maxEventLoopExecuteTime: Duration = 5.seconds,
    maxWorkerExecuteTime: Duration = 30.seconds,
    blockedThreadCheckInterval: Duration = 5.seconds,
    warningExceptionTime: Duration = 60.seconds,
    jvmMetricsEnabled: Boolean = true,
    prometheusMetricsEnabled: Boolean = true,
    preferNativeTransport: Boolean = true,
    eventLoopPoolSize: Int = VertxOptions.DEFAULT_EVENT_LOOP_POOL_SIZE, // 2*#CPU default
    workerThreadPoolSize: Int = VertxOptions.DEFAULT_WORKER_POOL_SIZE, // 20 default
    internalBlockingPoolSize: Int = VertxOptions.DEFAULT_INTERNAL_BLOCKING_POOL_SIZE, // 20 Default

  ): Vertx {
    val options = VertxOptions()
      .setEventLoopPoolSize(eventLoopPoolSize)
      .setWorkerPoolSize(workerThreadPoolSize)
      .setInternalBlockingPoolSize(internalBlockingPoolSize)
      .setPreferNativeTransport(preferNativeTransport)
      .setMaxEventLoopExecuteTime(maxEventLoopExecuteTime.inWholeMilliseconds)
      .setMaxEventLoopExecuteTimeUnit(TimeUnit.MILLISECONDS)
      .setMaxWorkerExecuteTime(maxWorkerExecuteTime.inWholeMilliseconds)
      .setMaxWorkerExecuteTimeUnit(TimeUnit.MILLISECONDS)
      .setBlockedThreadCheckInterval(blockedThreadCheckInterval.inWholeMilliseconds)
      .setBlockedThreadCheckIntervalUnit(TimeUnit.MILLISECONDS)
      .setWarningExceptionTime(warningExceptionTime.inWholeMilliseconds)
      .setWarningExceptionTimeUnit(TimeUnit.MILLISECONDS)

    if (jvmMetricsEnabled || prometheusMetricsEnabled) {
      val metricsOptions = MicrometerMetricsOptions()
        .setEnabled(true)
        .setJvmMetricsEnabled(jvmMetricsEnabled)
      if (prometheusMetricsEnabled) {
        val prometheusOptions = VertxPrometheusOptions()
          .setEnabled(true)
          .setPublishQuantiles(true)
        metricsOptions.setPrometheusOptions(prometheusOptions)
      }
      options.setMetricsOptions(metricsOptions)
    }
    return Vertx.vertx(options)
  }
}

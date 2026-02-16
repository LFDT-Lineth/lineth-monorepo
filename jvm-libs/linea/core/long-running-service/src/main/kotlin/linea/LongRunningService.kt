package linea

import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import java.util.concurrent.CompletableFuture

interface LongRunningService {
  fun start(): CompletableFuture<Unit>
  fun stop(): CompletableFuture<Unit>

  companion object {
    /**
     * Creates an LongRunningService that will start all the provided services in order
     * and stops then in reverse order.
     */
    fun <T : LongRunningService> compose(services: List<T>): LongRunningService = ServiceAggregator(services)

    fun <T : LongRunningService> compose(vararg services: T): LongRunningService = ServiceAggregator(services.toList())

    fun disabled(serviceName: String): LongRunningService = DisabledService(serviceName)
  }
}

internal class ServiceAggregator<T : LongRunningService>(private val services: List<T>) : LongRunningService {
  override fun start(): CompletableFuture<Unit> {
    return CompletableFuture.allOf(*services.map { it.start() }.toTypedArray()).thenApply { Unit }
  }

  override fun stop(): CompletableFuture<Unit> {
    return CompletableFuture.allOf(*services.reversed().map { it.stop() }.toTypedArray()).thenApply { Unit }
  }
}

/**
 * Placeholder for disabled services
 * handy to avoid nullables and if/else
 */
open class DisabledService(
  val serviceName: String,
  val log: Logger = LogManager.getLogger(DisabledService::class.java),
) : LongRunningService {
  override fun start(): CompletableFuture<Unit> {
    log.info("skipping disabled service {} start", serviceName)
    return CompletableFuture.completedFuture(Unit)
  }
  override fun stop(): CompletableFuture<Unit> {
    log.debug("service {} is disabled", serviceName)
    return CompletableFuture.completedFuture(Unit)
  }
}

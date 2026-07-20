package linea.coordinator.extensions

import io.vertx.core.Vertx
import io.vertx.sqlclient.SqlClient
import linea.LongRunningService
import net.consensys.linea.jsonrpc.JsonRpcRequestHandler
import net.consensys.linea.metrics.MetricsFacade

/**
 * Read-only handle to the shared infrastructure the [CoordinatorApp] has already built.
 *
 * Passed to [CoordinatorExtensionFactory] so extensions can wire their own services and
 * request handlers against the same Vertx instance, metrics registry and database connection
 * the core app uses, instead of standing up duplicates.
 *
 * Keep this surface intentionally small: every member added here becomes a contract the core
 * app must keep honouring for downstream extensions. Add accessors only when an extension
 * genuinely needs them.
 */
interface CoordinatorContext {
  val vertx: Vertx
  val metricsFacade: MetricsFacade
  val sqlClient: SqlClient
}

/**
 * Optional add-on to the coordinator: extra long-running services and/or JSON-RPC methods that
 * run inside the same process and lifecycle as the core app.
 *
 * The core app owns the lifecycle: it starts [services] alongside its own and stops them on
 * shutdown, and merges [jsonRpcHandlers] into the JSON-RPC router. Implementations must not
 * start/stop their own services or open their own server.
 *
 * This is the single seam the enterprise distribution layers on top of the OSS coordinator.
 * Both members default to empty so an extension can contribute services only, handlers only,
 * or both.
 */
interface CoordinatorExtension {
  /** Services to be started with, and stopped alongside, the core app. */
  fun services(): List<LongRunningService> = emptyList()

  /** Additional JSON-RPC methods, keyed by method name, merged into the core router. */
  fun jsonRpcHandlers(): Map<String, JsonRpcRequestHandler> = emptyMap()
}

/**
 * Builds the extensions for a run, given the shared [CoordinatorContext].
 *
 * Invoked once by [CoordinatorApp] after its internal components are constructed but before
 * anything is started. Defaults to contributing nothing, so the OSS app behaves identically
 * when no extension is supplied.
 */
fun interface CoordinatorExtensionFactory {
  fun create(context: CoordinatorContext): List<CoordinatorExtension>

  companion object {
    val NOOP = CoordinatorExtensionFactory { emptyList() }
  }
}

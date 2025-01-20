package net.consensys.linea.traces.app

import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import io.micrometer.core.instrument.MeterRegistry
import io.vertx.core.Future
import io.vertx.core.Vertx
import io.vertx.core.json.jackson.VertxModule
import io.vertx.micrometer.backends.BackendRegistries
import net.consensys.linea.TracesConflationServiceV1Impl
import net.consensys.linea.TracesCountingServiceWithRetry
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import net.consensys.linea.traces.RawJsonTracesConflator
import net.consensys.linea.traces.RawJsonTracesCounter
import net.consensys.linea.traces.RawJsonTracesCounterV0
import net.consensys.linea.traces.app.api.Api
import net.consensys.linea.traces.app.api.ApiConfig
import net.consensys.linea.traces.app.api.TracesSemanticVersionValidator
import net.consensys.linea.traces.repository.FilesystemConflatedTracesRepository
import net.consensys.linea.traces.repository.FilesystemTracesRepositoryV1
import net.consensys.linea.vertx.loadVertxConfig
import org.apache.logging.log4j.LogManager
import java.nio.file.Files
import java.nio.file.Path

data class AppConfig(
  val inputTracesDirectory: String,
  val outputTracesDirectory: String,
  val tracesApiVersion: String,
  val api: ApiConfig,
  val tracesFileExtension: String,
  // This is meant fo be false for local Debug only. Not in prod
  // Override in CLI with --Dconfig.override.conflated_trace_compression=false
  val conflatedTracesCompression: Boolean = true
)

class TracesApiFacadeApp(config: AppConfig) {
  private val log = LogManager.getLogger(TracesApiFacadeApp::class.java)
  private val meterRegistry: MeterRegistry
  private val vertx: Vertx
  private var api: Api

  init {
    log.debug("System properties: {}", System.getProperties())
    val vertxConfig = loadVertxConfig()
    log.debug("Vertx full configs: {}", vertxConfig)
    log.info("App configs: {}", config)
    validateConfig(config)
    this.vertx = Vertx.vertx(vertxConfig)
    this.meterRegistry = BackendRegistries.getDefaultNow()
    val tracesRepository =
      FilesystemTracesRepositoryV1(
        FilesystemTracesRepositoryV1.Config(
          Path.of(config.inputTracesDirectory),
          config.tracesFileExtension
        )
      )
    val jsonSerializerObjectMapper = jacksonObjectMapper().apply {
      registerModule(VertxModule())
    }
    val conflatedTracesRepository =
      FilesystemConflatedTracesRepository(
        vertx,
        Path.of(config.outputTracesDirectory),
        gzipCompressionEnabled = config.conflatedTracesCompression,
        jsonSerializerObjectMapper
      )
    val tracesCounterService =
      TracesCountingServiceWithRetry(
        repository = tracesRepository,
        tracesCounter = RawJsonTracesCounter(config.tracesApiVersion),
        tracesCounterV0 = RawJsonTracesCounterV0(config.tracesApiVersion)
      )
    val tracesConflationService =
      TracesConflationServiceV1Impl(
        tracesRepository,
        RawJsonTracesConflator(config.tracesApiVersion),
        conflatedTracesRepository,
        config.tracesApiVersion
      )
    val semVerValidator = TracesSemanticVersionValidator(
      TracesSemanticVersionValidator.SemanticVersion.fromString(config.tracesApiVersion)
    )
    this.api =
      Api(
        configs = config.api,
        vertx = vertx,
        metricsFacade = MicrometerMetricsFacade(meterRegistry),
        semVerValidator = semVerValidator,
        tracesCountingService = tracesCounterService,
        tracesConflationService = tracesConflationService
      )
  }

  fun start(): Future<*> {
    return api.start().onComplete { log.info("App successfully started") }
  }

  fun stop(): Future<*> {
    log.info("Shooting down app..")
    return api.stop().onComplete { log.info("App successfully closed") }
  }

  private fun validateConfig(config: AppConfig): Boolean {
    assertDirectory(Path.of(config.inputTracesDirectory).toAbsolutePath())
    assertDirectory(
      Path.of(config.outputTracesDirectory).toAbsolutePath(),
      createIfDoesNotExist = true
    )
    return true
  }

  private fun assertDirectory(directory: Path, createIfDoesNotExist: Boolean = false) {
    if (!Files.exists(directory)) {
      if (createIfDoesNotExist) {
        Files.createDirectories(directory)
      } else {
        throw Exception("Directory not found: $directory")
      }
    }
    if (!Files.isReadable(directory)) throw Exception("Cannot read directory: $directory")
  }
}

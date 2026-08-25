package lineth.coordinator.app.conflationbacktesting

import io.vertx.core.Vertx
import linea.timer.TimerSchedule
import linea.timer.VertxPeriodicPollingService
import lineth.coordinator.config.v2.CoordinatorConfig
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.MetricsFacade
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ConcurrentHashMap
import kotlin.time.Duration.Companion.seconds

class ConflationBacktestingService(
  private val vertx: Vertx,
  private val configs: CoordinatorConfig,
  private val metricsFacade: MetricsFacade,
  private val httpJsonRpcClientFactory: VertxHttpJsonRpcClientFactory = VertxHttpJsonRpcClientFactory(
    vertx = vertx,
    metricsFacade = metricsFacade,
  ),
  private val log: Logger = LogManager.getLogger(ConflationBacktestingService::class.java),
) : VertxPeriodicPollingService(
  vertx = vertx,
  name = "ConflationBacktestingService",
  pollingIntervalMs = 1.seconds.inWholeMilliseconds,
  log = log,
  timerSchedule = TimerSchedule.FIXED_DELAY,
) {
  enum class ConflationBacktestingJobStatus {
    IN_PROGRESS,
    COMPLETED,
    FAILED,
  }

  private val jobLifecycleLock = Any()

  // jobIds that have been accepted but whose app is still being constructed. Guarded by [jobLifecycleLock].
  private val reservedJobIds: MutableSet<String> = ConcurrentHashMap.newKeySet()
  private val completedJobs: MutableSet<String> = ConcurrentHashMap.newKeySet()
  private val failedJobs: MutableSet<String> = ConcurrentHashMap.newKeySet()
  private val conflationBackTestingApps: MutableMap<String, ConflationBacktestingApp> = ConcurrentHashMap()

  fun submitConflationBacktestingJob(conflationBacktestingConfig: ConflationBacktestingConfig): String {
    val jobId = conflationBacktestingConfig.jobId()
    // Accept (or reject) the job id before doing any of the expensive app setup, so a duplicate or
    // already-completed submission is rejected without running the full construction path for work that
    // will never be scheduled.
    synchronized(jobLifecycleLock) {
      if (completedJobs.contains(jobId)) {
        throw IllegalArgumentException("Given Conflation backtesting Request with jobId=$jobId is already completed")
      }
      if (conflationBackTestingApps.containsKey(jobId) || !reservedJobIds.add(jobId)) {
        throw IllegalArgumentException(
          "Given Conflation backtesting Request with jobId=$jobId is already being processed",
        )
      }
      // A previously failed job may be resubmitted; clear the terminal state now that it is accepted again.
      failedJobs.remove(jobId)
    }

    val app = try {
      ConflationBacktestingApp(
        vertx = vertx,
        conflationBacktestingAppConfig = conflationBacktestingConfig,
        mainCoordinatorConfig = configs,
        httpJsonRpcClientFactory = httpJsonRpcClientFactory,
        metricsFacade = metricsFacade,
      )
    } catch (error: Throwable) {
      // Construction failed: release the reservation so the id can be retried.
      synchronized(jobLifecycleLock) {
        reservedJobIds.remove(jobId)
      }
      throw error
    }

    synchronized(jobLifecycleLock) {
      conflationBackTestingApps[jobId] = app
      reservedJobIds.remove(jobId)
    }

    app.start().thenPeek {
      log.info("Conflation backtesting job started: jobId={}", jobId)
    }.exceptionally { error ->
      log.error("Conflation backtesting job failed: jobId={}, errorMessage={}", jobId, error.message, error)
      handleFailedJobStart(jobId, app)
    }
    return jobId
  }

  /**
   * Handles a failure of [ConflationBacktestingApp.start] by moving the job to a terminal [FAILED] state
   * and stopping the partially started app, so status checks no longer report it as [IN_PROGRESS] and its
   * resources are released. Does nothing if the app is no longer the active one for [jobId] (e.g. it was
   * concurrently stopped), which already removed it and ran [ConflationBacktestingApp.stop].
   */
  private fun handleFailedJobStart(jobId: String, app: ConflationBacktestingApp) {
    val shouldStop = synchronized(jobLifecycleLock) {
      if (conflationBackTestingApps.remove(jobId, app)) {
        failedJobs.add(jobId)
        true
      } else {
        false
      }
    }
    if (shouldStop) {
      app.stop().whenException { error ->
        log.error(
          "Error while stopping failed conflation backtesting job: jobId={}, errorMessage={}",
          jobId,
          error.message,
          error,
        )
      }
    }
  }

  fun getConflationBacktestingJobStatus(jobId: String): ConflationBacktestingJobStatus {
    synchronized(jobLifecycleLock) {
      if (completedJobs.contains(jobId)) {
        return ConflationBacktestingJobStatus.COMPLETED
      }
      if (failedJobs.contains(jobId)) {
        return ConflationBacktestingJobStatus.FAILED
      }
      if (conflationBackTestingApps.containsKey(jobId) || reservedJobIds.contains(jobId)) {
        return ConflationBacktestingJobStatus.IN_PROGRESS
      }
    }
    throw IllegalArgumentException("No conflation backtesting job found with id: $jobId")
  }

  /**
   * Stops an in-progress conflation backtesting job and releases its resources.
   *
   * Map removal and marking the job completed happen under [jobLifecycleLock] without calling
   * [ConflationBacktestingApp.stop], so [action] cannot observe a completed job and schedule a second
   * stop for the same app. Only one path removes the job from [conflationBackTestingApps] and runs
   * [ConflationBacktestingApp.stop].
   *
   * @return a future that completes when the underlying app has fully stopped
   * @throws IllegalArgumentException if no in-progress job with the given id exists
   * (e.g. unknown id, or the job has already completed).
   */
  fun stopConflationBacktestingJob(jobId: String): SafeFuture<Unit> {
    val app = synchronized(jobLifecycleLock) {
      if (completedJobs.contains(jobId)) {
        throw IllegalArgumentException("Conflation backtesting job with jobId=$jobId is already completed")
      }
      val removed = conflationBackTestingApps.remove(jobId)
        ?: throw IllegalArgumentException("No in-progress conflation backtesting job found with jobId=$jobId")
      completedJobs.add(jobId)
      removed
    }
    log.info("Stopping conflation backtesting job: jobId={}", jobId)
    return app.stop().whenException { error ->
      log.error(
        "Error while stopping conflation backtesting job: jobId={}, errorMessage={}",
        jobId,
        error.message,
        error,
      )
    }
  }

  override fun action(): SafeFuture<*> {
    val appsToStop = synchronized(jobLifecycleLock) {
      val toStop = mutableListOf<ConflationBacktestingApp>()
      for ((jobId, app) in conflationBackTestingApps.toMap()) {
        if (app.isConflationBacktestingComplete() && conflationBackTestingApps.remove(jobId, app)) {
          completedJobs.add(jobId)
          toStop.add(app)
        }
      }
      toStop
    }
    return SafeFuture.allOf(*appsToStop.map { app -> app.stop() }.toTypedArray())
  }

  override fun stop(): SafeFuture<Unit> {
    return super.stop().thenCompose {
      val appsToShutdown = synchronized(jobLifecycleLock) {
        val apps = conflationBackTestingApps.values.toList()
        conflationBackTestingApps.clear()
        reservedJobIds.clear()
        completedJobs.clear()
        failedJobs.clear()
        apps
      }
      SafeFuture.allOf(*appsToShutdown.map { app -> app.stop() }.toTypedArray())
    }.thenApply { }
  }
}

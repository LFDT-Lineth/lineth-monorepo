package net.consensys.zkevm.fileio

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import io.vertx.core.Vertx
import net.consensys.linea.async.AsyncRetryer
import net.consensys.linea.async.RetriedExecutionException
import net.consensys.linea.async.toSafeFuture
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.nio.file.Path
import kotlin.time.Duration

class FileMonitor(
  private val vertx: Vertx,
  config: Config,
) {
  private val asyncRetryer =
    AsyncRetryer.retryer<List<Boolean>>(
      vertx = vertx,
      backoffDelay = config.pollingInterval,
      timeout = config.timeout,
    )

  data class Config(
    val pollingInterval: Duration,
    val timeout: Duration,
  )

  enum class ErrorType {
    TIMED_OUT,
  }

  /**
   * Monitors a list of files and completes when the first file is available on the file system
   */
  fun monitorFiles(filePaths: List<Path>): SafeFuture<Result<Path, ErrorType>> {
    return asyncRetryer.retry(stopRetriesPredicate = { filePathsFound -> filePathsFound.contains(true) }) {
      val filePathsExist = filePaths.map { filePath -> fileExists(filePath) }.stream()
      SafeFuture.collectAll(filePathsExist)
    }.handle { filesFound, t ->
      val fileFoundIndex = filesFound?.indexOf(true) ?: -1
      if (fileFoundIndex >= 0) {
        Ok(filePaths[fileFoundIndex])
      } else if (t != null) {
        when (t) {
          is RetriedExecutionException -> Err(ErrorType.TIMED_OUT)
          else -> throw t
        }
      } else {
        throw IllegalStateException()
      }
    }
  }

  fun monitor(filePath: Path): SafeFuture<Result<Path, ErrorType>> {
    return monitorFiles(listOf(filePath))
  }

  fun fileExists(filePath: Path): SafeFuture<Boolean> {
    return vertx.fileSystem().exists(filePath.toString()).toSafeFuture()
  }

  fun fileExists(directory: Path, pattern: String): SafeFuture<Boolean> {
    return findFile(directory, pattern).thenApply { it != null }
  }

  fun findFile(directory: Path, pattern: String): SafeFuture<String?> {
    return vertx
      .fileSystem()
      .readDir(
        directory.toString(),
        pattern,
      )
      .map { files -> files.firstOrNull() }
      .toSafeFuture()
  }
}

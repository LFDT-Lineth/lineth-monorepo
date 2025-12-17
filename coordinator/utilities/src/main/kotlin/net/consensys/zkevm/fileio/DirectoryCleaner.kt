package net.consensys.zkevm.fileio

import io.vertx.core.Vertx
import net.consensys.linea.async.toSafeFuture
import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.io.File
import java.io.FileFilter
import java.nio.file.Path
import kotlin.io.path.isDirectory

class DirectoryCleaner(
  val vertx: Vertx,
  val directories: List<Path>,
  val fileFilters: List<FileFilter>,
) {
  private val log = LogManager.getLogger(this::class.java)

  fun cleanup(): SafeFuture<Unit> {
    val directoriesCleanup = mutableListOf<SafeFuture<*>>()
    directories.forEach { directory ->
      if (directory.isDirectory()) {
        directoriesCleanup.add(cleanDirectory(directory))
      } else {
        log.warn("$directory is not a directory in the filesystem")
      }
    }
    return SafeFuture.allOf(directoriesCleanup.stream())
      .thenApply { log.info("File cleanup complete in $directories") }
  }

  internal fun cleanDirectory(path: Path): SafeFuture<*> {
    return vertx.fileSystem().readDir(path.toString()).toSafeFuture()
      .thenApply { filePaths ->
        val deletions = mutableListOf<SafeFuture<*>>()
        filePaths.forEach { filePath ->
          val file = File(filePath)
          if (file.isFile && fileFilters.any { it.accept(file) }) {
            val deletion =
              vertx.fileSystem()
                .delete(filePath)
                .toSafeFuture()
                .whenComplete { _, deleteException ->
                  deleteException?.also { log.warn("Failed to delete $filePath", it) }
                }
                .thenApply { }
            deletions.add(deletion)
          }
        }
        SafeFuture.allOf(deletions.stream())
      }
  }

  companion object {
    val JSON_FILE_FILTER =
      FileFilter { fileName: File ->
        fileName.extension.compareTo(other = "json", ignoreCase = true) == 0
      }

    fun getSuffixFileFilters(suffixes: List<String>): List<FileFilter> {
      return suffixes.map { suffix ->
        FileFilter { fileName: File ->
          fileName.name.endsWith(suffix)
        }
      }
    }
  }
}

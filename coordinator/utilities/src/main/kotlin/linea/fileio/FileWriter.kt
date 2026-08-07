package linea.fileio

import com.fasterxml.jackson.databind.ObjectMapper
import io.vertx.core.Vertx
import lineth.async.toSafeFutureNonNull
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.nio.file.Path
import java.util.concurrent.Callable
import kotlin.io.path.exists

fun inProgressFilePattern(fileName: String, inProgressSuffix: String): String {
  return "$fileName($inProgressSuffix)?"
}

class FileWriter(
  private val vertx: Vertx,
  private val mapper: ObjectMapper,
) {
  fun write(data: Any, filePath: Path, inProgressSuffix: String): SafeFuture<Path> {
    return vertx
      .executeBlocking(
        Callable {
          val tmpFile = inProgressFilePath(filePath, inProgressSuffix).toFile()
          mapper.writeValue(tmpFile, data)
          tmpFile.renameTo(filePath.toFile())
          filePath
        },
        false,
      ).toSafeFutureNonNull()
  }

  fun writingDoneOrInProgress(filePath: Path, inProgressSuffix: String): SafeFuture<Boolean> {
    return vertx.executeBlocking(
      Callable {
        filePath.exists() || inProgressFilePath(filePath, inProgressSuffix).exists()
      },
      false,
    ).toSafeFutureNonNull()
  }

  private fun inProgressFilePath(filePath: Path, inProgressSuffix: String): Path {
    return Path.of(filePath.toAbsolutePath().toString() + ".$inProgressSuffix")
  }
}

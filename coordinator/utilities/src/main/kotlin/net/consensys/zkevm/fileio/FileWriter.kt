package net.consensys.zkevm.fileio

import com.fasterxml.jackson.databind.ObjectMapper
import io.vertx.core.Vertx
import net.consensys.linea.async.toSafeFuture
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.io.File
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
  fun write(data: Any, filePath: Path, inProgressSuffix: String?): SafeFuture<Path> {
    return vertx
      .executeBlocking(
        Callable {
          val tmpFile = if (inProgressSuffix != null) {
            inProgressFilePath(filePath, inProgressSuffix).toFile()
          } else {
            File.createTempFile(filePath.fileName.toString(), null)
          }
          mapper.writeValue(tmpFile, data)
          tmpFile.renameTo(filePath.toFile())
          filePath
        },
        false,
      ).toSafeFuture()
  }

  fun writingDoneOrInProgress(filePath: Path, inProgressSuffix: String): SafeFuture<Boolean> {
    return vertx.executeBlocking(
      Callable {
        filePath.exists() || inProgressFilePath(filePath, inProgressSuffix).exists()
      },
      false,
    ).toSafeFuture()
  }
  private fun inProgressFilePath(filePath: Path, inProgressSuffix: String): Path {
    return Path.of(filePath.toAbsolutePath().toString() + ".$inProgressSuffix")
  }
}

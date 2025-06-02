package net.consensys.zkevm.fileio

import com.fasterxml.jackson.core.exc.StreamReadException
import com.fasterxml.jackson.databind.DatabindException
import com.fasterxml.jackson.databind.ObjectMapper
import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import io.vertx.core.Vertx
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.errors.ErrorResponse
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.nio.file.Path
import java.util.concurrent.Callable

class FileReader<T> (
  private val vertx: Vertx,
  private val mapper: ObjectMapper,
  private val classOfT: Class<T>,
) {

  enum class ErrorType {
    PARSING_ERROR,
  }

  fun read(filePath: Path): SafeFuture<Result<T, ErrorResponse<ErrorType>>> {
    return vertx
      .executeBlocking(
        Callable {
          try {
            val value = mapper.readValue(filePath.toFile(), classOfT)
            Ok(value)
          } catch (e: Exception) {
            when (e) {
              is StreamReadException, is DatabindException ->
                Err(ErrorResponse(ErrorType.PARSING_ERROR, e.message.orEmpty()))

              else -> throw e
            }
          }
        },
        false,
      )
      .toSafeFuture()
  }
}

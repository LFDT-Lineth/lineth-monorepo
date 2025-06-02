package net.consensys.zkevm.fileio

import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import net.consensys.linea.async.get
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Assertions
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import java.io.File

@ExtendWith(VertxExtension::class)
class FileReaderTest {
  private val mapper = jacksonObjectMapper()

  @AfterEach
  fun tearDown(vertx: Vertx) {
    val vertxStopFuture = vertx.close()
    vertxStopFuture.get()
  }

  @Test
  fun test_read_success(vertx: Vertx) {
    val fileReader = FileReader(vertx, mapper, String::class.java)
    val tempFile = File.createTempFile("file-reader-test", null)
    val data = "test-data"
    mapper.writeValue(tempFile, data)
    Assertions.assertEquals(
      data,
      fileReader.read(tempFile.toPath()).get().component1(),
    )
  }

  @Test
  fun test_read_parsingError(vertx: Vertx) {
    val fileReader = FileReader(vertx, mapper, Int::class.java)
    val tempFile = File.createTempFile("file-reader-test", null)
    val data = "test-data"
    mapper.writeValue(tempFile, data)
    Assertions.assertEquals(
      FileReader.ErrorType.PARSING_ERROR,
      fileReader.read(tempFile.toPath()).get().component2()?.type,
    )
  }
}

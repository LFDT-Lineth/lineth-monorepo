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
class FileWriterTest {
  private val mapper = jacksonObjectMapper()

  @AfterEach
  fun tearDown(vertx: Vertx) {
    val vertxStopFuture = vertx.close()
    vertxStopFuture.get()
  }

  @Test
  fun write_withInProgressSuffix(vertx: Vertx) {
    val testFilePath = File.createTempFile("file-writer-test", null).toPath()
    val data = "test-data"
    val fileWriter = FileWriter(vertx, mapper)
    val result = fileWriter.write(data, testFilePath, "inp").get()
    val text = mapper.readValue(result.toFile(), String::class.java)
    Assertions.assertEquals(
      data,
      text,
    )
  }

  @Test
  fun write_withoutInProgressSuffix(vertx: Vertx) {
    val testFilePath = File.createTempFile("file-writer-test", null).toPath()
    val data = "test-data"
    val fileWriter = FileWriter(vertx, mapper)
    val result = fileWriter.write(data, testFilePath, null).get()
    val text = mapper.readValue(result.toFile(), String::class.java)
    Assertions.assertEquals(
      data,
      text,
    )
  }
}

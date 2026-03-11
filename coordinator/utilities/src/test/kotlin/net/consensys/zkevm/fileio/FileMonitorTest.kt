package net.consensys.zkevm.fileio

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import net.consensys.linea.async.get
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Assertions
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import java.nio.file.Files
import java.nio.file.Path
import kotlin.time.DurationUnit
import kotlin.time.toDuration

@ExtendWith(VertxExtension::class)
class FileMonitorTest {
  lateinit var tmpDirectory: Path

  @BeforeEach
  fun setUp() {
    tmpDirectory = Files.createTempDirectory("file-monitor-test")
  }

  @AfterEach
  fun tearDown(vertx: Vertx) {
    vertx.fileSystem().deleteRecursiveBlocking(tmpDirectory.toString())
    val vertxStopFuture = vertx.close()
    vertxStopFuture.get()
  }

  @Test
  fun test_fileExists_exists(vertx: Vertx) {
    val config =
      FileMonitor.Config(
        pollingInterval = 50.toDuration(DurationUnit.MILLISECONDS),
        timeout = 1.toDuration(DurationUnit.SECONDS),
      )
    val fileMonitor = FileMonitor(vertx, config)

    val testFile = tmpDirectory.resolve("test_fileExists_exists").toFile()
    testFile.createNewFile()
    Assertions.assertTrue(fileMonitor.fileExists(testFile.toPath()).get())
  }

  @Test
  fun test_fileExists_doesNotExist(vertx: Vertx) {
    val config =
      FileMonitor.Config(
        pollingInterval = 50.toDuration(DurationUnit.MILLISECONDS),
        timeout = 1.toDuration(DurationUnit.SECONDS),
      )
    val fileMonitor = FileMonitor(vertx, config)
    val testFilePath = tmpDirectory.resolve("test_fileExists_doesNotExist")
    Assertions.assertFalse(fileMonitor.fileExists(testFilePath).get())
  }

  @Test
  fun test_fileExists_patternExists(vertx: Vertx) {
    val config =
      FileMonitor.Config(
        pollingInterval = 50.toDuration(DurationUnit.MILLISECONDS),
        timeout = 1.toDuration(DurationUnit.SECONDS),
      )
    val fileMonitor = FileMonitor(vertx, config)
    val testFile = tmpDirectory.resolve("file-monitor-test-1").toFile()
    testFile.createNewFile()
    Assertions.assertTrue(fileMonitor.fileExists(tmpDirectory, "file-monitor-test-.*").get())
  }

  @Test
  fun test_fileExists_patternDoesNotExists(vertx: Vertx) {
    val config =
      FileMonitor.Config(
        pollingInterval = 50.toDuration(DurationUnit.MILLISECONDS),
        timeout = 1.toDuration(DurationUnit.SECONDS),
      )
    val fileMonitor = FileMonitor(vertx, config)
    Assertions.assertFalse(fileMonitor.fileExists(tmpDirectory, "file-monitor-test-.*").get())
  }

  @Test
  fun test_monitor_fileAlreadyExists(vertx: Vertx) {
    val testFile = tmpDirectory.resolve("test_monitor_fileAlreadyExists").toFile()
    testFile.createNewFile()
    val config =
      FileMonitor.Config(
        pollingInterval = 50.toDuration(DurationUnit.MILLISECONDS),
        timeout = 1.toDuration(DurationUnit.SECONDS),
      )
    val fileMonitor = FileMonitor(vertx, config)
    val result = fileMonitor.monitor(testFile.toPath()).get()
    Assertions.assertTrue(result is Ok)
    Assertions.assertEquals(
      testFile.toPath(),
      result.component1(),
    )
  }

  @Test
  fun test_monitor_timeOut(vertx: Vertx) {
    val config =
      FileMonitor.Config(
        pollingInterval = 50.toDuration(DurationUnit.MILLISECONDS),
        timeout = 1.toDuration(DurationUnit.SECONDS),
      )
    val fileMonitor = FileMonitor(vertx, config)
    val testFilePath = tmpDirectory.resolve("test_monitor_timeOut")
    val result = fileMonitor.monitor(testFilePath).get()
    Assertions.assertTrue(result is Err)
    Assertions.assertEquals(
      FileMonitor.ErrorType.TIMED_OUT,
      result.component2(),
    )
  }

  @Test
  fun test_monitor_fileCreatedInTime(vertx: Vertx) {
    val tempFilePath = tmpDirectory.resolve("test_monitor_fileCreatedInTime")
    val config =
      FileMonitor.Config(
        pollingInterval = 50.toDuration(DurationUnit.MILLISECONDS),
        timeout = 1.toDuration(DurationUnit.SECONDS),
      )
    val fileMonitor = FileMonitor(vertx, config)
    Assertions.assertFalse(fileMonitor.fileExists(tempFilePath).get())
    vertx.setTimer(500) {
      tempFilePath.toFile().createNewFile()
    }
    val result = fileMonitor.monitor(tempFilePath).get()
    Assertions.assertTrue(result is Ok)
    Assertions.assertEquals(
      tempFilePath,
      result.component1(),
    )
  }

  @Test
  fun test_monitor_multipleFiles_fileCreatedInTime(vertx: Vertx) {
    val tempFilePath1 = tmpDirectory.resolve("test_monitor_fileCreatedInTime1")
    val tempFilePath2 = tmpDirectory.resolve("test_monitor_fileCreatedInTime2")
    val config =
      FileMonitor.Config(
        pollingInterval = 50.toDuration(DurationUnit.MILLISECONDS),
        timeout = 1.toDuration(DurationUnit.SECONDS),
      )
    val fileMonitor = FileMonitor(vertx, config)
    Assertions.assertFalse(fileMonitor.fileExists(tempFilePath1).get())
    Assertions.assertFalse(fileMonitor.fileExists(tempFilePath2).get())

    vertx.setTimer(500) {
      tempFilePath2.toFile().createNewFile()
    }
    val result = fileMonitor.awaitForAnyOfFiles(listOf(tempFilePath1, tempFilePath2)).get()
    Assertions.assertTrue(result is Ok)
    Assertions.assertEquals(
      tempFilePath2,
      result.component1(),
    )
  }
}

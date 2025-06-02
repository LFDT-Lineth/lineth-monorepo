package net.consensys.zkevm.fileio

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import net.consensys.linea.async.get
import net.consensys.linea.async.toSafeFuture
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.kotlin.any
import org.mockito.kotlin.mock
import org.mockito.kotlin.whenever
import java.io.File
import java.io.FileFilter
import java.nio.file.Files
import kotlin.io.path.absolutePathString
import kotlin.io.path.name
import kotlin.io.path.nameWithoutExtension

@ExtendWith(VertxExtension::class)
class DirectoryCleanerTest {

  @Test
  fun test_tmp_directory_cleanup(vertx: Vertx) {
    val tmpDirectory = Files.createTempDirectory(
      DirectoryCleanerTest::class.simpleName + "-test_tmp_directory_cleanup",
    )
    for (i in 1..9) {
      val fileExtension = (i % 3).run {
        when (this) {
          0 -> ".json"
          1 -> ".json.inprogress"
          else -> ".json.inprogress_coordinator_writing"
        }
      }
      Files.createTempFile(
        tmpDirectory,
        "directory_cleaner_test-request-$i",
        fileExtension,
      )
    }
    assertThat(vertx.fileSystem().readDir(tmpDirectory.absolutePathString()).get().size).isEqualTo(9)
    val directoryCleaner = DirectoryCleaner(
      vertx,
      listOf(tmpDirectory),
      DirectoryCleaner.getSuffixFileFilters(listOf(".inprogress_coordinator_writing")) +
        DirectoryCleaner.JSON_FILE_FILTER,
    )
    directoryCleaner.cleanup().get()
    var remainingFiles = 0
    vertx.fileSystem().readDir(tmpDirectory.absolutePathString()).toSafeFuture().thenApply { filePaths ->
      filePaths.forEach { filePath ->
        val file = File(filePath)
        assertThat(file.extension.contains("inprogress")).isTrue()
        remainingFiles += 1
      }
    }.get()
    assertThat(remainingFiles).isEqualTo(3)
  }

  @Test
  fun test_deletion_of_absent_file_does_not_throw_exception(vertx: Vertx) {
    val tmpDirectory = Files.createTempDirectory(
      DirectoryCleanerTest::class.simpleName + "-test_deletion_of_absent_file_throws_exception",
    )
    val fileToBeMovedDuringCleanup = Files.createTempFile(
      tmpDirectory,
      "absent_file",
      ".json",
    )

    Files.createTempFile(
      tmpDirectory,
      "directory_cleaner_test-request-1",
      ".json",
    )

    val inProgressFile = Files.createTempFile(
      tmpDirectory,
      "directory_cleaner_test-request-2",
      ".json.inprogress",
    )

    val mockFileFilter = mock<FileFilter> {}
    whenever(mockFileFilter.accept(any())).thenAnswer {
      val file = it.getArgument<File>(0)
      if (file.nameWithoutExtension.contains(fileToBeMovedDuringCleanup.nameWithoutExtension)) {
        vertx.fileSystem().deleteBlocking(it.getArgument<File>(0).absolutePath)
      }
      if (file.extension.equals("json")) true else false
    }
    assertThat(vertx.fileSystem().readDir(tmpDirectory.absolutePathString()).get().size).isEqualTo(3)

    val directoryCleaner = DirectoryCleaner(vertx, listOf(tmpDirectory), listOf(mockFileFilter))
    directoryCleaner.cleanup().get()

    val remainingFiles = vertx.fileSystem().readDir(tmpDirectory.absolutePathString()).get()
    assertThat(remainingFiles.size).isEqualTo(1)
    assertThat(remainingFiles.first()).contains(inProgressFile.name)
  }

  @Test
  fun test_json_file_filter() {
    assertThat(DirectoryCleaner.JSON_FILE_FILTER.accept(File("11-27-getZkAggregatedProof.json"))).isTrue()
    assertThat(DirectoryCleaner.JSON_FILE_FILTER.accept(File("11-27-getZkAggregatedProof.Json"))).isTrue()
    assertThat(DirectoryCleaner.JSON_FILE_FILTER.accept(File("11-27-getZkAggregatedProof.JSON"))).isTrue()
    assertThat(
      DirectoryCleaner.JSON_FILE_FILTER.accept(File("11-27-getZkAggregatedProof.json.inprogress")),
    ).isFalse()
  }

  @Test
  fun test_extension_file_filter() {
    val extensionFileFilter = DirectoryCleaner.getSuffixFileFilters(listOf(".inprogress_coordinator_writing")).first()
    assertThat(
      extensionFileFilter.accept(File("11-27-getZkAggregatedProof.json.inprogress_coordinator_writing")),
    ).isTrue()
    assertThat(
      extensionFileFilter.accept(File("11-27-getZkAggregatedProof.json.inProgress_cooRdinator_Writing")),
    ).isFalse()
    assertThat(
      extensionFileFilter.accept(File("11-27-getZkAggregatedProof.json.inprogress")),
    ).isFalse()
  }
}

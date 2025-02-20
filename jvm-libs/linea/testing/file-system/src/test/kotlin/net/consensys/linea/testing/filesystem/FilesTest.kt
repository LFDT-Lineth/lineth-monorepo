package net.consensys.linea.testing.filesystem

import linea.kotlin.encodeHex
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.random.Random

class FilesTest {
  @Test
  fun `when file exists shall return path to it`() {
    val file = findPathTo("build.gradle")
    assertThat(file).isNotNull
    assertThat(file.toString()).contains("file-system/build.gradle")

    assertThat(findPathTo("settings.gradle", lookupParentDir = false)).isNull()
    assertThat(findPathTo("settings.gradle", lookupParentDir = true)).exists()
  }

  @Test
  fun `when file does not exists shall return null`() {
    val file = findPathTo("non-existing-file-${Random.nextBytes(10).encodeHex()}")
    assertThat(file).isNull()
  }
}

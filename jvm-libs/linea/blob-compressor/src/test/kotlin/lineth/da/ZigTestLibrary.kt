package lineth.da

import org.junit.jupiter.api.Assumptions.assumeTrue
import java.nio.file.Files
import java.nio.file.Path

/** The native library the tests of the Zig-backed codec run against: the file the `lineth.da.testLibrary` property names. */
internal object ZigTestLibrary {
  const val DICTIONARY = "v2025-04-21"

  val path: Path? = System.getProperty("lineth.da.testLibrary")?.takeIf { it.isNotBlank() }?.let { Path.of(it) }

  val factory: ZigDaFactory by lazy { ZigDaFactory.forLibrary(path!!) }

  val dictionary: Path by lazy { ZigDaFactory.bundledDictionary(DICTIONARY) }

  fun assumeAvailable() {
    assumeTrue(path != null, "the lineth.da.testLibrary property names no library")
    assumeTrue(Files.isRegularFile(path!!), "the lineth.da.testLibrary file $path does not exist")
  }
}

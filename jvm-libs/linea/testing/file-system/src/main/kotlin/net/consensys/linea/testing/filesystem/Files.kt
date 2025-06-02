package net.consensys.linea.testing.filesystem

import java.nio.file.Path
import java.nio.file.Paths

/**
 * Look for a file in the current directory and its parents.
 * Starts on the JVM running dir,
 * very useful for tests where this path changes dynamically when running in different
 * environments (e.g IntellIJ vs Gradle) we just want to hit play and have it work.
 */
fun findPathTo(
  targetFileOrDir: String,
  lookupDir: Path = Paths.get("").toAbsolutePath(),
  lookupParentDir: Boolean = true,
): Path? {
  var current: Path = lookupDir
  var keepSearching = true

  do {
    val targetFile = current.resolve(targetFileOrDir).toFile()
    if (targetFile.exists()) {
      return targetFile.toPath()
    }

    if (lookupParentDir && current.parent != null) {
      current = current.parent
    } else {
      keepSearching = false
    }
  } while (keepSearching)

  return null
}

fun getPathTo(
  targetFileOrDir: String,
  lookupDir: Path = Paths.get("").toAbsolutePath(),
  lookupParentDir: Boolean = true,
): Path {
  return findPathTo(targetFileOrDir, lookupDir, lookupParentDir)
    ?: throw IllegalArgumentException("Could not find $targetFileOrDir in path: $lookupDir or its parent directories")
}

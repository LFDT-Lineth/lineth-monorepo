package build.linea.jvm

import java.io.File
import java.nio.file.Files
import java.nio.file.Path

object ResourcesUtil {

  /**
   * Moves jar resource files to a temporary directory.
   * @param resourcePath The path to the resource file inside the jar
   * @param classLoader to use to find the resource. It's recommended to use SomeClass::class.java.classLoader
   *  where SomeClass is a class in the same jar as the resource file, otherwise the resource might not be found.
   * @param tmpDirPrefix The prefix to use for the temporary directory.
   */
  @JvmStatic
  fun copyResourceToTmpDir(
    resourcePath: String,
    classLoader: ClassLoader,
    tmpDirPrefix: String = "linea-resources-"
  ): Path {
    val fileDestination = File(
      Files.createTempDirectory(tmpDirPrefix)
        .resolve(Path.of(resourcePath).fileName)
        .toString()
    )
    val resourceInputStream = classLoader.getResourceAsStream(resourcePath)
      ?: throw IllegalStateException("Resource not found: $resourcePath")
    Files.copy(
      resourceInputStream,
      fileDestination.toPath()
    )
    return fileDestination.toPath()
  }
}

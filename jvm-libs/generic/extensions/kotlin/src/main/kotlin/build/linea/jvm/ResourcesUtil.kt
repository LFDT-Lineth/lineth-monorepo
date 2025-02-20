package build.linea.jvm

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
    // WARNING: this is to keep backwards compatibility with package location
    // otherwise, the we may get error at runtime: java.lang.NoClassDefFoundError: build/linea/jvm/ResourcesUtil
    return linea.jvm.ResourcesUtil.copyResourceToTmpDir(resourcePath, classLoader, tmpDirPrefix)
  }
}

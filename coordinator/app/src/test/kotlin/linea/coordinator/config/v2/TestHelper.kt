package linea.coordinator.config.v2

import java.nio.file.Path
import java.nio.file.Paths

class TestHelper {
  companion object {
    fun pathToResource(resource: String): Path {
      return Paths.get(
        this::class.java.classLoader.getResource(resource)?.toURI()
          ?: error("Resource not found: $resource"),
      )
    }
  }
}

package linea.coordinator.config.v2.docs

import kotlin.system.exitProcess

/**
 * Entry point for the `checkCoordinatorConfigDocs` Gradle task. Walks the root config classes
 * (see [COORDINATOR_CONFIG_FILES]), validates that every key is documented, prints any
 * violations, and exits non-zero on failure so the Gradle task (and CI) fails.
 */
object ConfigDocCheckMain {
  @JvmStatic
  fun main(args: Array<String>) {
    val keys = ConfigSchemaWalker.walkAll(COORDINATOR_CONFIG_FILES.map { it.rootClass })
    val violations = ConfigDocValidator.validate(keys)
    if (violations.isEmpty()) {
      println("Coordinator config docs OK: ${keys.size} keys, all documented.")
      return
    }
    System.err.println(ConfigDocValidator.formatViolations(violations))
    exitProcess(1)
  }
}

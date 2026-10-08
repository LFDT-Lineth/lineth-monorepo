package lineth.coordinator.config.v2.toml

import com.github.michaelbull.result.getOrElse
import com.github.michaelbull.result.recoverIf
import lineth.coordinator.config.v2.CoordinatorConfig
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import java.nio.file.Path
import kotlin.reflect.full.primaryConstructor

/**
 * Lets a coordinator extension own top-level tables of the main config files.
 *
 * The tables are read from the same layered files as the coordinator config, are not reported as unknown keys
 * by the coordinator pass, and are loaded and printed by `--check-configs-only` as well.
 */
interface ConfigExtension<E> {
  /** Top-level tables owned by the extension. They must not collide with a coordinator table. */
  val topLevelKeys: Set<String>

  /**
   * Loads the extension config from [configFiles], with the same layering as the coordinator config.
   * Implementations can use [loadExtensionConfigs] to follow the same strict-then-lenient policy.
   */
  fun load(configFiles: List<Path>, coordinatorConfig: CoordinatorConfig): E

  fun toPrettyLog(config: E): String

  /** Default extension, used when none is provided: owns no key and loads nothing. */
  object None : ConfigExtension<Unit> {
    override val topLevelKeys: Set<String> = emptySet()

    override fun load(configFiles: List<Path>, coordinatorConfig: CoordinatorConfig) = Unit

    override fun toPrettyLog(config: Unit): String = ""
  }
}

/**
 * Loads the [ConfigExtension.topLevelKeys] tables of the config files into [T].
 *
 * Same policy as the coordinator config: unknown keys log a WARN and are dropped, invalid values fail.
 */
inline fun <reified T : Any> loadExtensionConfigs(
  configFiles: List<Path>,
  topLevelKeys: Set<String>,
  enforceStrict: Boolean = false,
  logger: Logger = LogManager.getLogger("lineth.coordinator.config"),
): T {
  return loadConfigsAndLogErrors<T>(configFiles, logger, strict = true,
    onlyTopLevelKeys = topLevelKeys,
    addDefaultPreprocessors = false,
    addDefaultPropertySources = false,
  )
    .recoverIf({ !enforceStrict }, {
      loadConfigsAndLogErrors<T>(configFiles, logger, strict = false,
    onlyTopLevelKeys = topLevelKeys,
    addDefaultPreprocessors = false,
    addDefaultPropertySources = false,
  )
        .getOrElse { throw RuntimeException("Invalid configurations: $it") }
    })
    .getOrElse { throw RuntimeException("Invalid configurations: $it") }
}

private val coordinatorTopLevelKeys: Set<String> by lazy {
  CoordinatorConfigFilesToml::class.primaryConstructor!!.parameters.map { normalizeConfigKey(it.name!!) }.toSet()
}

/**
 * Fails when an extension claims a top-level table that already belongs to the coordinator.
 */
fun requireNoCoordinatorKeyCollision(extensionKeys: Set<String>) {
  val collisions = extensionKeys.filter { normalizeConfigKey(it) in coordinatorTopLevelKeys }
  require(collisions.isEmpty()) {
    "Config extension top-level keys collide with coordinator config sections: ${collisions.sorted()}"
  }
}

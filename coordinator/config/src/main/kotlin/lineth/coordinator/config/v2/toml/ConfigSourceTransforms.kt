package lineth.coordinator.config.v2.toml

import com.sksamuel.hoplite.ConfigFailure
import com.sksamuel.hoplite.ConfigResult
import com.sksamuel.hoplite.MapNode
import com.sksamuel.hoplite.Node
import com.sksamuel.hoplite.PropertySource
import com.sksamuel.hoplite.PropertySourceContext
import com.sksamuel.hoplite.fp.flatMap
import com.sksamuel.hoplite.fp.invalid
import com.sksamuel.hoplite.fp.valid
import org.apache.logging.log4j.Logger

/**
 * Maps a deprecated key path to its replacement, both as dot separated paths (e.g. `database.schema`).
 *
 * When a config file sets [oldPath], its value is applied to [newPath] and a deprecation warning is logged.
 * Setting both keys in the same file is an error. Layering across files keeps working: the file with the
 * highest priority wins, whichever of the two keys it uses.
 */
data class DeprecatedKeyAlias(val oldPath: String, val newPath: String) {
  init {
    require(oldPath.isNotBlank() && newPath.isNotBlank() && oldPath != newPath) {
      "Invalid alias: '$oldPath' -> '$newPath'"
    }
  }
}

/** Aliases for keys that were renamed in the coordinator config. */
val coordinatorDeprecatedKeyAliases: List<DeprecatedKeyAlias> =
  listOf(
    // it is the database name, passed to PgConnectOptions.setDatabase
    DeprecatedKeyAlias(oldPath = "database.schema", newPath = "database.database-name"),
  )

/** Ignores case and the `-`/`_` separators, as Hoplite does when mapping TOML keys to constructor parameters. */
internal fun normalizeConfigKey(key: String): String = key.replace("-", "").replace("_", "").lowercase()

/**
 * Wraps a [PropertySource] and rewrites the root node of what it provides.
 */
internal class TransformedPropertySource(
  private val delegate: PropertySource,
  private val transform: (MapNode) -> ConfigResult<Node>,
) : PropertySource {
  override fun source(): String = delegate.source()

  override fun node(context: PropertySourceContext): ConfigResult<Node> =
    delegate.node(context).flatMap { root ->
      if (root is MapNode) transform(root) else root.valid()
    }
}

/** Drops the given top-level tables, so they are not reported as unknown keys. */
internal fun PropertySource.withoutTopLevelKeys(keys: Set<String>): PropertySource {
  if (keys.isEmpty()) return this
  val normalized = keys.map(::normalizeConfigKey).toSet()
  return TransformedPropertySource(this) { root ->
    root.copy(map = root.map.filterKeys { normalizeConfigKey(it) !in normalized }).valid()
  }
}

/** Keeps only the given top-level tables. */
internal fun PropertySource.onlyTopLevelKeys(keys: Set<String>): PropertySource {
  val normalized = keys.map(::normalizeConfigKey).toSet()
  return TransformedPropertySource(this) { root ->
    root.copy(map = root.map.filterKeys { normalizeConfigKey(it) in normalized }).valid()
  }
}

internal fun PropertySource.withDeprecatedAliases(
  aliases: List<DeprecatedKeyAlias>,
  logger: Logger?,
): PropertySource {
  if (aliases.isEmpty()) return this
  return TransformedPropertySource(this) { root ->
    var current: Node = root
    for (alias in aliases) {
      val result = applyAlias(current, alias, source(), logger)
      when (result) {
        is AliasResult.Failure -> return@TransformedPropertySource result.message.toFailure()
        is AliasResult.Applied -> current = result.node
      }
    }
    current.valid()
  }
}

private sealed interface AliasResult {
  data class Applied(val node: Node) : AliasResult

  data class Failure(val message: String) : AliasResult
}

private fun String.toFailure(): ConfigResult<Node> =
  ConfigFailure.PropertySourceFailure(this, IllegalArgumentException(this)).invalid()

private fun applyAlias(root: Node, alias: DeprecatedKeyAlias, source: String, logger: Logger?): AliasResult {
  val oldSegments = alias.oldPath.split('.')
  val newSegments = alias.newPath.split('.')
  val oldValue = root.find(oldSegments) ?: return AliasResult.Applied(root)
  if (root.find(newSegments) != null) {
    return AliasResult.Failure(
      "Config key '${alias.oldPath}' is deprecated in favour of '${alias.newPath}' and both are set in $source; " +
        "remove '${alias.oldPath}'",
    )
  }
  logger?.warn(
    "Config key '{}' is deprecated, use '{}' instead (found in {})",
    alias.oldPath,
    alias.newPath,
    source,
  )
  val withoutOld = root.remove(oldSegments)
  return AliasResult.Applied(withoutOld.put(newSegments, oldValue))
}

private fun Node.find(segments: List<String>): Node? {
  if (segments.isEmpty()) return this
  if (this !is MapNode) return null
  val child = map.entries.firstOrNull { normalizeConfigKey(it.key) == normalizeConfigKey(segments.first()) }
  return child?.value?.find(segments.drop(1))
}

private fun Node.remove(segments: List<String>): Node {
  if (this !is MapNode) return this
  val head = normalizeConfigKey(segments.first())
  val key = map.keys.firstOrNull { normalizeConfigKey(it) == head } ?: return this
  return if (segments.size == 1) {
    copy(map = map - key)
  } else {
    copy(map = map + (key to map.getValue(key).remove(segments.drop(1))))
  }
}

private fun Node.put(segments: List<String>, value: Node): Node {
  require(this is MapNode)
  val head = segments.first()
  if (segments.size == 1) return copy(map = map + (head to value))
  val existingKey = map.keys.firstOrNull { normalizeConfigKey(it) == normalizeConfigKey(head) } ?: head
  val child =
    map[existingKey] as? MapNode
      ?: MapNode(map = emptyMap(), pos = pos, path = path.with(existingKey))
  return copy(map = map + (existingKey to child.put(segments.drop(1), value)))
}

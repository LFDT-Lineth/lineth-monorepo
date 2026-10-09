package lineth.coordinator.config.v2.toml

import com.sksamuel.hoplite.ArrayNode
import com.sksamuel.hoplite.BooleanNode
import com.sksamuel.hoplite.ConfigFailure
import com.sksamuel.hoplite.ConfigResult
import com.sksamuel.hoplite.DoubleNode
import com.sksamuel.hoplite.LongNode
import com.sksamuel.hoplite.MapNode
import com.sksamuel.hoplite.Node
import com.sksamuel.hoplite.NullNode
import com.sksamuel.hoplite.PropertySource
import com.sksamuel.hoplite.PropertySourceContext
import com.sksamuel.hoplite.StringNode
import com.sksamuel.hoplite.Undefined
import com.sksamuel.hoplite.decoder.DotPath
import com.sksamuel.hoplite.fp.flatMap
import com.sksamuel.hoplite.fp.invalid
import com.sksamuel.hoplite.fp.valid
import org.apache.logging.log4j.Logger
import kotlin.reflect.full.primaryConstructor

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

/**
 * Collects the sub-tables of the table at [path] that are not [knownKeys] under [targetKey], so a table whose
 * sub-tables are chosen by its users (e.g. `signer.<registered-type>`) is not reported as unknown keys.
 * Works per file, so a layered file can override the settings of a type declared in another file.
 */
data class UnknownTablesCapture(val path: String, val targetKey: String, val knownKeys: Set<String>)

internal fun PropertySource.withCapturedTables(captures: List<UnknownTablesCapture>): PropertySource {
  if (captures.isEmpty()) return this
  return TransformedPropertySource(this) { root ->
    captures.fold<UnknownTablesCapture, Node>(root) { node, capture -> node.capture(capture) }.valid()
  }
}

private fun Node.capture(capture: UnknownTablesCapture): Node {
  val segments = capture.path.split('.')
  val known = (capture.knownKeys + capture.targetKey).map(::normalizeConfigKey).toSet()
  return update(segments) { table ->
    val moved = table.map.filter { (key, value) -> value is MapNode && normalizeConfigKey(key) !in known }
    if (moved.isEmpty()) {
      table
    } else {
      val targetKey =
        table.map.keys.firstOrNull { normalizeConfigKey(it) == normalizeConfigKey(capture.targetKey) }
          ?: capture.targetKey
      val target =
        table.map[targetKey] as? MapNode
          ?: MapNode(map = emptyMap(), pos = table.pos, path = table.path.with(targetKey))
      // Hoplite derives Map keys from the nodes' sourceKey (their full dotted path), so rebase the moved nodes
      val rebased =
        moved.mapValues { (key, node) ->
          node.rebase(
            oldPrefix = node.sourceKey ?: "${table.sourceKey}.$key",
            newPrefix = "${target.sourceKey}.$key",
            newPath = target.path.with(key),
          )
        }
      table.copy(map = (table.map - moved.keys) + (targetKey to target.copy(map = target.map + rebased)))
    }
  }
}

private fun Node.rebase(oldPrefix: String, newPrefix: String, newPath: DotPath): Node {
  fun String?.rebased() = this?.let { if (it.startsWith(oldPrefix)) newPrefix + it.removePrefix(oldPrefix) else it }
  return when (this) {
    is MapNode ->
      copy(
        path = newPath,
        sourceKey = sourceKey.rebased(),
        map = map.mapValues { (key, child) -> child.rebase(oldPrefix, newPrefix, newPath.with(key)) },
      )
    is ArrayNode ->
      copy(
        path = newPath,
        sourceKey = sourceKey.rebased(),
        elements = elements.map {
          it.rebase(oldPrefix, newPrefix, newPath)
        },
      )
    is StringNode -> copy(path = newPath, sourceKey = sourceKey.rebased())
    is BooleanNode -> copy(path = newPath, sourceKey = sourceKey.rebased())
    is LongNode -> copy(path = newPath, sourceKey = sourceKey.rebased())
    is DoubleNode -> copy(path = newPath, sourceKey = sourceKey.rebased())
    is NullNode -> copy(path = newPath, sourceKey = sourceKey.rebased())
    Undefined -> this
  }
}

private fun Node.update(segments: List<String>, transform: (MapNode) -> MapNode): Node {
  if (this !is MapNode) return this
  if (segments.isEmpty()) return transform(this)
  val head = normalizeConfigKey(segments.first())
  val key = map.keys.firstOrNull { normalizeConfigKey(it) == head } ?: return this
  return copy(map = map + (key to map.getValue(key).update(segments.drop(1), transform)))
}

/** The coordinator tables whose sub-tables are named after pluggable signer types. */
val coordinatorSignerTableCaptures: List<UnknownTablesCapture> by lazy {
  val signerKeys = SignerConfigToml::class.primaryConstructor!!.parameters.map { it.name!! }.toSet()
  listOf(
    "l1-submission.blob.signer",
    "l1-submission.aggregation.signer",
    "message-anchoring.signer",
  ).map { UnknownTablesCapture(it, targetKey = "registered-settings", knownKeys = signerKeys) }
}

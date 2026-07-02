package linea.coordinator.app

import com.sksamuel.hoplite.Masked
import linea.coordinator.config.v2.CoordinatorConfig
import linea.coordinator.config.v2.SignerConfig
import linea.kotlin.encodeHex
import org.apache.logging.log4j.Logger
import kotlin.reflect.full.memberProperties
import kotlin.reflect.full.primaryConstructor
import kotlin.reflect.jvm.isAccessible
import kotlin.reflect.jvm.javaGetter
import kotlin.time.Duration
import kotlin.time.Instant

private const val PLACEHOLDER_HINT = "enable TRACE on linea.coordinator.app for full list"

// (declaringClass.simpleName, ctorParamName) of Map-valued fields that the INFO
// render replaces with a one-line summary instead of expanding entry-by-entry.
private val noisyMapFields: Set<String> = setOf(
  "CoordinatorConfig.smartContractErrors",
  "DynamicGasPriceCapConfig.timeOfDayMultipliers",
  "GasPriceCapCalculationConfig.timeOfTheDayMultipliers",
)

/**
 * Flattens the config into one fully-qualified `path: value` line per leaf, e.g.
 * `p2p.reputation.cooldownPeriod: 30s`. Secrets render as `***`; maps in [noisyMapFields] collapse
 * to an entry-count summary when [summarizeNoisyFields] is true.
 */
internal fun CoordinatorConfig.toPrettyLogLines(summarizeNoisyFields: Boolean = true): List<String> {
  val lines = mutableListOf<String>()
  renderObject(this, prefix = "", lines, summarize = summarizeNoisyFields)
  return lines
}

/** Joins [toPrettyLogLines] into a single multi-line string (one leaf per line). */
internal fun CoordinatorConfig.toPrettyLog(summarizeNoisyFields: Boolean = true): String =
  toPrettyLogLines(summarizeNoisyFields).joinToString("\n")

/** Logs the config one leaf per INFO event, each prefixed with [linePrefix]. */
internal fun CoordinatorConfig.logPretty(
  log: Logger,
  linePrefix: String = "config: ",
  summarizeNoisyFields: Boolean = true,
) {
  toPrettyLogLines(summarizeNoisyFields).forEach { log.info("{}{}", linePrefix, it) }
}

private fun renderValue(value: Any?, path: String, lines: MutableList<String>, summarize: Boolean) {
  when (value) {
    null -> lines.add("$path: null")
    is Masked -> lines.add("$path: ***")
    is Duration -> lines.add("$path: $value")
    is Instant -> lines.add("$path: $value")
    is ByteArray -> lines.add("$path: ${value.encodeHex()}")
    is SignerConfig.Web3jConfig ->
      lines.add("$path.privateKey: ***${value.privateKey.size} bytes***")
    is Number, is Boolean, is Enum<*> -> lines.add("$path: $value")
    is CharSequence -> lines.add("$path: $value")
    is Map<*, *> -> renderMap(value, path, lines)
    is List<*> -> renderList(value, path, lines, summarize)
    else -> if (value::class.isData) {
      renderObject(value, path, lines, summarize)
    } else {
      lines.add("$path: $value")
    }
  }
}

private fun renderMap(m: Map<*, *>, path: String, lines: MutableList<String>) {
  if (m.isEmpty()) {
    lines.add("$path: {}")
    return
  }
  m.forEach { (k, v) -> renderValue(v, "$path.$k", lines, summarize = false) }
}

private fun renderList(list: List<*>, path: String, lines: MutableList<String>, summarize: Boolean) {
  if (list.isEmpty()) {
    lines.add("$path: []")
    return
  }
  list.forEachIndexed { index, item -> renderValue(item, "$path[$index]", lines, summarize) }
}

private fun renderObject(value: Any, prefix: String, lines: MutableList<String>, summarize: Boolean) {
  val kClass = value::class
  val ctor = kClass.primaryConstructor
  if (ctor == null) {
    lines.add(if (prefix.isEmpty()) value.toString() else "$prefix: $value")
    return
  }
  val props = kClass.memberProperties.associateBy { it.name }
  ctor.parameters.forEach { p ->
    val name = p.name ?: return@forEach
    val prop = props[name] ?: return@forEach
    prop.isAccessible = true
    val v = try {
      // Kotlin reflection — preserves value-class boxing (e.g. Duration → Duration, not raw Long).
      prop.getter.call(value)
    } catch (_: Throwable) {
      // Falls over on some nullable-nested-value-class shapes (e.g. BlockParameter.BlockNumber?).
      // Java reflection returns the unboxed primitive in those cases; readable enough for a log.
      prop.javaGetter?.also { it.isAccessible = true }?.invoke(value)
    }
    val childPath = if (prefix.isEmpty()) name else "$prefix.$name"
    if (summarize && v is Map<*, *> && "${kClass.simpleName}.$name" in noisyMapFields) {
      lines.add("$childPath: <${v.size} entries, $PLACEHOLDER_HINT>")
    } else {
      renderValue(v, childPath, lines, summarize)
    }
  }
}

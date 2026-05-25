package linea.coordinator.app

import com.sksamuel.hoplite.Masked
import linea.coordinator.config.v2.CoordinatorConfig
import linea.coordinator.config.v2.SignerConfig
import linea.kotlin.encodeHex
import kotlin.reflect.full.memberProperties
import kotlin.reflect.full.primaryConstructor
import kotlin.reflect.jvm.isAccessible
import kotlin.reflect.jvm.javaGetter
import kotlin.time.Duration
import kotlin.time.Instant

private const val INDENT_WIDTH = 2
private const val PLACEHOLDER_HINT = "enable TRACE on linea.coordinator.app for full list"

// (declaringClass.simpleName, ctorParamName) of Map-valued fields that the INFO
// render replaces with a one-line summary instead of expanding entry-by-entry.
private val noisyMapFields: Set<String> = setOf(
  "CoordinatorConfig.smartContractErrors",
  "DynamicGasPriceCapConfig.timeOfDayMultipliers",
  "GasPriceCapCalculationConfig.timeOfTheDayMultipliers",
)

internal fun CoordinatorConfig.toPrettyLog(summarizeNoisyFields: Boolean = true): String =
  renderRoot(this, summarize = summarizeNoisyFields)

private fun renderRoot(value: Any, summarize: Boolean): String {
  val sb = StringBuilder()
  renderObject(value, indent = -INDENT_WIDTH, sb, summarize)
  return sb.toString().trimStart('\n')
}

private fun renderValue(value: Any?, indent: Int, sb: StringBuilder, summarize: Boolean) {
  when (value) {
    null -> sb.append(" null")
    is Masked -> sb.append(" ***")
    is Duration -> sb.append(' ').append(value.toString())
    is Instant -> sb.append(' ').append(value.toString())
    is ByteArray -> sb.append(' ').append(value.encodeHex())
    is SignerConfig.Web3jConfig -> {
      sb.append('\n').append(spaces(indent + INDENT_WIDTH))
        .append("privateKey: ***").append(value.privateKey.size).append(" bytes***")
    }
    is Number, is Boolean, is Enum<*> -> sb.append(' ').append(value.toString())
    is CharSequence -> sb.append(' ').append(value.toString())
    is Map<*, *> -> renderMap(value, indent, sb)
    is List<*> -> renderList(value, indent, sb, summarize)
    else -> if (value::class.isData) {
      renderObject(value, indent, sb, summarize)
    } else {
      sb.append(' ').append(value.toString())
    }
  }
}

private fun renderMap(m: Map<*, *>, indent: Int, sb: StringBuilder) {
  if (m.isEmpty()) {
    sb.append(" {}")
    return
  }
  m.forEach { (k, v) ->
    sb.append('\n').append(spaces(indent + INDENT_WIDTH)).append(k.toString()).append(':')
    renderValue(v, indent + INDENT_WIDTH, sb, summarize = false)
  }
}

private fun renderList(list: List<*>, indent: Int, sb: StringBuilder, summarize: Boolean) {
  if (list.isEmpty()) {
    sb.append(" []")
    return
  }
  list.forEach { item ->
    sb.append('\n').append(spaces(indent + INDENT_WIDTH)).append('-')
    renderValue(item, indent + INDENT_WIDTH, sb, summarize)
  }
}

private fun renderObject(value: Any, indent: Int, sb: StringBuilder, summarize: Boolean) {
  val kClass = value::class
  val ctor = kClass.primaryConstructor
  if (ctor == null) {
    sb.append(' ').append(value.toString())
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
    sb.append('\n').append(spaces(indent + INDENT_WIDTH)).append(name).append(':')
    if (summarize && v is Map<*, *> && "${kClass.simpleName}.$name" in noisyMapFields) {
      sb.append(" <").append(v.size).append(" entries, ").append(PLACEHOLDER_HINT).append('>')
    } else {
      renderValue(v, indent + INDENT_WIDTH, sb, summarize)
    }
  }
}

private fun spaces(n: Int): String = if (n <= 0) "" else " ".repeat(n)

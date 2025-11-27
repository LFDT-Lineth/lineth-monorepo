package net.consensys.linea.traces

import linea.kotlin.KMath

/**
 * These operations are immutable. Each operation creates a new instance.
 */
interface TracesCounters {
  operator fun get(module: TracingModule): UInt
  fun entries(): Set<Pair<TracingModule, UInt>>
  fun allTracesWithinLimits(tracesCountersLimits: TracesCounters): Boolean
  fun oversizedTraces(tracesCountersLimit: TracesCounters): List<Triple<TracingModule, UInt, UInt>>
  fun add(o: TracesCounters): TracesCounters
  val emptyTracesCounters: TracesCounters
}

abstract class TracesCountersImpl internal constructor(
  private val countersMap: Map<out TracingModule, UInt>,
  private val modules: List<TracingModule>,
) : TracesCounters {
  init {
    require(countersMap.size == modules.size && countersMap.keys.containsAll(modules)) {
      "Traces counters are incomplete. " +
        "Missing modules=${modules - countersMap.keys} " +
        "Extra modules=${countersMap.keys - modules} "
    }
  }

  override fun get(module: TracingModule): UInt = countersMap[module]!!

  override fun entries(): Set<Pair<TracingModule, UInt>> {
    return countersMap.entries.map { it.key to it.value }.toSet()
  }

  override fun allTracesWithinLimits(tracesCountersLimits: TracesCounters): Boolean {
    return countersMap.entries.all { (moduleName, moduleCount) ->
      val moduleCap = tracesCountersLimits[moduleName]
      moduleCount <= moduleCap
    }
  }

  /**
   * Returns a list of modules that have exceeded the limits provided as argument.
   * Each element of the list is a triple of the module, the current count and the limit.
   */
  override fun oversizedTraces(tracesCountersLimit: TracesCounters): List<Triple<TracingModule, UInt, UInt>> {
    val overSizedTraces = mutableListOf<Triple<TracingModule, UInt, UInt>>()
    for (moduleEntry in countersMap.entries) {
      val moduleCap = tracesCountersLimit[moduleEntry.key]
      if (moduleEntry.value > moduleCap) {
        overSizedTraces.add(Triple(moduleEntry.key, moduleEntry.value, moduleCap))
      }
    }
    return overSizedTraces
  }

  override fun toString(): String {
    return countersMap.entries
      .sortedBy { it.value }.joinToString(prefix = "[", postfix = "]", separator = " ") { (module, count) ->
        "$module=$count"
      }
  }
}

private fun add(tc1: TracesCounters, tc2: TracesCounters): Map<TracingModule, UInt> {
  if (tc1::class.java != tc2::class.java) {
    throw IllegalArgumentException(
      "Cannot add different traces counters. " +
        "Adding ${tc1::class.java} to ${tc2::class.java}",
    )
  }

  val tc2Entries = tc2.entries().toMap()
  val sum: Map<TracingModule, UInt> = tc1.entries().toMap().mapValues { (module, count) ->
    KMath.addExact(tc2Entries[module]!!, count)
  }

  return sum
}

data class TracesCountersV2(private val countersMap: Map<TracingModuleV2, UInt>) :
  TracesCountersImpl(countersMap, TracingModuleV2.entries) {
  companion object {
    val EMPTY_TRACES_COUNT = TracesCountersV2(TracingModuleV2.entries.associateWith { 0u })
  }

  override fun add(o: TracesCounters): TracesCountersV2 {
    val sum = add(this, o)
    @Suppress("UNCHECKED_CAST")
    return TracesCountersV2(sum as Map<TracingModuleV2, UInt>)
  }

  override val emptyTracesCounters = EMPTY_TRACES_COUNT
}

data class TracesCountersV4(private val countersMap: Map<TracingModuleV4, UInt>) :
  TracesCountersImpl(countersMap, TracingModuleV4.entries) {
  companion object {
    val EMPTY_TRACES_COUNT = TracesCountersV4(TracingModuleV4.entries.associateWith { 0u })
  }

  override fun add(o: TracesCounters): TracesCountersV4 {
    val sum = add(this, o)
    @Suppress("UNCHECKED_CAST")
    return TracesCountersV4(sum as Map<TracingModuleV4, UInt>)
  }

  override val emptyTracesCounters = EMPTY_TRACES_COUNT
}

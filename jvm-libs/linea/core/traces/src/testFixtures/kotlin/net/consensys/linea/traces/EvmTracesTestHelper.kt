package net.consensys.linea.traces

import kotlin.random.Random
import kotlin.random.nextUInt

fun fakeTracesCountersV5(defaultValue: UInt?, moduleValue: Map<TracingModuleV5, UInt> = emptyMap()): TracesCountersV5 {
  return TracesCountersV5(
    TracingModuleV5.entries.associateWith {
      moduleValue[it] ?: defaultValue ?: Random.nextUInt(0u, UInt.MAX_VALUE)
    },
  )
}

package net.consensys.linea.traces

import com.sksamuel.hoplite.ConfigLoaderBuilder
import com.sksamuel.hoplite.addFileSource
import net.consensys.linea.testing.filesystem.findPathTo
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows

class TracesCountersTest {
  data class TracesConfigV4(val tracesLimits: Map<TracingModuleV4, UInt>)

  @Test
  fun `configs v4 match specifiedModules`() {
    val path = findPathTo("config/common/traces-limits-v4.4.toml")

    val tracesConfig = ConfigLoaderBuilder.default()
      .addFileSource(path.toString())
      .build()
      .loadConfigOrThrow<TracesConfigV4>()

    val tracesCountersLimit = TracesCountersV4(tracesConfig.tracesLimits)

    tracesCountersLimit.entries().forEach { moduleLimit ->
      // PoW 2 requirement only apply to traces passed to the prover
      if (TracingModuleV4.evmModules.contains(moduleLimit.first) && TracingModuleV4.BLOCK_HASH != moduleLimit.first) {
        val isPowerOf2 = (moduleLimit.second and (moduleLimit.second - 1u)) == 0u
        assertThat(isPowerOf2)
          .withFailMessage("Trace limit ${moduleLimit.first}=${moduleLimit.second} is not a power of 2!")
          .isTrue()
      }
    }
  }

  @Test
  fun add_notOverflow() {
    val counters1 = fakeTracesCountersV2(10u)
    val counters2 = fakeTracesCountersV2(20u)
    val counters3 = fakeTracesCountersV2(20u)
    assertThat(counters1.add(counters2).add(counters3))
      .isEqualTo(fakeTracesCountersV2(50u))
  }

  @Test
  fun add_Overflow_throwsError() {
    val counters1 = fakeTracesCountersV2(10u)
    val counters2 = fakeTracesCountersV2(UInt.MAX_VALUE)
    assertThatThrownBy { counters1.add(counters2) }.isInstanceOf(ArithmeticException::class.java)
      .withFailMessage("integer overflow")
  }

  @Test
  fun allTracesWithinLimits() {
    val limits = fakeTracesCountersV2(20u, mapOf(Pair(TracingModuleV2.ADD, 10u)))
    val countersWithinLimits = fakeTracesCountersV2(3u)
    val countersOvertLimits = fakeTracesCountersV2(5u, mapOf(Pair(TracingModuleV2.ADD, 11u)))

    assertThat(countersWithinLimits.allTracesWithinLimits(limits)).isTrue()
    assertThat(countersOvertLimits.allTracesWithinLimits(limits)).isFalse()
  }

  @Test
  fun empty_counters() {
    val tracesCountersV2 = TracesCountersV2(
      TracingModuleV2.entries.associateWith { 0u },
    )
    assertThat(tracesCountersV2).isEqualTo(TracesCountersV2.EMPTY_TRACES_COUNT)
  }

  @Test
  fun incomplete_counters_throwsError() {
    assertThrows<IllegalArgumentException> {
      TracesCountersV2(emptyMap())
    }
    assertThrows<IllegalArgumentException> {
      TracesCountersV2(mapOf(Pair(TracingModuleV2.ADD, 10u)))
    }
  }

  @Test
  fun oversizedTraces() {
    val limits = fakeTracesCountersV2(20u, mapOf(Pair(TracingModuleV2.ADD, 10u)))
    val countersWithinLimits = fakeTracesCountersV2(3u)
    val countersOvertLimits = fakeTracesCountersV2(5u, mapOf(Pair(TracingModuleV2.ADD, 11u)))

    assertThat(countersWithinLimits.oversizedTraces(limits)).isEmpty()
    val oversizedTraces = countersOvertLimits.oversizedTraces(limits)
    assertThat(oversizedTraces).hasSize(1)
    assertThat(oversizedTraces.first()).isEqualTo(Triple(TracingModuleV2.ADD, 11u, 10u))
  }
}

package net.consensys.linea.traces

import com.sksamuel.hoplite.ConfigLoaderBuilder
import com.sksamuel.hoplite.addFileSource
import net.consensys.linea.testing.filesystem.findPathTo
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows

class TracesCountersTest {
  data class TracesConfigV5(val tracesLimits: Map<TracingModuleV5, UInt>)

  @Test
  fun `configs v5 match specifiedModules`() {
    val path = findPathTo("docker/config/common/traces-limits-v5.toml")

    val tracesConfig = ConfigLoaderBuilder.default()
      .addFileSource(path.toString())
      .build()
      .loadConfigOrThrow<TracesConfigV5>()

    val tracesCountersLimit = TracesCountersV5(tracesConfig.tracesLimits)

    tracesCountersLimit.entries().forEach { moduleLimit ->
      // PoW 2 requirement only apply to traces passed to the prover
      if (TracingModuleV5.evmModules.contains(moduleLimit.first) && TracingModuleV5.BLOCK_HASH != moduleLimit.first) {
        val isPowerOf2 = (moduleLimit.second and (moduleLimit.second - 1u)) == 0u
        assertThat(isPowerOf2)
          .withFailMessage("Trace limit ${moduleLimit.first}=${moduleLimit.second} is not a power of 2!")
          .isTrue()
      }
    }
  }

  @Test
  fun add_notOverflow() {
    val counters1 = fakeTracesCountersV5(10u)
    val counters2 = fakeTracesCountersV5(20u)
    val counters3 = fakeTracesCountersV5(20u)
    assertThat(counters1.add(counters2).add(counters3))
      .isEqualTo(fakeTracesCountersV5(50u))
  }

  @Test
  fun add_Overflow_throwsError() {
    val counters1 = fakeTracesCountersV5(10u)
    val counters2 = fakeTracesCountersV5(UInt.MAX_VALUE)
    assertThatThrownBy { counters1.add(counters2) }.isInstanceOf(ArithmeticException::class.java)
      .withFailMessage("integer overflow")
  }

  @Test
  fun allTracesWithinLimits() {
    val limits = fakeTracesCountersV5(20u, mapOf(Pair(TracingModuleV5.ADD, 10u)))
    val countersWithinLimits = fakeTracesCountersV5(3u)
    val countersOvertLimits = fakeTracesCountersV5(5u, mapOf(Pair(TracingModuleV5.ADD, 11u)))

    assertThat(countersWithinLimits.allTracesWithinLimits(limits)).isTrue()
    assertThat(countersOvertLimits.allTracesWithinLimits(limits)).isFalse()
  }

  @Test
  fun empty_counters() {
    val tracesCountersV5 = TracesCountersV5(
      TracingModuleV5.entries.associateWith { 0u },
    )
    assertThat(tracesCountersV5).isEqualTo(TracesCountersV5.EMPTY_TRACES_COUNT)
  }

  @Test
  fun incomplete_counters_throwsError() {
    assertThrows<IllegalArgumentException> {
      TracesCountersV5(emptyMap())
    }
    assertThrows<IllegalArgumentException> {
      TracesCountersV5(mapOf(Pair(TracingModuleV5.ADD, 10u)))
    }
  }

  @Test
  fun oversizedTraces() {
    val limits = fakeTracesCountersV5(20u, mapOf(Pair(TracingModuleV5.ADD, 10u)))
    val countersWithinLimits = fakeTracesCountersV5(3u)
    val countersOvertLimits = fakeTracesCountersV5(5u, mapOf(Pair(TracingModuleV5.ADD, 11u)))

    assertThat(countersWithinLimits.oversizedTraces(limits)).isEmpty()
    val oversizedTraces = countersOvertLimits.oversizedTraces(limits)
    assertThat(oversizedTraces).hasSize(1)
    assertThat(oversizedTraces.first()).isEqualTo(Triple(TracingModuleV5.ADD, 11u, 10u))
  }
}

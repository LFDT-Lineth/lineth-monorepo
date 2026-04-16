package net.consensys.zkevm.coordinator.blockcreation

import linea.domain.createBlock
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import java.util.concurrent.atomic.AtomicLong
import kotlin.time.Instant

class ConflationTargetCheckpointPauseControllerTest {
  private class TestL1Provider(
    initial: Long = 0L,
  ) : LatestL1FinalizedBlockProviderSync {
    private val inner = AtomicLong(initial)
    override fun getLatestL1FinalizedBlock(): Long = inner.get()
    fun setLatest(block: Long) {
      inner.set(block)
    }
  }

  private fun pauseConfig(
    initialTs: Instant = Instant.fromEpochSeconds(0L),
    targetEndBlocks: Set<ULong> = emptySet(),
    targetTimestamps: List<Instant> = emptyList(),
    waitL1: Boolean = false,
    waitApi: Boolean = false,
  ): ConflationTargetCheckpointPauseController.Config =
    ConflationTargetCheckpointPauseController.Config(
      initialLastImportedBlockTimestamp = initialTs,
      targetEndBlocks = targetEndBlocks,
      targetTimestamps = targetTimestamps,
      waitTargetBlockL1Finalization = waitL1,
      waitApiResumeAfterTargetBlock = waitApi,
    )

  @Test
  fun `does nothing when both wait switches are false`() {
    val l1 = TestL1Provider()
    val c =
      ConflationTargetCheckpointPauseController(
        pauseConfig(
          targetEndBlocks = setOf(9uL, 19uL),
        ),
        l1,
      )
    assertThat(c.pauseFeatureEnabled).isFalse()
    c.importBlock(createBlock(number = 10uL))
    assertThat(c.shouldPauseConflation()).isFalse()
    c.importBlock(createBlock(number = 20uL))
    assertThat(c.shouldPauseConflation()).isFalse()
  }

  @Test
  fun `pauses at target block number plus one and resumes on L1 only`() {
    val l1 = TestL1Provider()
    val c =
      ConflationTargetCheckpointPauseController(
        pauseConfig(
          targetEndBlocks = setOf(9uL, 19uL, 29uL),
          waitL1 = true,
        ),
        l1,
      )
    c.importBlock(createBlock(number = 10uL))
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(8L)
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(9L)
    assertThat(c.shouldPauseConflation()).isFalse()
    c.importBlock(createBlock(number = 20uL))
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(18L)
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(19L)
    assertThat(c.shouldPauseConflation()).isFalse()
    c.importBlock(createBlock(number = 30uL))
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(28L)
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(29L)
    assertThat(c.shouldPauseConflation()).isFalse()
  }

  @Test
  fun `timestamp pauses only on first block that crosses from strictly below threshold`() {
    val tbt1 = Instant.fromEpochSeconds(1000L)
    val tbt2 = Instant.fromEpochSeconds(2000L)
    val l1 = TestL1Provider()
    val c =
      ConflationTargetCheckpointPauseController(
        pauseConfig(
          targetTimestamps = listOf(tbt1, tbt2),
          waitL1 = true,
        ),
        l1,
      )
    c.importBlock(createBlock(number = 1uL, timestamp = Instant.fromEpochSeconds(999L)))
    assertThat(c.shouldPauseConflation()).isFalse()
    c.importBlock(createBlock(number = 2uL, timestamp = tbt1))
    assertThat(c.shouldPauseConflation()).isTrue()
  }

  @Test
  fun `multiple timestamp pauses in order of threshold`() {
    val tbt1 = Instant.fromEpochSeconds(1000L)
    val tbt2 = Instant.fromEpochSeconds(2000L)
    val l1 = TestL1Provider()
    val c =
      ConflationTargetCheckpointPauseController(
        pauseConfig(
          targetTimestamps = listOf(tbt1, tbt2),
          waitL1 = true,
        ),
        l1,
      )
    c.importBlock(createBlock(number = 3uL, timestamp = Instant.fromEpochSeconds(999L)))
    c.importBlock(createBlock(number = 4uL, timestamp = tbt1))
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(1L)
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(2L)
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(3L)
    assertThat(c.shouldPauseConflation()).isFalse()
    c.importBlock(createBlock(number = 5uL, timestamp = Instant.fromEpochSeconds(1500L)))
    assertThat(c.shouldPauseConflation()).isFalse()
    c.importBlock(createBlock(number = 10uL, timestamp = tbt2))
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(8L)
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(9L)
    assertThat(c.shouldPauseConflation()).isFalse()
  }

  @Test
  fun `pauses on timestamp threshold and resumes when L1 reaches target block timestamp by block number minus one`() {
    val tbt1 = Instant.fromEpochSeconds(1000L)
    val tbt2 = Instant.fromEpochSeconds(2000L)
    val l1 = TestL1Provider()
    val c =
      ConflationTargetCheckpointPauseController(
        pauseConfig(
          targetTimestamps = listOf(tbt1, tbt2),
          waitL1 = true,
        ),
        l1,
      )
    c.importBlock(
      createBlock(number = 5uL, timestamp = tbt1),
    )
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(3L)
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(4L)
    assertThat(c.shouldPauseConflation()).isFalse()
  }

  @Test
  fun `API resume when waitApi true`() {
    val l1 = TestL1Provider()
    val c =
      ConflationTargetCheckpointPauseController(
        pauseConfig(
          targetEndBlocks = setOf(9uL, 19uL),
          waitApi = true,
        ),
        l1,
      )
    c.importBlock(createBlock(number = 10uL))
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(100L)
    assertThat(c.shouldPauseConflation()).isTrue()
    assertThat(c.signalResumeFromApi()).isTrue()
    assertThat(c.shouldPauseConflation()).isFalse()
  }

  @Test
  fun `signalResumeFromApi returns false when API gate disabled`() {
    val l1 = TestL1Provider()
    val c =
      ConflationTargetCheckpointPauseController(
        pauseConfig(
          waitL1 = true,
        ),
        l1,
      )
    assertThat(c.signalResumeFromApi()).isFalse()
  }

  @Test
  fun `signalResumeFromApi returns false when no checkpoint pause active`() {
    val l1 = TestL1Provider()
    val c =
      ConflationTargetCheckpointPauseController(
        pauseConfig(
          targetEndBlocks = setOf(9uL, 19uL),
          waitApi = true,
        ),
        l1,
      )
    assertThat(c.signalResumeFromApi()).isFalse()
  }

  @Test
  fun `multiple target timestamps pause sequentially with independent L1 gates`() {
    val t1 = Instant.fromEpochSeconds(1000L)
    val t2 = Instant.fromEpochSeconds(2000L)
    val t3 = Instant.fromEpochSeconds(3000L)
    val l1 = TestL1Provider()
    val c =
      ConflationTargetCheckpointPauseController(
        pauseConfig(
          targetTimestamps = listOf(t1, t2, t3),
          waitL1 = true,
        ),
        l1,
      )
    c.importBlock(createBlock(number = 1uL, timestamp = Instant.fromEpochSeconds(500L)))
    assertThat(c.shouldPauseConflation()).isFalse()
    c.importBlock(createBlock(number = 2uL, timestamp = t1))
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(1L)
    assertThat(c.shouldPauseConflation()).isFalse()
    c.importBlock(createBlock(number = 3uL, timestamp = t2))
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(2L)
    assertThat(c.shouldPauseConflation()).isFalse()
    c.importBlock(createBlock(number = 4uL, timestamp = t3))
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(2L)
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(3L)
    assertThat(c.shouldPauseConflation()).isFalse()
  }

  @Test
  fun `single import crossing multiple timestamp thresholds uses first in list order for required L1 height`() {
    val t1 = Instant.fromEpochSeconds(1000L)
    val t2 = Instant.fromEpochSeconds(2000L)
    val l1 = TestL1Provider()
    val c =
      ConflationTargetCheckpointPauseController(
        pauseConfig(
          targetTimestamps = listOf(t1, t2),
          waitL1 = true,
        ),
        l1,
      )
    c.importBlock(createBlock(number = 1uL, timestamp = Instant.fromEpochSeconds(500L)))
    c.importBlock(createBlock(number = 2uL, timestamp = Instant.fromEpochSeconds(2500L)))
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(0L)
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(1L)
    assertThat(c.shouldPauseConflation()).isFalse()
  }

  @Test
  fun `multiple target end blocks and timestamps each gate conflation in order`() {
    val t1 = Instant.fromEpochSeconds(1000L)
    val t2 = Instant.fromEpochSeconds(2000L)
    val l1 = TestL1Provider()
    val c =
      ConflationTargetCheckpointPauseController(
        pauseConfig(
          initialTs = Instant.fromEpochSeconds(0L),
          targetEndBlocks = setOf(9uL, 14uL),
          targetTimestamps = listOf(t1, t2),
          waitL1 = true,
        ),
        l1,
      )
    c.importBlock(createBlock(number = 9uL, timestamp = Instant.fromEpochSeconds(500L)))
    assertThat(c.shouldPauseConflation()).isFalse()
    c.importBlock(createBlock(number = 10uL, timestamp = Instant.fromEpochSeconds(900L)))
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(8L)
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(9L)
    assertThat(c.shouldPauseConflation()).isFalse()
    c.importBlock(createBlock(number = 11uL, timestamp = Instant.fromEpochSeconds(1500L)))
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(10L)
    assertThat(c.shouldPauseConflation()).isFalse()
    c.importBlock(createBlock(number = 15uL, timestamp = Instant.fromEpochSeconds(1600L)))
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(13L)
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(14L)
    assertThat(c.shouldPauseConflation()).isFalse()
    c.importBlock(createBlock(number = 16uL, timestamp = t2))
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(14L)
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(15L)
    assertThat(c.shouldPauseConflation()).isFalse()
  }

  @Test
  fun `same import hitting block checkpoint and timestamp uses max of required L1 block numbers`() {
    val t1 = Instant.fromEpochSeconds(1000L)
    val t2 = Instant.fromEpochSeconds(2000L)
    val l1 = TestL1Provider()
    val c =
      ConflationTargetCheckpointPauseController(
        pauseConfig(
          initialTs = Instant.fromEpochSeconds(500L),
          targetEndBlocks = setOf(9uL, 19uL),
          targetTimestamps = listOf(t1, t2),
          waitL1 = true,
        ),
        l1,
      )
    c.importBlock(createBlock(number = 1uL, timestamp = Instant.fromEpochSeconds(600L)))
    c.importBlock(createBlock(number = 10uL, timestamp = Instant.fromEpochSeconds(1500L)))
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(8L)
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(9L)
    assertThat(c.shouldPauseConflation()).isFalse()
    c.importBlock(createBlock(number = 20uL, timestamp = Instant.fromEpochSeconds(2500L)))
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(18L)
    assertThat(c.shouldPauseConflation()).isTrue()
    l1.setLatest(19L)
    assertThat(c.shouldPauseConflation()).isFalse()
  }
}

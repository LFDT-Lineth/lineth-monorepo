package lineth.persistence.conflation

import io.vertx.junit5.VertxExtension
import linea.domain.BlobData
import linea.domain.RiscvRollupRecord
import linea.domain.createBlobData
import linea.domain.createRiscvRollupRecord
import linea.error.DuplicatedRecordException
import linea.kotlin.trimToSecondPrecision
import linea.persistence.db.DbHelper
import linea.persistence.db.test.CleanDbTestSuiteParallel
import lineth.persistence.RiscvRollupsDao
import net.consensys.FakeFixedClock
import net.consensys.linea.async.get
import net.consensys.linea.async.toSafeFuture
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.junit.jupiter.api.extension.ExtendWith
import java.util.concurrent.ExecutionException
import kotlin.random.Random
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

@ExtendWith(VertxExtension::class)
class RiscvRollupsPostgresDaoTest : CleanDbTestSuiteParallel() {
  init {
    // riscv_rollups, riscv_blobs and riscv_rollup_blobs are added in V5
    target = "5"
  }

  override val databaseName = DbHelper.generateUniqueDbName("coordinator-tests-riscv-rollups-dao")

  private val maxRollupsToReturn = 6u
  private val fakeClock = FakeFixedClock()
  private val startTime = fakeClock.now().trimToSecondPrecision()
  private lateinit var rollupsDao: RiscvRollupsDao

  @BeforeEach
  fun beforeEach() {
    rollupsDao = RiscvRollupsPostgresDao(
      config = RiscvRollupsPostgresDao.Config(maxRollupProofsToReturn = maxRollupsToReturn),
      connection = sqlClient,
      clock = fakeClock,
    )
  }

  private fun rollup(start: ULong, end: ULong, startBlockTimestamp: Instant = startTime): RiscvRollupRecord =
    createRiscvRollupRecord(startBlockNumber = start, endBlockNumber = end, startBlockTimestamp = startBlockTimestamp)

  private fun save(vararg rollups: RiscvRollupRecord) {
    rollups.forEach { rollupsDao.saveRollupWithBlobs(it).get() }
  }

  private fun count(table: String): Int =
    sqlClient.preparedQuery("select count(*) as n from $table")
      .execute()
      .toSafeFuture()
      .get()
      .first()
      .getInteger("n")

  private fun storedBlobHashes(): List<ByteArray> =
    sqlClient.preparedQuery("select data_rolling_hash from ${RiscvRollupsPostgresDao.BlobsTableName}")
      .execute()
      .toSafeFuture()
      .get()
      .map { it.getBuffer("data_rolling_hash").bytes }

  private fun assertStoredBlobs(vararg expected: BlobData) {
    assertThat(storedBlobHashes()).containsExactlyInAnyOrderElementsOf(expected.map { it.dataRollingHash })
  }

  // rollup1 = [b0, b1] ending inside b1; rollup2 = [b1 (shared), b2].
  private val hashes = List(4) { Random.nextBytes(32) }
  private val blobs = List(3) { createBlobData(parentDataRollingHash = hashes[it], dataRollingHash = hashes[it + 1]) }
  private val rollup1 = createRiscvRollupRecord(
    startBlockNumber = 1UL,
    endBlockNumber = 10UL,
    startBlockTimestamp = startTime,
    parentDataRollingHash = hashes[0],
    dataRollingHash = hashes[2],
    finalSharedBlobCarryOverTailBytes = 100u,
    blobsData = blobs.subList(0, 2),
  )
  private val rollup2 = createRiscvRollupRecord(
    startBlockNumber = 11UL,
    endBlockNumber = 20UL,
    startBlockTimestamp = startTime,
    parentDataRollingHash = hashes[2],
    dataRollingHash = hashes[3],
    parentSharedBlobCarryOverTailBytes = 100u,
    parentBlobDataRollingHash = hashes[1],
    blobsData = blobs.subList(1, 3),
  )

  @Test
  fun `saveRollupWithBlobs stores the rollup row and its blobs`() {
    val rollup = rollup(1UL, 100UL)
    save(rollup)

    val row = sqlClient.preparedQuery("select * from ${RiscvRollupsPostgresDao.RollupsTableName}")
      .execute()
      .toSafeFuture()
      .get()
      .single()
    assertThat(row.getLong("created_epoch_milli")).isEqualTo(fakeClock.now().toEpochMilliseconds())
    assertThat(row.getLong("start_block_number")).isEqualTo(1L)
    assertThat(row.getLong("end_block_number")).isEqualTo(100L)
    assertThat(row.getInteger("batches_count")).isEqualTo(rollup.totalBatchesCount.toInt())
    assertThat(row.getBuffer("proof_hash").bytes).isEqualTo(rollup.proofHash)
    assertStoredBlobs(*rollup.blobsData.toTypedArray())
    assertThat(rollupsDao.findRollupByStartBlockNumber(1UL).get()).isEqualTo(rollup)
  }

  @Test
  fun `saving a rollup that is already stored is ignored by default`() {
    val rollup = rollup(1UL, 100UL)
    save(rollup)

    rollupsDao.saveRollupWithBlobs(rollup).get()
    assertThat(count(RiscvRollupsPostgresDao.RollupsTableName)).isEqualTo(1)
    assertThat(rollupsDao.findRollupByStartBlockNumber(1UL).get()).isEqualTo(rollup)
  }

  @Test
  fun `saving a rollup that is already stored fails when duplicates are not ignored`() {
    val strictDao = RiscvRollupsPostgresDao(
      config = RiscvRollupsPostgresDao.Config(maxRollupProofsToReturn = maxRollupsToReturn, ignoreDuplicates = false),
      connection = sqlClient,
      clock = fakeClock,
    )
    val rollup = rollup(1UL, 100UL)
    save(rollup)

    assertThrows<ExecutionException> { strictDao.saveRollupWithBlobs(rollup).get() }
      .also { error ->
        assertThat(error.cause).isInstanceOf(DuplicatedRecordException::class.java)
        assertThat(error.cause!!.message).isEqualTo("Rollup [1..100]100 is already persisted!")
      }
    assertThat(count(RiscvRollupsPostgresDao.RollupsTableName)).isEqualTo(1)
  }

  @Test
  fun `saveRollupWithBlobs rejects a rollup that shares its start or end block with a stored one`() {
    save(rollup(11UL, 20UL))

    listOf(rollup(11UL, 25UL), rollup(5UL, 20UL)).forEach { conflicting ->
      assertThrows<ExecutionException> { rollupsDao.saveRollupWithBlobs(conflicting).get() }
        .also { error ->
          assertThat(error.cause).isNotInstanceOf(DuplicatedRecordException::class.java)
          assertThat(error.cause!!.message).contains("duplicate key value violates unique constraint")
        }
    }
    assertThat(count(RiscvRollupsPostgresDao.RollupsTableName)).isEqualTo(1)
    assertThat(count(RiscvRollupsPostgresDao.RollupBlobsTableName)).isEqualTo(1)
  }

  @Test
  fun `a rollup starting inside a shared blob round-trips with its tail fields`() {
    val rollup = createRiscvRollupRecord(
      startBlockNumber = 1UL,
      endBlockNumber = 10UL,
      startBlockTimestamp = startTime,
      parentSharedBlobCarryOverTailBytes = 100u,
      finalSharedBlobCarryOverTailBytes = 200u,
    )
    save(rollup)

    assertThat(rollupsDao.findRollupByEndBlockNumber(10UL).get()).isEqualTo(rollup)
  }

  @Test
  fun `findRollupByStartBlockNumber and findRollupByEndBlockNumber`() {
    val rollup = rollup(1UL, 90UL)
    save(rollup)

    assertThat(rollupsDao.findRollupByEndBlockNumber(90UL).get()).isEqualTo(rollup)
    assertThat(rollupsDao.findRollupByEndBlockNumber(91UL).get()).isNull()
    assertThat(rollupsDao.findRollupByStartBlockNumber(1UL).get()).isEqualTo(rollup)
    assertThat(rollupsDao.findRollupByStartBlockNumber(2UL).get()).isNull()
  }

  @Test
  fun `getConsecutiveRollupsFromBlockNumber returns nothing when no rollup starts at the block`() {
    save(rollup(1UL, 100UL), rollup(101UL, 200UL))

    assertThat(rollupsDao.getConsecutiveRollupsFromBlockNumber(2UL, Instant.DISTANT_FUTURE).get()).isEmpty()
  }

  @Test
  fun `getConsecutiveRollupsFromBlockNumber returns the gap-free run, cut by end block time`() {
    val r1 = rollup(1UL, 40UL)
    val r2 = rollup(41UL, 60UL, r1.endBlockTimestamp.plus(3.seconds))
    val r3 = rollup(61UL, 100UL, r2.endBlockTimestamp.plus(3.seconds))
    val r4 = rollup(101UL, 111UL, r3.endBlockTimestamp.plus(3.seconds))
    val r5 = rollup(112UL, 132UL, r4.endBlockTimestamp.plus(3.seconds))
    // gap at 133
    val r6 = rollup(134UL, 156UL, r5.endBlockTimestamp.plus(3.seconds))
    save(r1, r2, r3, r4, r5, r6)

    assertThat(rollupsDao.getConsecutiveRollupsFromBlockNumber(61UL, r5.endBlockTimestamp.plus(1.seconds)).get())
      .containsExactly(r3, r4, r5)
    assertThat(rollupsDao.getConsecutiveRollupsFromBlockNumber(61UL, r4.endBlockTimestamp.plus(1.seconds)).get())
      .containsExactly(r3, r4)
  }

  @Test
  fun `getConsecutiveRollupsFromBlockNumber stops at the first gap whatever the insertion order`() {
    val run = listOf(rollup(1UL, 10UL), rollup(11UL, 20UL), rollup(21UL, 30UL))
    val afterFirstGap = listOf(rollup(32UL, 40UL), rollup(41UL, 50UL))
    val afterSecondGap = listOf(rollup(60UL, 70UL), rollup(71UL, 80UL))
    save(*(afterSecondGap + afterFirstGap.reversed() + run.reversed()).toTypedArray())

    assertThat(rollupsDao.getConsecutiveRollupsFromBlockNumber(1UL, Instant.DISTANT_FUTURE).get())
      .containsExactlyElementsOf(run)
    assertThat(rollupsDao.getConsecutiveRollupsFromBlockNumber(32UL, Instant.DISTANT_FUTURE).get())
      .containsExactlyElementsOf(afterFirstGap)
  }

  @Test
  fun `getConsecutiveRollupsFromBlockNumber returns at most the configured number of rollups`() {
    val rollups = (0UL until 8UL).map { rollup(it * 10UL + 1UL, it * 10UL + 10UL) }
    save(*rollups.toTypedArray())

    assertThat(rollupsDao.getConsecutiveRollupsFromBlockNumber(1UL, Instant.DISTANT_FUTURE).get())
      .containsExactlyElementsOf(rollups.take(maxRollupsToReturn.toInt()))
  }

  @Test
  fun `findHighestConsecutiveEndBlockNumberFromBlockNumber returns the end of the gap-free run`() {
    save(rollup(41UL, 50UL), rollup(21UL, 30UL), rollup(1UL, 10UL), rollup(11UL, 20UL))

    assertThat(rollupsDao.findHighestConsecutiveEndBlockNumberFromBlockNumber(1UL).get()).isEqualTo(30UL)
    assertThat(rollupsDao.findHighestConsecutiveEndBlockNumberFromBlockNumber(21UL).get()).isEqualTo(30UL)
    assertThat(rollupsDao.findHighestConsecutiveEndBlockNumberFromBlockNumber(41UL).get()).isEqualTo(50UL)
    assertThat(rollupsDao.findHighestConsecutiveEndBlockNumberFromBlockNumber(31UL).get()).isNull()
    assertThat(rollupsDao.findHighestConsecutiveEndBlockNumberFromBlockNumber(5UL).get()).isNull()
  }

  @Test
  fun `a blob shared by two rollups is stored once`() {
    save(rollup1, rollup2)

    assertStoredBlobs(*blobs.toTypedArray())
    assertThat(count(RiscvRollupsPostgresDao.RollupBlobsTableName)).isEqualTo(4)
    assertThat(rollupsDao.getConsecutiveRollupsFromBlockNumber(1UL, Instant.DISTANT_FUTURE).get())
      .containsExactly(rollup1, rollup2)
  }

  @Test
  fun `deleteRollupsUpToEndBlockNumber keeps the blob shared with a remaining rollup`() {
    save(rollup1, rollup2)

    assertThat(rollupsDao.deleteRollupsUpToEndBlockNumber(10UL).get()).isEqualTo(1)
    assertStoredBlobs(blobs[1], blobs[2])
    assertThat(rollupsDao.findRollupByStartBlockNumber(11UL).get()).isEqualTo(rollup2)

    assertThat(rollupsDao.deleteRollupsUpToEndBlockNumber(20UL).get()).isEqualTo(1)
    assertStoredBlobs()
    assertThat(count(RiscvRollupsPostgresDao.RollupBlobsTableName)).isZero()
  }

  @Test
  fun `deleteRollupsAfterBlockNumber keeps the blobs of the remaining rollups`() {
    save(rollup1, rollup2)

    assertThat(rollupsDao.deleteRollupsAfterBlockNumber(11UL).get()).isEqualTo(1)
    assertStoredBlobs(blobs[0], blobs[1])
    assertThat(rollupsDao.findRollupByStartBlockNumber(1UL).get()).isEqualTo(rollup1)
  }

  @Test
  fun `a rollup saved after its shared blob was deleted stores the blob again`() {
    save(rollup1)
    rollupsDao.deleteRollupsUpToEndBlockNumber(10UL).get()
    assertStoredBlobs()

    save(rollup2)
    assertStoredBlobs(blobs[1], blobs[2])
    assertThat(rollupsDao.findRollupByStartBlockNumber(11UL).get()).isEqualTo(rollup2)
  }

  @Test
  fun `RISC-V rollups are not stored in the blobs table`() {
    save(rollup1)

    assertThat(count(BlobsPostgresDaoG.TableName)).isZero()
  }
}

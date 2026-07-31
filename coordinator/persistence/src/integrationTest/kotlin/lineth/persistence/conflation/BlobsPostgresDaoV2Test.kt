<<<<<<< HEAD:coordinator/persistence/src/integrationTest/kotlin/lineth/persistence/conflation/BlobsPostgresDaoV2Test.kt
<<<<<<< HEAD:coordinator/persistence/src/integrationTest/kotlin/lineth/persistence/conflation/BlobsPostgresDaoV2Test.kt
package lineth.persistence.conflation
=======
package linea.persistence.conflation
>>>>>>> 83adcc937 (chore(coordinator): riscv blob data skeleton (#3609)):coordinator/persistence/src/integrationTest/kotlin/linea/persistence/conflation/BlobsPostgresDaoV2Test.kt
=======
package linea.persistence.conflation
>>>>>>> 83adcc937 (chore(coordinator): riscv blob data skeleton (#3609)):coordinator/persistence/src/integrationTest/kotlin/linea/persistence/conflation/BlobsPostgresDaoV2Test.kt

import io.vertx.sqlclient.Row
import io.vertx.sqlclient.SqlClient
import linea.domain.BlobRecordV2
import linea.domain.createBlobRecordV2
import linea.persistence.db.DbHelper
import org.assertj.core.api.Assertions.assertThat
import kotlin.time.Clock
import kotlin.time.Instant

class BlobsPostgresDaoV2Test : BlobsPostgresDaoTestBase<BlobRecordV2>() {
  override val databaseName = DbHelper.generateUniqueDbName("coordinator-tests-blobs-dao-v2")

  override fun buildDao(maxBlobsToReturn: UInt, connection: SqlClient, clock: Clock): BlobsDaoG<BlobRecordV2> =
    BlobsPostgresDaoV2(
      config = BlobsPostgresDaoV2.Config(maxRollupProofsToReturn = maxBlobsToReturn),
      connection = connection,
      clock = clock,
    )

  override fun createBlob(startBlockNumber: ULong, endBlockNumber: ULong, startBlockTime: Instant): BlobRecordV2 =
    createBlobRecordV2(
      startBlockNumber = startBlockNumber,
      endBlockNumber = endBlockNumber,
      startBlockTimestamp = startBlockTime,
    )

  override fun parseRecord(row: Row): BlobRecordV2 = BlobsPostgresDaoV2.parseRecord(row)

  override fun endBlockTime(blob: BlobRecordV2): Instant = blob.endBlockTimestamp

  override fun startBlockNumber(blob: BlobRecordV2): ULong = blob.startBlockNumber

  override fun endBlockNumber(blob: BlobRecordV2): ULong = blob.endBlockNumber

  override fun assertAdditionalInsertedColumns(row: Row, blob: BlobRecordV2) {
    assertThat(row.getInteger("batches_count")).isEqualTo(blob.totalBatchesCount.toInt())
  }
}

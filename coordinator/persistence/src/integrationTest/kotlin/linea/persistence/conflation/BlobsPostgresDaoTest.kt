package linea.persistence.conflation

import io.vertx.sqlclient.Row
import io.vertx.sqlclient.SqlClient
import linea.domain.BlobRecord
import linea.domain.createBlobRecord
import linea.persistence.db.DbHelper
import kotlin.time.Clock
import kotlin.time.Instant

class BlobsPostgresDaoTest : BlobsPostgresDaoTestBase<BlobRecord>() {
  override val databaseName = DbHelper.generateUniqueDbName("coordinator-tests-blobs-dao")

  override fun buildDao(maxBlobsToReturn: UInt, connection: SqlClient, clock: Clock): BlobsDaoG<BlobRecord> =
    BlobsPostgresDao(
      config = BlobsPostgresDao.Config(maxBlobsToReturn),
      connection = connection,
      clock = clock,
    )

  override fun createBlob(startBlockNumber: ULong, endBlockNumber: ULong, startBlockTime: Instant): BlobRecord =
    createBlobRecord(
      startBlockNumber = startBlockNumber,
      endBlockNumber = endBlockNumber,
      startBlockTime = startBlockTime,
    )

  override fun parseRecord(row: Row): BlobRecord = BlobsPostgresDao.parseRecord(row)

  override fun endBlockTime(blob: BlobRecord): Instant = blob.endBlockTime

  override fun startBlockNumber(blob: BlobRecord): ULong = blob.startBlockNumber

  override fun endBlockNumber(blob: BlobRecord): ULong = blob.endBlockNumber
}

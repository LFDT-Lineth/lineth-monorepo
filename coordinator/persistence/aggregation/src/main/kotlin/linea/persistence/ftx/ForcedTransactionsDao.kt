package linea.persistence.ftx

import net.consensys.zkevm.domain.ForcedTransactionRecord
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface ForcedTransactionsDao {
  fun save(ftx: ForcedTransactionRecord): SafeFuture<Unit>
  fun findByNumber(ftxNumber: ULong): SafeFuture<ForcedTransactionRecord?>
  fun list(): SafeFuture<List<ForcedTransactionRecord>>
  fun deleteFtxUpToInclusive(ftxNumber: ULong): SafeFuture<Int>
  fun findHighestForcedTransaction(): SafeFuture<ForcedTransactionRecord?> {
    return list().thenApply { it.maxByOrNull { ftx -> ftx.ftxNumber } }
  }
}

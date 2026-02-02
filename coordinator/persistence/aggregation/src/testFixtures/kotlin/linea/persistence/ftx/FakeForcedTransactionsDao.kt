package linea.persistence.ftx

import net.consensys.zkevm.domain.ForcedTransactionRecord
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ConcurrentHashMap

class FakeForcedTransactionsDao(
  var records: MutableMap<ULong, ForcedTransactionRecord> = ConcurrentHashMap(),
) : ForcedTransactionsDao {

  override fun save(ftx: ForcedTransactionRecord): SafeFuture<Unit> {
    records[ftx.ftxNumber] = ftx
    return SafeFuture.completedFuture(Unit)
  }

  override fun findByNumber(ftxNumber: ULong): SafeFuture<ForcedTransactionRecord?> {
    return SafeFuture.completedFuture(records[ftxNumber])
  }

  override fun list(): SafeFuture<List<ForcedTransactionRecord>> {
    return SafeFuture.completedFuture(records.values.toList().sortedBy { it.ftxNumber })
  }

  override fun deleteFtxUpToInclusive(ftxNumber: ULong): SafeFuture<Int> {
    var count = 0
    records.keys.forEach {
      if (it <= ftxNumber) {
        count++
        records.remove(it)
      }
    }
    return SafeFuture.completedFuture(count)
  }
}

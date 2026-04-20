package net.consensys.zkevm.persistence.db

import io.vertx.pgclient.PgException
import linea.error.DuplicatedRecordException

fun isDuplicateKeyException(th: Throwable): Boolean {
  return when (th) {
    is PgException ->
      th.errorMessage.contains("duplicate key value violates unique constraint", ignoreCase = true)
    is DuplicatedRecordException ->
      th.message!!.contains("is already persisted", ignoreCase = true)
    else -> false
  }
}

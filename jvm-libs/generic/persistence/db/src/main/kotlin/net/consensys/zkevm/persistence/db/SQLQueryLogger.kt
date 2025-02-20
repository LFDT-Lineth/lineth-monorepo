package net.consensys.zkevm.persistence.db

import io.vertx.sqlclient.Tuple
import linea.kotlin.encodeHex
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.Logger

class SQLQueryLogger(private val log: Logger) {
  fun log(level: Level, query: String) {
    if (log.isEnabled(level)) {
      log.log(level, "Performing query:\"{}\"", query)
    }
  }

  fun log(level: Level, query: String, params: List<Any?>) {
    if (log.isEnabled(level)) {
      log.log(level, "Performing query:\"{}\" with params:\"{}\"", query, getLoggableParams(params))
    }
  }

  private fun getLoggableParams(params: List<Any?>): String {
    return params.joinToString(",") { param: Any? -> getLoggableParam(param) }
  }

  private fun getLoggableParams(tuple: Tuple): String {
    val s = StringBuilder("(")
    for (i in 0 until tuple.size()) {
      s.append(getLoggableParam(tuple.getValue(i)))
      if (i < tuple.size() - 1) {
        s.append(",")
      }
    }
    s.append(")")
    return s.toString()
  }

  private fun getLoggableParam(param: Any?): String {
    if (param is ByteArray) {
      return param.encodeHex(prefix = true)
    } else if (param is Tuple) {
      return getLoggableParams(param)
    }
    return param.toString()
  }
}

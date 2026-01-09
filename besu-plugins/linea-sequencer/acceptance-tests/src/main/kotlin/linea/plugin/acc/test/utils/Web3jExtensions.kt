package linea.plugin.acc.test.utils

import org.web3j.protocol.core.Response

fun Response.Error.toLogString(): String {
  return "Error(code=$code, message=$message, data=$data)"
}

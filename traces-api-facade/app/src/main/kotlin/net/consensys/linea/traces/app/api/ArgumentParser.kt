package net.consensys.linea.traces.app.api

import linea.kotlin.fromHexString
import net.consensys.linea.jsonrpc.argument.getArgument
import org.apache.tuweni.bytes.Bytes32

object ArgumentParser {
  fun getHexString(arguments: List<*>, index: Int, argumentName: String): String {
    return getArgument(String::class, arguments, index, argumentName)
      .also { require(it.startsWith("0x")) { "$argumentName must have '0x' hexadecimal prefix." } }
  }

  fun <T> getHexStringParsed(arguments: List<*>, index: Int, argumentName: String, parser: (String) -> T): T {
    return getArgument(String::class, arguments, index, argumentName)
      .also { require(it.startsWith("0x")) { "$argumentName must have '0x' hexadecimal prefix." } }
      .let(parser)
  }

  fun getBlockNumber(arguments: List<*>, index: Int, argumentName: String): ULong {
    return getHexStringParsed(arguments, index, argumentName) {
      try {
        ULong.fromHexString(it)
      } catch (ex: NumberFormatException) {
        throw NumberFormatException("${ex.message} on argument $argumentName")
      }
    }
  }

  fun getBytes32(arguments: List<*>, index: Int, argumentName: String): Bytes32 {
    return getHexStringParsed(arguments, index, argumentName, Bytes32::fromHexString)
  }
}

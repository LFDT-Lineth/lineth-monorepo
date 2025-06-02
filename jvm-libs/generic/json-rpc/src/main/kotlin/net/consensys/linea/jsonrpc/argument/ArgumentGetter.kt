package net.consensys.linea.jsonrpc.argument

import kotlin.reflect.KClass

fun <T : Any> getArgument(
  clazz: KClass<T>,
  arguments: List<*>,
  argPosition: Int,
  argumentName: String,
): T {
  return getArgument(clazz, arguments, argPosition, argumentName, nullable = false)!!
}

fun <T : Any> getOptionalArgument(
  clazz: KClass<T>,
  arguments: List<*>,
  argPosition: Int,
  argumentName: String,
): T? {
  return getArgument(clazz, arguments, argPosition, argumentName, nullable = true)
}

internal fun <T : Any> getArgument(
  clazz: KClass<T>,
  arguments: List<*>,
  argPosition: Int,
  argumentName: String,
  nullable: Boolean = false,
): T? {
  require(arguments.size > argPosition) {
    "Argument $argumentName not provided in arguments list at position $argPosition. Total arguments ${arguments.size}"
  }

  val value =
    arguments[argPosition]
      ?: if (!nullable) {
        throw IllegalArgumentException(
          "Required argument $argumentName at position $argPosition is null.",
        )
      } else {
        return null
      }

  return TypeCast.safeCast(clazz.java, value, argumentName, nullable)
}

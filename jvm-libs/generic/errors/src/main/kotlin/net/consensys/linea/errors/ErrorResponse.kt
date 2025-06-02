package net.consensys.linea.errors

data class ErrorResponse<E>(val type: E, val message: String) {
  fun asException(messagePrefix: String? = null, messageSuffix: String? = null) =
    Exception(
      "${messagePrefix?.let { "$it: " } ?: ""}${this.type} ${this.message}.${messageSuffix?.let { " $it" } ?: ""}",
    )
}

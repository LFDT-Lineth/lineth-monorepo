package linea.coordinator.config.v2.docs

/**
 * Converts a camelCase Kotlin property name to the kebab-case form used to *display* config
 * keys in the generated documentation.
 *
 * The input is always the camelCase `KParameter.name` from a TOML schema class — TOML files
 * are never read. Hoplite matches keys case- and separator-insensitively, so the kebab-case
 * rendering is purely a presentation choice; it is the prevailing convention in the real
 * config files.
 *
 * A hyphen is inserted before an uppercase letter when it follows a lowercase letter or a
 * digit (`l1Endpoint` -> `l1-endpoint`, `maxFeePerGasCap` -> `max-fee-per-gas-cap`), or when
 * it ends an acronym that is followed by a lowercase letter (`httpServer`/`HTTPServer` ->
 * `http-server`). Digits stay attached to the preceding token
 * (`type2StateProofProvider` -> `type2-state-proof-provider`).
 */
fun camelToKebabCase(name: String): String {
  val sb = StringBuilder(name.length + 8)
  for (i in name.indices) {
    val c = name[i]
    if (c.isUpperCase()) {
      val prev = if (i > 0) name[i - 1] else null
      val next = if (i < name.length - 1) name[i + 1] else null
      val boundary = prev != null && (
        prev.isLowerCase() ||
          prev.isDigit() ||
          (prev.isUpperCase() && next != null && next.isLowerCase())
        )
      if (boundary) {
        sb.append('-')
      }
      sb.append(c.lowercaseChar())
    } else {
      sb.append(c)
    }
  }
  return sb.toString()
}

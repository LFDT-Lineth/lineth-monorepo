package linea

import java.net.URI

fun URI.getPortWithSchemeDefaults(): Int {
  return if (port != -1) {
    port
  } else {
    when (scheme.lowercase()) {
      "http" -> 80
      "https" -> 443
      // Focous on HTTP as it is what we need for now
      else -> throw IllegalArgumentException("Unsupported scheme: $scheme")
    }
  }
}

package net.consensys.linea.metrics.micrometer

import net.consensys.linea.metrics.MetricsCategory

fun MetricsCategory.toValidMicrometerName(): String {
  return this.name.lowercase().replace('_', '.')
}

package net.consensys.linea.metrics.micrometer

import net.consensys.linea.metrics.MetricsCategory
import net.consensys.linea.metrics.Tag
import io.micrometer.core.instrument.Tag as MicrometerTag

fun MetricsCategory.toValidMicrometerName(): String {
  return this.name.lowercase().replace('_', '.')
}

fun Tag.requireValidMicrometerName() {
  this.key.requireValidMicrometerName()
}

fun String.requireValidMicrometerName() {
  require(this.lowercase().trim() == this && this.all { it.isLetterOrDigit() || it == '.' }) {
    "$this must adhere to Micrometer naming convention!"
  }
}

fun Tag.toMicrometerTags(): MicrometerTag {
  return MicrometerTag.of(this.key, this.value)
}

fun List<Tag>.toMicrometerTags(): Iterable<MicrometerTag> {
  return this.map { MicrometerTag.of(it.key, it.value) }
}

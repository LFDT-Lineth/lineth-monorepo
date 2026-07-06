package linea.domain.serialization

import com.fasterxml.jackson.core.JsonGenerator
import com.fasterxml.jackson.core.JsonParser
import com.fasterxml.jackson.databind.DeserializationContext
import com.fasterxml.jackson.databind.JsonDeserializer
import com.fasterxml.jackson.databind.JsonSerializer
import com.fasterxml.jackson.databind.SerializerProvider
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import kotlin.time.Instant

/** Serializes `ByteArray` as `0x`-prefixed hex. */
class ByteArrayHexSerializer : JsonSerializer<ByteArray>() {
  override fun serialize(value: ByteArray, gen: JsonGenerator, serializers: SerializerProvider) {
    gen.writeString(value.encodeHex())
  }
}

/** Deserializes a `0x`-prefixed hex string into `ByteArray`. */
class ByteArrayHexDeserializer : JsonDeserializer<ByteArray>() {
  override fun deserialize(p: JsonParser, ctxt: DeserializationContext): ByteArray = p.valueAsString.decodeHex()
}

/** Serializes `kotlin.time.Instant` as epoch milliseconds (a JSON number). */
class InstantEpochMilliSerializer : JsonSerializer<Instant>() {
  override fun serialize(value: Instant, gen: JsonGenerator, serializers: SerializerProvider) {
    gen.writeNumber(value.toEpochMilliseconds())
  }
}

/** Deserializes an epoch-milliseconds JSON number into `kotlin.time.Instant`. */
class InstantEpochMilliDeserializer : JsonDeserializer<Instant>() {
  override fun deserialize(p: JsonParser, ctxt: DeserializationContext): Instant =
    Instant.fromEpochMilliseconds(p.longValue)
}

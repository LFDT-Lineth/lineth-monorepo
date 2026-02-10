package build.linea.s11n.jackson

import com.fasterxml.jackson.core.JsonGenerator
import com.fasterxml.jackson.core.JsonParser
import com.fasterxml.jackson.databind.DeserializationContext
import com.fasterxml.jackson.databind.JsonDeserializer
import com.fasterxml.jackson.databind.JsonSerializer
import com.fasterxml.jackson.databind.SerializerProvider
import kotlin.time.Instant

object InstantAsHexNumberSerializer : JsonSerializer<Instant>() {
  override fun serialize(value: Instant, gen: JsonGenerator, serializers: SerializerProvider) {
    gen.writeString("0x${value.epochSeconds.toString(16)}")
  }
}

object InstantAsHexNumberDeserializer : JsonDeserializer<Instant>() {
  override fun deserialize(p: JsonParser, ctxt: DeserializationContext): Instant {
    return Instant.fromEpochSeconds(p.text.replace("0x", "").toLong(16))
  }
}

package linea.s11n.jackson

import com.fasterxml.jackson.core.JsonGenerator
import com.fasterxml.jackson.databind.JsonSerializer
import com.fasterxml.jackson.databind.SerializerProvider
import java.math.BigInteger

internal fun Number.toHex(): String = "0x" + BigInteger(toString()).toString(16)
internal fun ULong.toHex(): String = "0x" + BigInteger(toString()).toString(16)

object IntToHexSerializer : JsonSerializer<Int>() {
  override fun serialize(value: Int, gen: JsonGenerator, serializers: SerializerProvider) {
    gen.writeString(value.toHex())
  }
}

@Suppress("PLATFORM_CLASS_MAPPED_TO_KOTLIN")
object JIntegerToHexSerializer : JsonSerializer<Integer>() {
  override fun serialize(value: Integer, gen: JsonGenerator, serializers: SerializerProvider) {
    gen.writeString(value.toHex())
  }
}

object LongToHexSerializer : JsonSerializer<Long>() {
  override fun serialize(value: Long, gen: JsonGenerator, serializers: SerializerProvider) {
    gen.writeString(value.toHex())
  }
}

@Suppress("PLATFORM_CLASS_MAPPED_TO_KOTLIN")
object JLongToHexSerializer : JsonSerializer<java.lang.Long>() {
  override fun serialize(value: java.lang.Long, gen: JsonGenerator, serializers: SerializerProvider) {
    gen.writeString(value.toHex())
  }
}

object ULongToHexSerializer : JsonSerializer<ULong>() {
  override fun serialize(value: ULong, gen: JsonGenerator, serializers: SerializerProvider) {
    gen.writeString(value.toHex())
  }
}

object BigIntegerToHexSerializer : JsonSerializer<BigInteger>() {
  override fun serialize(value: BigInteger, gen: JsonGenerator, serializers: SerializerProvider) {
    gen.writeString(value.toHex())
  }
}

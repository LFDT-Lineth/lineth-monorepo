package linea.s11n.jackson

import com.fasterxml.jackson.core.JsonGenerator
import com.fasterxml.jackson.core.JsonParser
import com.fasterxml.jackson.core.JsonToken
import com.fasterxml.jackson.databind.DeserializationContext
import com.fasterxml.jackson.databind.JsonDeserializer
import com.fasterxml.jackson.databind.JsonSerializer
import com.fasterxml.jackson.databind.SerializerProvider
import linea.kotlin.toBigIntegerFromHex
import linea.kotlin.toULongFromHex
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

object ULongFromHexDeserializer : JsonDeserializer<ULong>() {
  override fun deserialize(parser: JsonParser, context: DeserializationContext): ULong =
    hexQuantity(parser, context, ULong::class.java).toULongFromHex()
}

object BigIntegerFromHexDeserializer : JsonDeserializer<BigInteger>() {
  override fun deserialize(parser: JsonParser, context: DeserializationContext): BigInteger =
    hexQuantity(parser, context, BigInteger::class.java).toBigIntegerFromHex()
}

private fun hexQuantity(parser: JsonParser, context: DeserializationContext, type: Class<*>): String {
  val text = parser.text
  if (parser.currentToken != JsonToken.VALUE_STRING || !text.startsWith("0x")) {
    throw context.weirdStringException(text, type, "expected a 0x-prefixed hex string")
  }
  return text
}

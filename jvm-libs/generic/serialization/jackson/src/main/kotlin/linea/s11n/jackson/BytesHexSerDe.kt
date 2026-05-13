package linea.s11n.jackson

import com.fasterxml.jackson.core.JsonGenerator
import com.fasterxml.jackson.core.JsonParser
import com.fasterxml.jackson.databind.DeserializationContext
import com.fasterxml.jackson.databind.JsonDeserializer
import com.fasterxml.jackson.databind.JsonSerializer
import com.fasterxml.jackson.databind.SerializerProvider
import java.util.HexFormat

private val hexFormatter = HexFormat.of()

object ByteArrayToHexSerializer : JsonSerializer<ByteArray>() {
  override fun serialize(value: ByteArray, gen: JsonGenerator, serializers: SerializerProvider?) {
    gen.writeString("0x" + hexFormatter.formatHex(value))
  }

  override fun handledType(): Class<ByteArray> {
    return ByteArray::class.java
  }
}

object ByteToHexSerializer : JsonSerializer<Byte>() {
  override fun serialize(value: Byte, gen: JsonGenerator, serializers: SerializerProvider?) {
    gen.writeString("0x" + hexFormatter.toHexDigits(value))
  }
}

object UByteToHexSerializer : JsonSerializer<UByte>() {
  override fun serialize(value: UByte, gen: JsonGenerator, serializers: SerializerProvider?) {
    gen.writeString("0x" + hexFormatter.toHexDigits(value.toByte()))
  }
}

object ByteArrayToHexDeserializer : JsonDeserializer<ByteArray>() {
  override fun deserialize(parser: JsonParser, contex: DeserializationContext): ByteArray {
    return hexFormatter.parseHex(parser.text.removePrefix("0x"))
  }
}

object ByteToHexDeserializer : JsonDeserializer<Byte>() {
  override fun deserialize(parser: JsonParser, contex: DeserializationContext): Byte {
    return hexFormatter.parseHex(parser.text.removePrefix("0x"))[0]
  }
}

object UByteToHexDeserializer : JsonDeserializer<UByte>() {
  override fun deserialize(parser: JsonParser, contex: DeserializationContext): UByte {
    return hexFormatter.parseHex(parser.text.removePrefix("0x"))[0].toUByte()
  }
}

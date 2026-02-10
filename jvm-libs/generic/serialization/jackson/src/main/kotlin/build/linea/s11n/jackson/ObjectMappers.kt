package build.linea.s11n.jackson

import com.fasterxml.jackson.databind.ObjectMapper
import com.fasterxml.jackson.databind.module.SimpleModule
import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import java.math.BigInteger
import kotlin.time.Instant

@Suppress("PLATFORM_CLASS_MAPPED_TO_KOTLIN")
val ethNumberAsHexSerialisersModule = SimpleModule().apply {
  this.addSerializer(Instant::class.java, InstantISO8601Serializer)
  this.addDeserializer(Instant::class.java, InstantISO8601Deserializer)
  this.addSerializer(Int::class.java, IntToHexSerializer)
  this.addSerializer(Integer::class.java, JIntegerToHexSerializer)
  this.addSerializer(Long::class.java, LongToHexSerializer)
  this.addSerializer(java.lang.Long::class.java, JLongToHexSerializer)
  this.addSerializer(ULong::class.java, ULongToHexSerializer)
  this.addSerializer(BigInteger::class.java, BigIntegerToHexSerializer)
}

val ethByteAsHexSerialisersModule = SimpleModule().apply {
  this.addSerializer(Byte::class.java, ByteToHexSerializer)
  this.addSerializer(UByte::class.java, UByteToHexSerializer)
  this.addSerializer(ByteArray::class.java, ByteArrayToHexSerializer)
}

val ethByteAsHexDeserialisersModule = SimpleModule().apply {
  this.addDeserializer(Byte::class.java, ByteToHexDeserializer)
  this.addDeserializer(UByte::class.java, UByteToHexDeserializer)
  this.addDeserializer(ByteArray::class.java, ByteArrayToHexDeserializer)
}

val ethApiObjectMapper: ObjectMapper = jacksonObjectMapper()
  .registerModules(
    ethNumberAsHexSerialisersModule,
    ethByteAsHexSerialisersModule,
  )

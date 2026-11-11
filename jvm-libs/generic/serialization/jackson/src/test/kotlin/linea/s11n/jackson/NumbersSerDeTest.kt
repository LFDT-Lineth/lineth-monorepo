package linea.s11n.jackson

import com.fasterxml.jackson.annotation.JsonProperty
import com.fasterxml.jackson.databind.JsonMappingException
import com.fasterxml.jackson.databind.ObjectMapper
import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import net.javacrumbs.jsonunit.assertj.JsonAssertions.assertThatJson
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import java.math.BigInteger

class NumbersSerDeTest {
  private lateinit var objectMapper: ObjectMapper

  @BeforeEach
  fun setUp() {
    objectMapper = ethApiObjectMapper
  }

  @Test
  fun numbersSerialization() {
    data class SomeObject(
      // Int
      val intNull: Int? = null,
      val intZero: Int = 0,
      val intMaxValue: Int = Int.MAX_VALUE,
      val intSomeValue: Int = 0xff00aa,

      // Long
      val longNull: Long? = null,
      val longZero: Long = 0,
      val longMaxValue: Long = Long.MAX_VALUE,
      val longSomeValue: Long = 0xff00aa,

      // ULong
      // jackson has a bug and serializes any cAmelCase as camelCase, need to set it with @JsonProperty
      // it's only on 1st character, so we can't use uLongNull, uLongZero, without annotation etc.
      @get:JsonProperty("uLongNull") val uLongNull: ULong? = null,
      @get:JsonProperty("uLongZero") val uLongZero: ULong = 0uL,
      @get:JsonProperty("uLongMaxValue") val uLongMaxValue: ULong = ULong.MAX_VALUE,
      @get:JsonProperty("uLongSomeValue") val uLongSomeValue: ULong = 0xff00aa.toULong(),

      // BigInteger
      val bigIntegerNull: BigInteger? = null,
      val bigIntegerZero: BigInteger = BigInteger.ZERO,
      val bigIntegerSomeValue: BigInteger = BigInteger.valueOf(0xff00aaL),

      // nested Structures
      val listOfInts: List<Int> = listOf(1, 10),
      val listOfLongs: List<Long> = listOf(1, 10),
      val listOfULongs: List<ULong> = listOf(1UL, 10UL),
      val listOfBigIntegers: List<BigInteger> = listOf(1L, 10L).map(BigInteger::valueOf),
    )

    val json = objectMapper.writeValueAsString(SomeObject())
    assertThatJson(json).isEqualTo(
      """
      {
          "intNull": null,
          "intZero": "0x0",
          "intMaxValue": "0x7fffffff",
          "intSomeValue": "0xff00aa",
          "longNull": null,
          "longZero": "0x0",
          "longMaxValue": "0x7fffffffffffffff",
          "longSomeValue": "0xff00aa",
          "uLongNull": null,
          "uLongZero": "0x0",
          "uLongMaxValue": "0xffffffffffffffff",
          "uLongSomeValue": "0xff00aa",
          "bigIntegerNull": null,
          "bigIntegerZero": "0x0",
          "bigIntegerSomeValue": "0xff00aa",
          "listOfInts": ["0x1", "0xa"],
          "listOfLongs": ["0x1", "0xa"],
          "listOfULongs": ["0x1", "0xa"],
          "listOfBigIntegers": ["0x1", "0xa"]
      }
      """.trimIndent(),
    )
  }

  @Test
  fun numbersDeserializationFromHex() {
    class SomeObject(
      val uLongZero: ULong,
      val uLongMaxValue: ULong,
      val uLongNull: ULong?,
      val bigInteger: BigInteger,
    )
    val mapper = jacksonObjectMapper().registerModules(ethNumberAsHexDeserialisersModule)

    val result = mapper.readValue(
      """
        {"uLongZero":"0x0","uLongMaxValue":"0xffffffffffffffff","uLongNull":null,"bigInteger":"0x1ffffffffffffffff"}
      """,
      SomeObject::class.java,
    )

    assertThat(result.uLongZero).isEqualTo(0UL)
    assertThat(result.uLongMaxValue).isEqualTo(ULong.MAX_VALUE)
    assertThat(result.uLongNull).isNull()
    assertThat(result.bigInteger).isEqualTo(BigInteger("1ffffffffffffffff", 16))
  }

  @Test
  fun numbersDeserializationRejectsNonHexQuantities() {
    class SomeObject(val value: ULong)
    val mapper = jacksonObjectMapper().registerModules(ethNumberAsHexDeserialisersModule)

    listOf("""{"value":100}""", """{"value":"100"}""", """{"value":"0xzz"}""").forEach { json ->
      assertThatThrownBy { mapper.readValue(json, SomeObject::class.java) }
        .describedAs(json)
        .isInstanceOf(JsonMappingException::class.java)
    }
  }
}

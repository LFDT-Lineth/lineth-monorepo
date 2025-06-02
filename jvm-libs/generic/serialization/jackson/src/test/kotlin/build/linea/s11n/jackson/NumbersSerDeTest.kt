package build.linea.s11n.jackson

import com.fasterxml.jackson.annotation.JsonProperty
import com.fasterxml.jackson.databind.ObjectMapper
import net.javacrumbs.jsonunit.assertj.JsonAssertions.assertThatJson
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
}

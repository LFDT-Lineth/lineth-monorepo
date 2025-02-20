package build.linea.s11n.jackson

import com.fasterxml.jackson.databind.ObjectMapper
import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import com.fasterxml.jackson.module.kotlin.readValue
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import net.javacrumbs.jsonunit.assertj.JsonAssertions.assertThatJson
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test

class BytesSerDeTest {
  private lateinit var objectMapper: ObjectMapper

  private val jsonObj = """
    {
        "nullBytes": null,
        "emptyBytes": "0x",
        "someBytes": "0x01aaff04",
        "listOfByteArray": ["0x01aaff04", "0x01aaff05"],
        "nullUByte": null,
        "someUByte": "0xaf",
        "minUByte": "0x00",
        "maxUByte": "0xff",
        "nullByte": null,
        "someByte": "0xaf",
        "minByte": "0x80",
        "maxByte": "0x7f"
    }
  """.trimIndent()
  private val objWithBytesFields = SomeObject(
    // ByteArray
    nullBytes = null,
    emptyBytes = byteArrayOf(),
    someBytes = "0x01aaff04".decodeHex(),
    listOfByteArray = listOf("0x01aaff04", "0x01aaff05").map { it.decodeHex() },

    // UByte
    nullUByte = null,
    someUByte = 0xaf.toUByte(),
    minUByte = UByte.MIN_VALUE,
    maxUByte = UByte.MAX_VALUE,

    // Byte
    nullByte = null,
    someByte = 0xaf.toByte(),
    minByte = Byte.MIN_VALUE,
    maxByte = Byte.MAX_VALUE
  )

  @BeforeEach
  fun setUp() {
    objectMapper = jacksonObjectMapper()
      .registerModules(ethByteAsHexSerialisersModule)
      .registerModules(ethByteAsHexDeserialisersModule)
  }

  @Test
  fun bytesSerDe() {
    assertThatJson(objectMapper.writeValueAsString(objWithBytesFields)).isEqualTo(jsonObj)
    assertThat(objectMapper.readValue<SomeObject>(jsonObj)).isEqualTo(objWithBytesFields)
  }

  private data class SomeObject(
    // ByteArray
    val nullBytes: ByteArray?,
    val emptyBytes: ByteArray,
    val someBytes: ByteArray,
    val listOfByteArray: List<ByteArray>,

    // UByte
    val nullUByte: UByte?,
    val someUByte: UByte,
    val minUByte: UByte,
    val maxUByte: UByte,
    // Byte
    val nullByte: Byte?,
    val someByte: Byte,
    val minByte: Byte,
    val maxByte: Byte
  ) {
    override fun equals(other: Any?): Boolean {
      if (this === other) return true
      if (javaClass != other?.javaClass) return false

      other as SomeObject

      if (nullBytes != null) {
        if (other.nullBytes == null) return false
        if (!nullBytes.contentEquals(other.nullBytes)) return false
      } else if (other.nullBytes != null) return false
      if (!emptyBytes.contentEquals(other.emptyBytes)) return false
      if (!someBytes.contentEquals(other.someBytes)) return false
      if (!contentEquals(listOfByteArray, other.listOfByteArray)) return false
      if (nullUByte != other.nullUByte) return false
      if (someUByte != other.someUByte) return false
      if (minUByte != other.minUByte) return false
      if (maxUByte != other.maxUByte) return false
      if (nullByte != other.nullByte) return false
      if (someByte != other.someByte) return false
      if (minByte != other.minByte) return false
      if (maxByte != other.maxByte) return false

      return true
    }

    override fun hashCode(): Int {
      var result = nullBytes?.contentHashCode() ?: 0
      result = 31 * result + emptyBytes.contentHashCode()
      result = 31 * result + someBytes.contentHashCode()
      result = 31 * result + listOfByteArray.hashCode()
      result = 31 * result + (nullUByte?.hashCode() ?: 0)
      result = 31 * result + someUByte.hashCode()
      result = 31 * result + minUByte.hashCode()
      result = 31 * result + maxUByte.hashCode()
      result = 31 * result + (nullByte ?: 0)
      result = 31 * result + someByte
      result = 31 * result + minByte
      result = 31 * result + maxByte
      return result
    }

    override fun toString(): String {
      return "SomeObject(" +
        "nullBytes=${nullBytes?.contentToString()}, " +
        "emptyBytes=${emptyBytes.contentToString()}, " +
        "someByte=${someBytes.contentToString()}, " +
        "listOfByteArray=${listOfByteArray.joinToString(",", "[", "]") { it.encodeHex() }}, " +
        "nullUByte=$nullUByte, " +
        "someUByte=$someUByte, " +
        "minUByte=$minUByte, " +
        "maxUByte=$maxUByte, " +
        "nullByte=$nullByte, " +
        "someByte=$someByte, " +
        "minByte=$minByte, " +
        "maxByte=$maxByte" +
        ")"
    }
  }

  companion object {
    fun contentEquals(list1: List<ByteArray>, list2: List<ByteArray>): Boolean {
      if (list1.size != list2.size) return false

      return list1.zip(list2).all { (arr1, arr2) -> arr1.contentEquals(arr2) }
    }
  }
}

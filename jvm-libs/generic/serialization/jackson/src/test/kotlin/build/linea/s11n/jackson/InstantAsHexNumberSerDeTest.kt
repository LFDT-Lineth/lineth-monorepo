package build.linea.s11n.jackson

import com.fasterxml.jackson.databind.ObjectMapper
import com.fasterxml.jackson.databind.module.SimpleModule
import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import net.javacrumbs.jsonunit.assertj.JsonAssertions.assertThatJson
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import kotlin.time.Instant

class InstantAsHexNumberSerDeTest {
  private lateinit var objectMapper: ObjectMapper

  @BeforeEach
  fun setUp() {
    objectMapper = jacksonObjectMapper()
      .registerModules(
        SimpleModule().apply {
          this.addSerializer(Instant::class.java, InstantAsHexNumberSerializer)
          this.addDeserializer(Instant::class.java, InstantAsHexNumberDeserializer)
        },
      )
  }

  @Test
  fun instantSerDeDeSerialization() {
    data class SomeObject(
      // Int
      val instantNull: Instant? = null,
      // 2021-01-02T09:00:45Z UTC,
      val instantUTC: Instant = Instant.fromEpochSeconds(1609578045),
      // 2021-07-01T08:00:45Z UTC
      val instantUTCDST: Instant = Instant.fromEpochSeconds(1625126445),
      // 2021-01-02T09:00:45+01:30 UTC+01:30,
      val instantUTCPlus: Instant = Instant.fromEpochSeconds(1609572645),
      // 2021-01-02T09:00:45-01:30" UTC-01:30
      val instantUTCMinus: Instant = Instant.fromEpochSeconds(1609583445),
    )

    val expectedJson = """
      {
        "instantNull": null,
        "instantUTC": "0x5ff0363d",
        "instantUTCDST": "0x60dd762d",
        "instantUTCPlus": "0x5ff02125",
        "instantUTCMinus": "0x5ff04b55"
      }
    """.trimIndent()

    // assert serialization
    assertThatJson(objectMapper.writeValueAsString(SomeObject())).isEqualTo(expectedJson)

    // assert deserialization
    assertThatJson(objectMapper.readValue(expectedJson, SomeObject::class.java)).isEqualTo(SomeObject())
  }
}

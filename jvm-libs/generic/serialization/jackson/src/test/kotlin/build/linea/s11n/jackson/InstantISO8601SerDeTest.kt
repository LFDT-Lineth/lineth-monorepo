package build.linea.s11n.jackson

import com.fasterxml.jackson.databind.ObjectMapper
import net.javacrumbs.jsonunit.assertj.JsonAssertions.assertThatJson
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import kotlin.time.Instant

class InstantISO8601SerDeTest {
  private lateinit var objectMapper: ObjectMapper

  @BeforeEach
  fun setUp() {
    objectMapper = ethApiObjectMapper
  }

  @Test
  fun instantSerialization() {
    data class SomeObject(
      // Int
      val instantNull: Instant? = null,
      val instantUTC: Instant = Instant.parse("2021-01-02T09:00:45Z"),
      val instantUTCPlus: Instant = Instant.parse("2021-01-02T09:00:45+01:30"),
      val instantUTCMinus: Instant = Instant.parse("2021-01-02T09:00:45-01:30"),
    )

    val json = objectMapper.writeValueAsString(SomeObject())
    assertThatJson(json).isEqualTo(
      """
      {
        "instantNull": null,
        "instantUTC": "2021-01-02T09:00:45Z",
        "instantUTCPlus": "2021-01-02T07:30:45Z",
        "instantUTCMinus": "2021-01-02T10:30:45Z"
      }
      """.trimIndent(),
    )
  }

  @Test
  fun instantSerDeDeSerialization() {
    data class SomeObject(
      // Int
      val instantNull: Instant? = null,
      val instantUTC: Instant = Instant.parse("2021-01-02T09:00:45Z"),
      val instantUTCPlus: Instant = Instant.parse("2021-01-02T09:00:45+01:30"),
      val instantUTCMinus: Instant = Instant.parse("2021-01-02T09:00:45-01:30"),
    )

    val expectedJson = """
      {
        "instantNull": null,
        "instantUTC": "2021-01-02T09:00:45Z",
        "instantUTCPlus": "2021-01-02T07:30:45Z",
        "instantUTCMinus": "2021-01-02T10:30:45Z"
      }
    """.trimIndent()

    // assert serialization
    assertThatJson(objectMapper.writeValueAsString(SomeObject())).isEqualTo(expectedJson)

    // assert deserialization
    assertThatJson(objectMapper.readValue(expectedJson, SomeObject::class.java)).isEqualTo(SomeObject())
  }
}

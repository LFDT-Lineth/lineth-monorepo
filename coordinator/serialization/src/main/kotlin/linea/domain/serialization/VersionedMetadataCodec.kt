package linea.domain.serialization

import com.fasterxml.jackson.databind.DeserializationFeature
import com.fasterxml.jackson.databind.JsonNode
import com.fasterxml.jackson.databind.ObjectMapper
import com.fasterxml.jackson.databind.module.SimpleModule
import com.fasterxml.jackson.module.kotlin.jacksonMapperBuilder
import linea.domain.BlobMetadataPayload
import linea.domain.BlobMetadataV1
import linea.domain.BlobMetadataVersion
import linea.domain.VersionedMetadata
import linea.domain.VersionedMetadataPayload
import linea.s11n.jackson.InstantISO8601Deserializer
import linea.s11n.jackson.InstantISO8601Serializer
import linea.s11n.jackson.ethByteAsHexDeserialisersModule
import linea.s11n.jackson.ethByteAsHexSerialisersModule
import kotlin.time.Instant

/**
 * Serializes a [VersionedMetadata] envelope as `{ "version": <int>, "metadata": { ... } }`.
 * On read, `version` selects the concrete payload class; the payload itself carries no version
 * field. Reusable for any versioned-JSON column (blob submission metadata today).
 */
class VersionedMetadataCodec<T : VersionedMetadataPayload>(
  private val mapper: ObjectMapper,
  private val versionToClass: (Int) -> Class<out T>,
  private val versionOf: (T) -> Int,
) {
  fun toJson(metadata: T): String {
    val node = mapper.createObjectNode()
    node.put("version", versionOf(metadata))
    node.set<JsonNode>("metadata", mapper.valueToTree(metadata))
    return mapper.writeValueAsString(node)
  }

  fun fromJson(json: String): VersionedMetadata<T> {
    val root = mapper.readTree(json)
    val version = root.get("version").asInt()
    val payload = mapper.treeToValue(root.get("metadata"), versionToClass(version))
    return VersionedMetadata(version, payload)
  }

  companion object {
    /**
     * An [ObjectMapper] that (de)serializes `ByteArray` as `0x`-hex and `kotlin.time.Instant`
     * as ISO-8601, reusing the shared serializers in `jvm-libs:generic:serialization:jackson`,
     * so the annotation-free domain payloads serialize correctly.
     */
    fun versionedMetadataMapper(): ObjectMapper =
      jacksonMapperBuilder()
        .configure(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES, false)
        .addModule(ethByteAsHexSerialisersModule)
        .addModule(ethByteAsHexDeserialisersModule)
        .addModule(
          SimpleModule().apply {
            addSerializer(Instant::class.java, InstantISO8601Serializer)
            addDeserializer(Instant::class.java, InstantISO8601Deserializer)
          },
        )
        .build()
  }
}

/** Codec for [linea.domain.BlobMetadata]. */
object BlobMetadataSerialization {
  val codec: VersionedMetadataCodec<BlobMetadataPayload> =
    VersionedMetadataCodec(
      mapper = VersionedMetadataCodec.versionedMetadataMapper(),
      versionToClass = { BlobMetadataVersion.fromValue(it).payloadClass },
      versionOf = { payload ->
        when (payload) {
          is BlobMetadataV1 -> BlobMetadataVersion.V1.value
        }
      },
    )
}

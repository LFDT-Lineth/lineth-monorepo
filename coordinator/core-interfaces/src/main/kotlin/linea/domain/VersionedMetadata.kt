package linea.domain

/**
 * Marker for a versioned metadata payload persisted as JSON. Each concrete payload type maps
 * 1:1 to a [VersionedMetadata.version]. Reusable across columns (blob submission metadata
 * today; aggregation metadata etc. later).
 */
interface VersionedMetadataPayload

/**
 * A versioned JSON envelope: `{ "version": <int>, "metadata": { ... } }`. `version` is the
 * only field common to every version; `metadata` is the version-specific payload. Serialized
 * by `VersionedMetadataCodec` (in the serialization module).
 */
data class VersionedMetadata<T : VersionedMetadataPayload>(
  val version: Int,
  val metadata: T,
)

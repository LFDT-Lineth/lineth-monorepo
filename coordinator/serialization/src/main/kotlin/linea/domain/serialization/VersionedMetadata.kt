package linea.domain.serialization

/**
 * Marker for a versioned metadata payload persisted as serialized object. Each concrete payload type maps
 * 1:1 to a [VersionedMetadata.version]
 */
interface VersionedMetadataPayload

/**
 * The stored serialized envelope. `version` is the only
 * field common to every version; `metadata` is the version-specific payload. Reusable for any
 * versioned-serialized column.
 */
data class VersionedMetadata<T : VersionedMetadataPayload>(
  val version: Int,
  val metadata: T,
)

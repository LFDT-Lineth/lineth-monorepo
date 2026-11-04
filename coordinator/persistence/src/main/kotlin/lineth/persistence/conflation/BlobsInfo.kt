package lineth.persistence.conflation

import io.vertx.core.json.JsonArray
import io.vertx.core.json.JsonObject
import linea.domain.BlobData
import linea.domain.BlobRecordV2
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex

data class BlobsInfo(
  val parentDataRollingHash: ByteArray,
  val dataRollingHash: ByteArray,
  val parentDataTailTakeBytes: Int,
  val boundaryPrevDataRollingHash: ByteArray?,
  val finalDataTailDiscardBytes: Int,
  val blobsData: List<BlobData>,
) {
  fun toJsonString(): String =
    JsonObject()
      .put("parentDataRollingHash", parentDataRollingHash.encodeHex())
      .put("dataRollingHash", dataRollingHash.encodeHex())
      .put("parentDataTailTakeBytes", parentDataTailTakeBytes)
      .put("boundaryPrevDataRollingHash", boundaryPrevDataRollingHash?.encodeHex())
      .put("finalDataTailDiscardBytes", finalDataTailDiscardBytes)
      .put(
        "blobsData",
        JsonArray(
          blobsData.map { blobData ->
            JsonObject()
              .put("chunkHash", blobData.chunkHash.encodeHex())
              .put("blobBytes", blobData.blobBytes.encodeHex())
              .put("batchesCount", blobData.batchesCount.toInt())
              .put("prevDataRollingHash", blobData.prevDataRollingHash.encodeHex())
              .put("dataRollingHash", blobData.dataRollingHash.encodeHex())
          },
        ),
      )
      .encode()

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false
    other as BlobsInfo
    if (!parentDataRollingHash.contentEquals(other.parentDataRollingHash)) return false
    if (!dataRollingHash.contentEquals(other.dataRollingHash)) return false
    if (parentDataTailTakeBytes != other.parentDataTailTakeBytes) return false
    if (!boundaryPrevDataRollingHash.contentEquals(other.boundaryPrevDataRollingHash)) return false
    if (finalDataTailDiscardBytes != other.finalDataTailDiscardBytes) return false
    if (blobsData != other.blobsData) return false
    return true
  }

  override fun hashCode(): Int {
    var result = parentDataRollingHash.contentHashCode()
    result = 31 * result + dataRollingHash.contentHashCode()
    result = 31 * result + parentDataTailTakeBytes
    result = 31 * result + (boundaryPrevDataRollingHash?.contentHashCode() ?: 0)
    result = 31 * result + finalDataTailDiscardBytes
    result = 31 * result + blobsData.hashCode()
    return result
  }

  companion object {
    fun fromJsonString(jsonString: String): BlobsInfo {
      val json = JsonObject(jsonString)
      return BlobsInfo(
        parentDataRollingHash = json.requiredString("parentDataRollingHash").decodeHex(),
        dataRollingHash = json.requiredString("dataRollingHash").decodeHex(),
        parentDataTailTakeBytes = json.requiredInteger("parentDataTailTakeBytes"),
        boundaryPrevDataRollingHash = json.getString("boundaryPrevDataRollingHash")?.decodeHex(),
        finalDataTailDiscardBytes = json.requiredInteger("finalDataTailDiscardBytes"),
        blobsData = (json.getJsonArray("blobsData") ?: error("missing blobsData")).map { item ->
          val blobDataJson = item as JsonObject
          BlobData(
            chunkHash = blobDataJson.requiredString("chunkHash").decodeHex(),
            blobBytes = blobDataJson.requiredString("blobBytes").decodeHex(),
            batchesCount = blobDataJson.requiredInteger("batchesCount").toUInt(),
            prevDataRollingHash = blobDataJson.requiredString("prevDataRollingHash").decodeHex(),
            dataRollingHash = blobDataJson.requiredString("dataRollingHash").decodeHex(),
          )
        },
      )
    }

    fun fromDomainObject(record: BlobRecordV2): BlobsInfo =
      BlobsInfo(
        parentDataRollingHash = record.parentDataRollingHash,
        dataRollingHash = record.dataRollingHash,
        parentDataTailTakeBytes = record.parentDataTailTakeBytes,
        boundaryPrevDataRollingHash = record.boundaryPrevDataRollingHash,
        finalDataTailDiscardBytes = record.finalDataTailDiscardBytes,
        blobsData = record.blobsData,
      )

    private fun JsonObject.requiredString(key: String): String = getString(key) ?: error("missing $key")

    private fun JsonObject.requiredInteger(key: String): Int = getInteger(key) ?: error("missing $key")
  }
}

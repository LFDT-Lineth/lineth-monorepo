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
  val endOffset: Int,
  val blobsData: List<BlobData>,
) {
  fun toJsonString(): String =
    JsonObject()
      .put("parentDataRollingHash", parentDataRollingHash.encodeHex())
      .put("dataRollingHash", dataRollingHash.encodeHex())
      .put("endOffset", endOffset)
      .put(
        "blobsData",
        JsonArray(
          blobsData.map { blobData ->
            JsonObject()
              .put("chunkHash", blobData.chunkHash.encodeHex())
              .put("blobBytes", blobData.blobBytes.encodeHex())
              .put("conflationsCount", blobData.conflationsCount.toInt())
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
    if (endOffset != other.endOffset) return false
    if (blobsData != other.blobsData) return false
    return true
  }

  override fun hashCode(): Int {
    var result = parentDataRollingHash.contentHashCode()
    result = 31 * result + dataRollingHash.contentHashCode()
    result = 31 * result + endOffset
    result = 31 * result + blobsData.hashCode()
    return result
  }

  companion object {
    fun fromJsonString(jsonString: String): BlobsInfo {
      val json = JsonObject(jsonString)
      return BlobsInfo(
        parentDataRollingHash = json.getString("parentDataRollingHash").decodeHex(),
        dataRollingHash = json.getString("dataRollingHash").decodeHex(),
        endOffset = json.getInteger("endOffset"),
        blobsData = json.getJsonArray("blobsData").map { item ->
          val blobDataJson = item as JsonObject
          BlobData(
            chunkHash = blobDataJson.getString("chunkHash").decodeHex(),
            blobBytes = blobDataJson.getString("blobBytes").decodeHex(),
            conflationsCount = blobDataJson.getInteger("conflationsCount").toUInt(),
          )
        },
      )
    }

    fun fromDomainObject(record: BlobRecordV2): BlobsInfo =
      BlobsInfo(
        parentDataRollingHash = record.parentDataRollingHash,
        dataRollingHash = record.dataRollingHash,
        endOffset = record.endOffset,
        blobsData = record.blobsData,
      )
  }
}

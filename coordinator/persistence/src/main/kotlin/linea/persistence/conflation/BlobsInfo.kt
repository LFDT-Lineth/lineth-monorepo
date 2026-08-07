package linea.persistence.conflation

import io.vertx.core.json.JsonArray
import io.vertx.core.json.JsonObject
import lineth.domain.BlobData
import lineth.domain.BlobRecordV2
import lineth.kotlin.decodeHex
import lineth.kotlin.encodeHex

data class BlobsInfo(
  val parentShnarf: ByteArray,
  val endShnarf: ByteArray,
  val blobsData: List<BlobData>,
) {
  fun toJsonString(): String =
    JsonObject()
      .put("parentShnarf", parentShnarf.encodeHex())
      .put("endShnarf", endShnarf.encodeHex())
      .put(
        "blobsData",
        JsonArray(
          blobsData.map { blobData ->
            JsonObject()
              .put("blobHash", blobData.blobHash.encodeHex())
              .put("compressedData", blobData.compressedData.encodeHex())
              .put("batchesCount", blobData.batchesCount.toInt())
          },
        ),
      )
      .encode()

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false
    other as BlobsInfo
    if (!parentShnarf.contentEquals(other.parentShnarf)) return false
    if (!endShnarf.contentEquals(other.endShnarf)) return false
    if (blobsData != other.blobsData) return false
    return true
  }

  override fun hashCode(): Int {
    var result = parentShnarf.contentHashCode()
    result = 31 * result + endShnarf.contentHashCode()
    result = 31 * result + blobsData.hashCode()
    return result
  }

  companion object {
    fun fromJsonString(jsonString: String): BlobsInfo {
      val json = JsonObject(jsonString)
      return BlobsInfo(
        parentShnarf = json.getString("parentShnarf").decodeHex(),
        endShnarf = json.getString("endShnarf").decodeHex(),
        blobsData = json.getJsonArray("blobsData").map { item ->
          val blobDataJson = item as JsonObject
          BlobData(
            blobHash = blobDataJson.getString("blobHash").decodeHex(),
            compressedData = blobDataJson.getString("compressedData").decodeHex(),
            batchesCount = blobDataJson.getInteger("batchesCount").toUInt(),
          )
        },
      )
    }

    fun fromDomainObject(record: BlobRecordV2): BlobsInfo =
      BlobsInfo(
        parentShnarf = record.parentShnarf,
        endShnarf = record.endShnarf,
        blobsData = record.blobsData,
      )
  }
}

package lineth.persistence.conflation

import io.vertx.core.json.JsonObject
import linea.domain.createBlobRecordV2
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test

class BlobsInfoTest {
  @Test
  fun `round-trips a record starting inside a shared blob`() {
    val record = createBlobRecordV2(
      startBlockNumber = 1UL,
      endBlockNumber = 10UL,
      parentDataTailTakeBytes = 100,
      finalDataTailDiscardBytes = 200,
    )
    val info = BlobsInfo.fromDomainObject(record)

    assertThat(BlobsInfo.fromJsonString(info.toJsonString())).isEqualTo(info)
  }

  @Test
  fun `round-trips a fresh-start record without a boundary hash`() {
    val info = BlobsInfo.fromDomainObject(createBlobRecordV2(startBlockNumber = 1UL, endBlockNumber = 10UL))

    val parsed = BlobsInfo.fromJsonString(info.toJsonString())
    assertThat(parsed).isEqualTo(info)
    assertThat(parsed.boundaryPrevDataRollingHash).isNull()
  }

  @Test
  fun `the new fields are required`() {
    val json = JsonObject(
      BlobsInfo.fromDomainObject(createBlobRecordV2(startBlockNumber = 1UL, endBlockNumber = 10UL)).toJsonString(),
    )

    assertThatThrownBy { BlobsInfo.fromJsonString(json.copy().apply { remove("finalDataTailDiscardBytes") }.encode()) }
      .hasMessageContaining("missing finalDataTailDiscardBytes")
    val withoutBlobPrevHash = json.copy().apply {
      getJsonArray("blobsData").getJsonObject(0).remove("prevDataRollingHash")
    }
    assertThatThrownBy { BlobsInfo.fromJsonString(withoutBlobPrevHash.encode()) }
      .hasMessageContaining("missing prevDataRollingHash")
  }
}

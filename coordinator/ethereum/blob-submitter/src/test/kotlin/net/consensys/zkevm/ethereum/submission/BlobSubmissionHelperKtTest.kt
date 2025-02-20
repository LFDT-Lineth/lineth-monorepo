package net.consensys.zkevm.ethereum.submission

import linea.domain.BlockIntervalData
import linea.domain.BlockIntervals
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows

class BlobSubmissionHelperKtTest {
  @Test
  fun chunkBlobs_chunksRegardingAggregationLimitAndChunkSize() {
    val blobs = listOf(
      BlockIntervalData(0UL, 9UL),
      BlockIntervalData(10UL, 19UL),
      BlockIntervalData(20UL, 29UL),

      BlockIntervalData(30UL, 39UL),
      BlockIntervalData(40UL, 49UL),
      BlockIntervalData(50UL, 59UL),

      BlockIntervalData(60UL, 69UL),
      // agg: 0, 69

      BlockIntervalData(70UL, 79UL),
      BlockIntervalData(80UL, 89UL),
      // agg: 70, 89

      // 90+... are discarded because not part of any aggregation, cannot be sent yet
      BlockIntervalData(90UL, 99UL),
      BlockIntervalData(100UL, 109UL),
      BlockIntervalData(110UL, 119UL),
      BlockIntervalData(120UL, 121UL)
    )

    val result = chunkBlobs(
      blobs,
      aggregations = BlockIntervals(startingBlockNumber = 0UL, listOf(69UL, 89UL)),
      targetChunkSize = 3
    )
    assertThat(result).isEqualTo(
      listOf(
        listOf(
          BlockIntervalData(0UL, 9UL),
          BlockIntervalData(10UL, 19UL),
          BlockIntervalData(20UL, 29UL)
        ),
        listOf(
          BlockIntervalData(30UL, 39UL),
          BlockIntervalData(40UL, 49UL),
          BlockIntervalData(50UL, 59UL)
        ),
        listOf(
          BlockIntervalData(60UL, 69UL)
        ),
        listOf(
          BlockIntervalData(70UL, 79UL),
          BlockIntervalData(80UL, 89UL)
        )
      )
    )
  }

  @Test
  fun `chunkBlobs when first blob is after 1 aggregation start block number, returns chucks`() {
    val blobs = listOf(
      BlockIntervalData(10UL, 19UL),
      BlockIntervalData(20UL, 29UL),
      BlockIntervalData(30UL, 39UL),

      BlockIntervalData(40UL, 49UL),
      BlockIntervalData(50UL, 59UL),
      BlockIntervalData(60UL, 69UL),

      BlockIntervalData(70UL, 79UL),
      BlockIntervalData(80UL, 89UL)
    )

    val result = chunkBlobs(
      blobs,
      aggregations = BlockIntervals(startingBlockNumber = 0UL, listOf(300UL, 500UL)),
      targetChunkSize = 3
    )
    assertThat(result).isEqualTo(
      listOf(
        listOf(
          BlockIntervalData(10UL, 19UL),
          BlockIntervalData(20UL, 29UL),
          BlockIntervalData(30UL, 39UL)
        ),
        listOf(
          BlockIntervalData(40UL, 49UL),
          BlockIntervalData(50UL, 59UL),
          BlockIntervalData(60UL, 69UL)
        )
      )
    )
  }

  @Test
  fun `chunkBlobs when blobs match aggregations`() {
    val blobs = listOf(
      BlockIntervalData(0UL, 9UL),
      BlockIntervalData(10UL, 19UL),
      BlockIntervalData(20UL, 29UL),

      BlockIntervalData(30UL, 39UL),
      BlockIntervalData(40UL, 49UL),
      BlockIntervalData(50UL, 59UL),
      // agg: 0, 59

      BlockIntervalData(60UL, 69UL),
      BlockIntervalData(70UL, 79UL),
      BlockIntervalData(80UL, 89UL)
    )

    val result = chunkBlobs(
      blobs,
      aggregations = BlockIntervals(startingBlockNumber = 0UL, listOf(59UL, 89UL)),
      targetChunkSize = 3
    )
    assertThat(result).isEqualTo(
      listOf(
        listOf(
          BlockIntervalData(0UL, 9UL),
          BlockIntervalData(10UL, 19UL),
          BlockIntervalData(20UL, 29UL)
        ),
        listOf(
          BlockIntervalData(30UL, 39UL),
          BlockIntervalData(40UL, 49UL),
          BlockIntervalData(50UL, 59UL)
        ),
        listOf(
          BlockIntervalData(60UL, 69UL),
          BlockIntervalData(70UL, 79UL),
          BlockIntervalData(80UL, 89UL)
        )
      )
    )
  }

  @Test
  fun `chunkBlobs when blobs match aggregation with last chunk size less than target chunk size`() {
    val blobs = listOf(
      BlockIntervalData(0UL, 9UL),
      BlockIntervalData(10UL, 19UL),
      BlockIntervalData(20UL, 29UL),

      BlockIntervalData(30UL, 39UL),
      BlockIntervalData(40UL, 49UL)
    )

    val result = chunkBlobs(
      blobs,
      aggregations = BlockIntervals(startingBlockNumber = 0UL, listOf(49UL)),
      targetChunkSize = 3
    )
    assertThat(result).isEqualTo(
      listOf(
        listOf(
          BlockIntervalData(0UL, 9UL),
          BlockIntervalData(10UL, 19UL),
          BlockIntervalData(20UL, 29UL)
        ),
        listOf(
          BlockIntervalData(30UL, 39UL),
          BlockIntervalData(40UL, 49UL)
        )
      )
    )
  }

  @Test
  fun `chunkBlobs when last aggregations does not have all blobs returns up to max chunks`() {
    val blobs = listOf(
      BlockIntervalData(0UL, 9UL),
      // agg: 0, 9

      BlockIntervalData(10UL, 19UL),
      BlockIntervalData(20UL, 29UL),
      BlockIntervalData(30UL, 39UL),

      BlockIntervalData(40UL, 49UL),
      BlockIntervalData(50UL, 59UL),
      BlockIntervalData(60UL, 69UL),

      BlockIntervalData(70UL, 79UL),
      BlockIntervalData(80UL, 89UL)
    )

    val result = chunkBlobs(
      blobs,
      aggregations = BlockIntervals(startingBlockNumber = 0UL, listOf(9UL, 500UL)),
      targetChunkSize = 3
    )
    assertThat(result).isEqualTo(
      listOf(
        listOf(
          BlockIntervalData(0UL, 9UL)
        ),
        listOf(
          BlockIntervalData(10UL, 19UL),
          BlockIntervalData(20UL, 29UL),
          BlockIntervalData(30UL, 39UL)
        ),
        listOf(
          BlockIntervalData(40UL, 49UL),
          BlockIntervalData(50UL, 59UL),
          BlockIntervalData(60UL, 69UL)
        )
      )
    )
  }

  @Test
  fun `chunkBlobs when last aggregations does not have enough blobs for full chunk returns up previous agg`() {
    val blobs = listOf(
      BlockIntervalData(0UL, 9UL),
      // agg: 0, 9

      BlockIntervalData(10UL, 19UL),
      BlockIntervalData(20UL, 29UL)
    )

    val result = chunkBlobs(
      blobs,
      aggregations = BlockIntervals(startingBlockNumber = 0UL, listOf(9UL, 500UL)),
      targetChunkSize = 3
    )
    assertThat(result).isEqualTo(
      listOf(
        listOf(
          BlockIntervalData(0UL, 9UL)
        )
      )
    )
  }

  @Test
  fun chunkBlobs_aggregationDoesNotMatchBlob() {
    val blobs = listOf(
      BlockIntervalData(0UL, 9UL),
      BlockIntervalData(10UL, 19UL),
      BlockIntervalData(20UL, 29UL)
    )

    assertThrows<IllegalArgumentException> {
      chunkBlobs(
        blobs,
        aggregations = BlockIntervals(startingBlockNumber = 0UL, listOf(15UL, 29UL)),
        targetChunkSize = 3
      )
    }.let { error ->
      assertThat(error).hasMessageMatching("blobs.* are inconsistent with aggregations.*")
    }

    assertThrows<IllegalArgumentException> {
      chunkBlobs(
        blobs,
        aggregations = BlockIntervals(startingBlockNumber = 0UL, listOf(9UL, 21UL)),
        targetChunkSize = 3
      )
    }.let { error ->
      assertThat(error).hasMessageMatching("blobs.* are inconsistent with aggregations.*")
    }
  }
}

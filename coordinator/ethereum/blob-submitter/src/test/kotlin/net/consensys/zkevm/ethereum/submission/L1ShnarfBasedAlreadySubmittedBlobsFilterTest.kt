package net.consensys.zkevm.ethereum.submission

import net.consensys.zkevm.coordinator.clients.smartcontract.LineaRollupSmartContractClient
import net.consensys.zkevm.domain.createBlobRecord
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.mockito.kotlin.any
import org.mockito.kotlin.mock
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.function.Consumer

class L1ShnarfBasedAlreadySubmittedBlobsFilterTest {
  @Test
  fun `filterOutAlreadySubmittedBlobRecords filters out all blobs before highest returned blockNumber`() {
    // previous tick submitted 2 chunks of blobs:
    // 1. b1[10..19], b2[20..29], b3[30..39]
    // 2. b4[40..49], b5[50..59]
    // so, mapping on L1 should be: {b3: 39, b5: 59}, and we should filter out b1, b2, b3, b4, b5
    val blob1 = createBlobRecord(10UL, 19UL)
    val blob2 = createBlobRecord(20UL, 29UL)
    val blob3 = createBlobRecord(30UL, 39UL)
    val blob4 = createBlobRecord(40UL, 49UL)
    val blob5 = createBlobRecord(50UL, 59UL)
    val blob6 = createBlobRecord(60UL, 69UL)
    val blob7 = createBlobRecord(70UL, 79UL)
    val blobs = listOf(blob1, blob2, blob3, blob4, blob5, blob6, blob7)

    val l1SmcClient = mock<LineaRollupSmartContractClient>()
    whenever(l1SmcClient.isBlobShnarfPresent(any(), any()))
      .thenAnswer { invocation ->
        val shnarfQueried = invocation.getArgument<ByteArray>(1)
        val endBlockNumber = when {
          shnarfQueried.contentEquals(blob3.expectedShnarf) -> true
          shnarfQueried.contentEquals(blob5.expectedShnarf) -> true
          else -> false
        }
        SafeFuture.completedFuture(endBlockNumber)
      }

    var acceptedBlob = 0UL
    val acceptedBlobEndBlockNumberConsumer = Consumer<ULong> { acceptedBlob = it }
    val blobsFilter = L1ShnarfBasedAlreadySubmittedBlobsFilter(
      lineaRollup = l1SmcClient,
      acceptedBlobEndBlockNumberConsumer = acceptedBlobEndBlockNumberConsumer,
    )

    val filteredBlobs = blobsFilter.invoke(blobs).get()

    assertThat(filteredBlobs).isEqualTo(listOf(blob6, blob7))
    assertThat(acceptedBlob).isEqualTo(blob5.endBlockNumber)
  }
}

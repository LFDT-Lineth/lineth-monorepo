package lineth.submission

import linea.contract.l1.BlobsSubmissionV9
import linea.contract.l1.LineaValidiumSmartContractClient
import linea.contract.l1.LinethRollupSmartContractClient
import linea.domain.BlobData
import linea.domain.BlobRecordV2
import linea.domain.createBlobData
import linea.domain.createBlobRecordV2
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.mockito.kotlin.any
import org.mockito.kotlin.anyOrNull
import org.mockito.kotlin.argumentCaptor
import org.mockito.kotlin.eq
import org.mockito.kotlin.mock
import org.mockito.kotlin.never
import org.mockito.kotlin.times
import org.mockito.kotlin.verify
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.random.Random

class BlobSubmittersV2Test {
  private val hashes = List(5) { Random.nextBytes(32) }
  private val blobs = List(4) { createBlobData(prevDataRollingHash = hashes[it], dataRollingHash = hashes[it + 1]) }

  // rollup1 = [b0, b1, b2] ending inside b2; rollup2 = [b2 (shared), b3] ending inside b3; rollup3 lies inside b3.
  private val rollup1 = record(1UL, 10UL, hashes[0], hashes[3], blobs.subList(0, 3), take = 0, discard = 100)
  private val rollup2 = record(11UL, 20UL, hashes[3], hashes[4], blobs.subList(2, 4), take = 100, discard = 50)
  private val rollup3 = record(21UL, 30UL, hashes[4], hashes[4], blobs.subList(3, 4), take = 50, discard = 10)

  private fun record(
    startBlockNumber: ULong,
    endBlockNumber: ULong,
    parentDataRollingHash: ByteArray,
    dataRollingHash: ByteArray,
    blobsData: List<BlobData>,
    take: Int,
    discard: Int,
  ): BlobRecordV2 =
    createBlobRecordV2(
      startBlockNumber = startBlockNumber,
      endBlockNumber = endBlockNumber,
      parentDataRollingHash = parentDataRollingHash,
      dataRollingHash = dataRollingHash,
      parentDataTailTakeBytes = take,
      boundaryPrevDataRollingHash = if (take > 0) blobsData.first().prevDataRollingHash else null,
      finalDataTailDiscardBytes = discard,
      blobsData = blobsData,
    )

  private fun rollupContract(): LinethRollupSmartContractClient = mock {
    on { submitBlobsV9(any(), anyOrNull(), any(), any()) }.thenReturn(SafeFuture.completedFuture("0x01"))
  }

  private fun eip4844Submitter(contract: LinethRollupSmartContractClient) =
    BlobSubmitterAsEIP4844MultipleBlobsPerTxV2(
      contract = contract,
      targetBlobsPerTransaction = 2U,
      gasPriceCapProvider = null,
    )

  @Test
  fun `blobs are submitted once in stream order, skipping the shared first blob`() {
    val contract = rollupContract()

    val txHashes = eip4844Submitter(contract).submitBlobs(listOf(rollup1, rollup2, rollup3)).get()

    val captor = argumentCaptor<BlobsSubmissionV9>()
    verify(contract, times(3)).submitBlobsV9(captor.capture(), anyOrNull(), eq(false), eq(false))
    assertThat(txHashes).hasSize(3)
    val (tx1, tx2, tx3) = captor.allValues
    assertThat(tx1.blobs).containsExactly(blobs[0].blobBytes, blobs[1].blobBytes)
    assertThat(tx1.parentDataRollingHash).isEqualTo(hashes[0])
    assertThat(tx1.storedDataRollingHash).isEqualTo(hashes[2])
    assertThat(tx2.blobs).containsExactly(blobs[2].blobBytes)
    assertThat(tx2.parentDataRollingHash).isEqualTo(hashes[2])
    assertThat(tx2.storedDataRollingHash).isEqualTo(hashes[3])
    // rollup2 skips the shared b2 and submits only b3; rollup3 has no new blob, so no transaction.
    assertThat(tx3.blobs).containsExactly(blobs[3].blobBytes)
    assertThat(tx3.parentDataRollingHash).isEqualTo(hashes[3])
    assertThat(tx3.storedDataRollingHash).isEqualTo(hashes[4])
  }

  @Test
  fun `eth_call checks the first transaction after the shared blob, and nothing for a rollup without new blobs`() {
    val contract = rollupContract()
    val submitter = eip4844Submitter(contract)

    submitter.submitBlobCall(rollup2).get()
    submitter.submitBlobCall(rollup3).get()

    val captor = argumentCaptor<BlobsSubmissionV9>()
    verify(contract).submitBlobsV9(captor.capture(), anyOrNull(), eq(false), eq(true))
    assertThat(captor.firstValue.blobs).containsExactly(blobs[3].blobBytes)
    assertThat(captor.firstValue.parentDataRollingHash).isEqualTo(hashes[3])
  }

  @Test
  fun `validium accepts each rollup's rolling hash range and skips a rollup without new data`() {
    val contract = mock<LineaValidiumSmartContractClient> {
      on { acceptDataRollingHash(any(), any(), anyOrNull(), any()) }.thenReturn(SafeFuture.completedFuture("0x01"))
    }
    val submitter = ValidiumBlobSubmitterV2(contract = contract, gasPriceCapProvider = null)

    assertThat(submitter.submitBlobs(listOf(rollup1, rollup2, rollup3)).get()).hasSize(2)
    submitter.submitBlobCall(rollup3).get()

    verify(contract).acceptDataRollingHash(eq(hashes[0]), eq(hashes[3]), anyOrNull(), eq(false))
    verify(contract).acceptDataRollingHash(eq(hashes[3]), eq(hashes[4]), anyOrNull(), eq(false))
    verify(contract, never()).acceptDataRollingHash(eq(hashes[4]), any(), anyOrNull(), any())
  }
}

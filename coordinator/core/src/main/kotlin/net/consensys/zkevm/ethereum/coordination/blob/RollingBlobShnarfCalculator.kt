package net.consensys.zkevm.ethereum.coordination.blob

import com.github.michaelbull.result.getOrThrow
import com.github.michaelbull.result.map
import com.github.michaelbull.result.onSuccess
import com.github.michaelbull.result.recover
import com.github.michaelbull.result.runCatching
import linea.domain.BlockInterval
import linea.domain.BlockIntervals
import linea.kotlin.encodeHex
import net.consensys.zkevm.domain.BlobRecord
import net.consensys.zkevm.persistence.BlobsRepository
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicReference

data class RollingBlobShnarfResult(
  val shnarfResult: ShnarfResult,
  val parentBlobHash: ByteArray,
  val parentBlobShnarf: ByteArray,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as RollingBlobShnarfResult

    if (shnarfResult != other.shnarfResult) return false
    if (!parentBlobHash.contentEquals(other.parentBlobHash)) return false
    if (!parentBlobShnarf.contentEquals(other.parentBlobShnarf)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = shnarfResult.hashCode()
    result = 31 * result + parentBlobHash.contentHashCode()
    result = 31 * result + parentBlobShnarf.contentHashCode()
    return result
  }
}

class ParentBlobDataProviderImpl(private val blobsRepository: BlobsRepository) : ParentBlobDataProvider {
  override fun getParentBlobShnarfMetaData(currentBlobRange: BlockInterval): SafeFuture<BlobShnarfMetaData> {
    val parentBlobEndBlockNumber = currentBlobRange.startBlockNumber.dec()
    return blobsRepository
      .findBlobByEndBlockNumber(parentBlobEndBlockNumber.toLong())
      .thenCompose { blobRecord: BlobRecord? ->
        if (blobRecord != null) {
          SafeFuture.completedFuture(
            BlobShnarfMetaData(
              startBlockNumber = blobRecord.startBlockNumber,
              endBlockNumber = blobRecord.endBlockNumber,
              blobHash = blobRecord.blobHash,
              blobShnarf = blobRecord.expectedShnarf,
            ),
          )
        } else {
          SafeFuture.failedFuture(
            IllegalStateException("Failed to find the parent blob in db with end block=$parentBlobEndBlockNumber"),
          )
        }
      }
  }
}

class RollingBlobShnarfCalculator(
  private val blobShnarfCalculator: BlobShnarfCalculator,
  private val parentBlobDataProvider: ParentBlobDataProvider,
  private val genesisShnarf: ByteArray,
) {
  private val log: Logger = LogManager.getLogger(this::class.java)

  private var parentBlobShnarfMetaDataReference: AtomicReference<BlobShnarfMetaData?> = AtomicReference(null)

  private fun getParentBlobData(blobBlockRange: BlockInterval): SafeFuture<BlobShnarfMetaData> {
    val parentBlobEndBlockNumber = blobBlockRange.startBlockNumber.dec()
    return if (parentBlobEndBlockNumber == 0UL) {
      log.info(
        "Requested parent shnarf for the genesis block, returning genesisShnarf={}",
        genesisShnarf.encodeHex(),
      )
      SafeFuture.completedFuture(
        BlobShnarfMetaData(
          startBlockNumber = 0UL,
          endBlockNumber = 0UL,
          blobHash = ByteArray(32),
          blobShnarf = genesisShnarf,
        ),
      )
    } else if (parentBlobShnarfMetaDataReference.get() != null) {
      val parentBlobData = parentBlobShnarfMetaDataReference.get()!!
      if (parentBlobData.endBlockNumber != parentBlobEndBlockNumber) {
        SafeFuture.failedFuture(
          IllegalStateException(
            "Blob block range start block number=${blobBlockRange.startBlockNumber} " +
              "is not equal to parent blob end block number=${parentBlobData.endBlockNumber} + 1",
          ),
        )
      } else {
        SafeFuture.completedFuture(parentBlobData)
      }
    } else {
      parentBlobDataProvider.getParentBlobShnarfMetaData(blobBlockRange)
    }
  }

  fun calculateShnarf(
    compressedData: ByteArray,
    parentStateRootHash: ByteArray,
    finalStateRootHash: ByteArray,
    conflationOrder: BlockIntervals,
  ): SafeFuture<RollingBlobShnarfResult> {
    val blobBlockRange =
      BlockInterval.between(
        conflationOrder.startingBlockNumber,
        conflationOrder.upperBoundaries.last(),
      )
    return getParentBlobData(blobBlockRange).thenCompose { parentBlobData ->
      runCatching {
        blobShnarfCalculator.calculateShnarf(
          compressedData = compressedData,
          parentStateRootHash = parentStateRootHash,
          finalStateRootHash = finalStateRootHash,
          prevShnarf = parentBlobData.blobShnarf,
          conflationOrder = conflationOrder,
        )
      }.onSuccess { shnarfResult ->
        parentBlobShnarfMetaDataReference.set(
          BlobShnarfMetaData(
            startBlockNumber = blobBlockRange.startBlockNumber,
            endBlockNumber = blobBlockRange.endBlockNumber,
            blobHash = shnarfResult.dataHash,
            blobShnarf = shnarfResult.expectedShnarf,
          ),
        )
      }.map {
        SafeFuture.completedFuture(
          RollingBlobShnarfResult(
            shnarfResult = it,
            parentBlobHash = parentBlobData.blobHash,
            parentBlobShnarf = parentBlobData.blobShnarf,
          ),
        )
      }.recover { error ->
        SafeFuture.failedFuture(error)
      }.getOrThrow()
    }
  }
}

package net.consensys.zkevm.ethereum.submission

import linea.contract.l1.LineaSmartContractClientReadOnly
import net.consensys.linea.async.AsyncFilter
import net.consensys.zkevm.domain.BlobRecord
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.function.Consumer

class L1ShnarfBasedAlreadySubmittedBlobsFilter(
  private val lineaSmartContractClientReadOnly: LineaSmartContractClientReadOnly,
  private val acceptedBlobEndBlockNumberConsumer: Consumer<ULong> = Consumer<ULong> { },
) : AsyncFilter<BlobRecord> {

  /**
   * Filters out blobs that have already been submitted to the smart contract.
   * shnarfFinalBlockNumbers stores only the last blob in a transaction: tx with multiple blobs:
   * b1[10..19], b2[20..30], b3[31..40] -> shnarfFinalBlockNumbers = {b3 => 40}
   *
   * if blobRecords=[b1, b2, b3, b4, b5, b6] the result will be [b4, b5, b6]
   */
  override fun invoke(items: List<BlobRecord>): SafeFuture<List<BlobRecord>> {
    val blockByShnarfQueryFutures = items.map { blobRecord ->
      lineaSmartContractClientReadOnly
        .isBlobShnarfPresent(shnarf = blobRecord.expectedShnarf)
        .thenApply { isShnarfPresent ->
          if (isShnarfPresent) {
            blobRecord.endBlockNumber
          } else {
            null
          }
        }
    }

    return SafeFuture.collectAll(blockByShnarfQueryFutures.stream())
      .thenApply { blockNumbersFoundInSmartContract ->
        blockNumbersFoundInSmartContract
          .filterNotNull()
          .maxOfOrNull { it }
      }
      .thenApply { highestBlobEndBlockNumberFoundInL1 ->
        highestBlobEndBlockNumberFoundInL1?.also(acceptedBlobEndBlockNumberConsumer::accept)
          ?.let { blockNumber -> items.filter { it.startBlockNumber > blockNumber } }
          ?: items
      }
  }
}

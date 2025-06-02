package net.consensys.zkevm.domain

import kotlinx.datetime.Clock
import kotlinx.datetime.Instant
import linea.kotlin.setFirstByteToZero
import linea.kotlin.trimToSecondPrecision
import kotlin.random.Random
import kotlin.time.Duration.Companion.seconds

fun createProofToFinalize(
  firstBlockNumber: Long,
  finalBlockNumber: Long,
  startBlockTime: Instant = Clock.System.now().trimToSecondPrecision(),
  parentAggregationProof: ProofToFinalize? = null,
  parentBlob: BlobRecord? = null,
  dataParentHash: ByteArray = parentBlob?.blobCompressionProof?.dataHash
    ?: Random.nextBytes(32).setFirstByteToZero(),
  parentStateRootHash: ByteArray = parentBlob?.blobCompressionProof?.finalStateRootHash
    ?: Random.nextBytes(32).setFirstByteToZero(),
  dataHashes: List<ByteArray> = (firstBlockNumber..finalBlockNumber).map {
    Random.nextBytes(32)
  },
  parentAggregationLastBlockTimestamp: Instant = parentAggregationProof?.finalTimestamp
    ?: startBlockTime.minus(2.seconds),
  finalTimestamp: Instant = startBlockTime.plus((2L * (finalBlockNumber - firstBlockNumber)).seconds),
  l2MessagingBlockOffsets: ByteArray = ByteArray(32),
): ProofToFinalize {
  return ProofToFinalize(
    aggregatedProof = Random.nextBytes(32).setFirstByteToZero(),
    parentStateRootHash = parentStateRootHash,
    aggregatedVerifierIndex = 0,
    aggregatedProofPublicInput = Random.nextBytes(32).setFirstByteToZero(),
    dataHashes = dataHashes,
    dataParentHash = dataParentHash,
    firstBlockNumber = firstBlockNumber,
    finalBlockNumber = finalBlockNumber,
    parentAggregationLastBlockTimestamp = parentAggregationLastBlockTimestamp,
    finalTimestamp = finalTimestamp,
    l1RollingHash = ByteArray(32),
    l1RollingHashMessageNumber = 0,
    l2MerkleRoots = listOf(Random.nextBytes(32).setFirstByteToZero()),
    l2MerkleTreesDepth = 0,
    l2MessagingBlocksOffsets = l2MessagingBlockOffsets,
  )
}

fun createProofToFinalizeFromBlobs(
  blobRecords: List<BlobRecord>,
  parentBlob: BlobRecord? = null,
  parentAggregationProof: ProofToFinalize? = null,
  parentStateRootHash: ByteArray = parentBlob?.blobCompressionProof?.finalStateRootHash
    ?: Random.nextBytes(32).setFirstByteToZero(),
  dataParentHash: ByteArray = parentBlob?.blobCompressionProof?.dataHash
    ?: Random.nextBytes(32).setFirstByteToZero(),
): ProofToFinalize {
  return createProofToFinalize(
    firstBlockNumber = blobRecords.first().startBlockNumber.toLong(),
    finalBlockNumber = blobRecords.last().endBlockNumber.toLong(),
    startBlockTime = blobRecords.first().startBlockTime,
    parentAggregationProof = parentAggregationProof,
    parentBlob = parentBlob,
    parentStateRootHash = parentStateRootHash,
    dataParentHash = dataParentHash,
    finalTimestamp = blobRecords.last().endBlockTime,
    dataHashes = blobRecords.map { it.blobCompressionProof!!.dataHash },
  )
}

fun createProofToFinalizeFromBlobs(
  blobsByAggregation: List<List<BlobRecord>>,
  parentStateRootHash: ByteArray = Random.nextBytes(32),
  dataParentHash: ByteArray = Random.nextBytes(32),
): List<ProofToFinalize> {
  val firstAggregationProof = createProofToFinalizeFromBlobs(
    blobRecords = blobsByAggregation.first(),
    parentStateRootHash = parentStateRootHash,
    dataParentHash = dataParentHash,
  )
  val aggregations = mutableListOf<ProofToFinalize>(firstAggregationProof)

  blobsByAggregation
    .drop(1)
    .fold(firstAggregationProof to blobsByAggregation.first().last()) { (parentAggProof, parentBlob), aggBlobs ->
      val aggregationProof = createProofToFinalizeFromBlobs(
        blobRecords = aggBlobs,
        parentAggregationProof = parentAggProof,
        parentBlob = parentBlob,
      )
      aggregations.add(aggregationProof)
      aggregationProof to aggBlobs.last()
    }

  return aggregations
}

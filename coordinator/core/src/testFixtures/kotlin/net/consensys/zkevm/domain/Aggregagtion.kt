package net.consensys.zkevm.domain

fun createAggregation(
  startBlockNumber: Long? = null,
  endBlockNumber: Long? = null,
  batchCount: Long = 1,
  aggregationProof: ProofToFinalize? = null,
): Aggregation {
  require(
    (startBlockNumber != null && endBlockNumber != null) ||
      aggregationProof != null,
  ) { "Either aggregationProof or startBlockNumber, endBlockNumber must be provided" }
  val effectiveAggregationProof = aggregationProof ?: createProofToFinalize(
    firstBlockNumber = startBlockNumber!!,
    finalBlockNumber = endBlockNumber!!,
  )

  return Aggregation(
    startBlockNumber = effectiveAggregationProof.firstBlockNumber.toULong(),
    endBlockNumber = effectiveAggregationProof.finalBlockNumber.toULong(),
    batchCount = batchCount.toULong(),
    aggregationProof = effectiveAggregationProof,
  )
}

fun createAggregation(
  endBlockNumber: Long,
  parentAggregation: Aggregation,
  batchCount: Long = 1,
  aggregationProof: ProofToFinalize = createProofToFinalize(
    firstBlockNumber = (parentAggregation.endBlockNumber + 1UL).toLong(),
    finalBlockNumber = endBlockNumber,
  ),
): Aggregation {
  return createAggregation(
    batchCount = batchCount,
    aggregationProof = aggregationProof,
  )
}

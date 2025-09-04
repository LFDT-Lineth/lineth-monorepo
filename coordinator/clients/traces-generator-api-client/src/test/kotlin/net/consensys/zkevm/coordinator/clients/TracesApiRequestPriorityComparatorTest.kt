package net.consensys.zkevm.coordinator.clients

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class TracesApiRequestPriorityComparatorTest {
  private val reqBuilder = TracesGeneratorJsonRpcClientV2.RequestBuilder("v2")
  private val comparator = TracesGeneratorJsonRpcClientV2.requestPriorityComparator

  @Test
  fun `should put tracesConflation first`() {
    val req1 = reqBuilder.buildGetTracesCountersV2Request(blockNumber = 1u)
    val req2 = reqBuilder.buildGenerateConflatedTracesToFileV2Request(startBlockNumber = 1u, endBlockNumber = 2u)

    assertThat(comparator.compare(req1, req2)).isGreaterThan(0)
    assertThat(comparator.compare(req2, req1)).isLessThan(0)
  }

  @Test
  fun `should compare by block number - trace conflation method`() {
    val req1 = reqBuilder.buildGetTracesCountersV2Request(blockNumber = 1u)
    val req2 = reqBuilder.buildGetTracesCountersV2Request(blockNumber = 2u)

    assertThat(comparator.compare(req1, req2)).isLessThan(0)
    assertThat(comparator.compare(req2, req1)).isGreaterThan(0)

    assertThat(comparator.compare(req1, reqBuilder.buildGetTracesCountersV2Request(blockNumber = 1u))).isEqualTo(0)
  }

  @Test
  fun `should compare by block number - trace counting method`() {
    val req1 = reqBuilder.buildGenerateConflatedTracesToFileV2Request(startBlockNumber = 1u, endBlockNumber = 20u)
    val req2 = reqBuilder.buildGenerateConflatedTracesToFileV2Request(startBlockNumber = 2u, endBlockNumber = 20u)

    assertThat(comparator.compare(req1, req2)).isLessThan(0)
    assertThat(comparator.compare(req2, req1)).isGreaterThan(0)

    assertThat(
      comparator.compare(
        req1,
        reqBuilder.buildGenerateConflatedTracesToFileV2Request(startBlockNumber = 1u, endBlockNumber = 10u),
      ),
    ).isEqualTo(0)
  }
}

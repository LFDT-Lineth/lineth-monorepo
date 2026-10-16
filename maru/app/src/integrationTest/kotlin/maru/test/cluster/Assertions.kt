package maru.test.cluster

import maru.test.extensions.assertNodesAreSyncedUpTo
import maru.test.extensions.besuBlocks
import maru.test.extensions.getBlockByNumber
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.kotlin.await
import org.hyperledger.besu.tests.acceptance.dsl.node.BesuNode
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

/**
 * Asserts that all EL nodes have the same canonical chain (same blockHash)
 * at the given minChainHeight
 */
fun MaruCluster.assertSameCanonicalElChain(
  chainHeight: ULong,
  referenceElNode: BesuNode = this.besuNode(NodeRole.Sequencer),
  timeout: Duration = 30.seconds,
  pollingInterval: Duration = 1.seconds,
) {
  await
    .pollDelay(pollingInterval.toJavaDuration())
    .timeout(timeout.toJavaDuration())
    .untilAsserted {
      this.assertNodesAreSyncedUpTo(targetBlockNumber = chainHeight, expectBeaconChain = false)

      val expectedChainHead = referenceElNode.getBlockByNumber(chainHeight)

      val chainHeads = this.besuBlocks(chainHeight)
      for ((node, head) in chainHeads) {
        assertThat(head)
          .withFailMessage(
            "Node ${node.label} head mismatch: ",
          ).isEqualTo(expectedChainHead)
      }
    }
}

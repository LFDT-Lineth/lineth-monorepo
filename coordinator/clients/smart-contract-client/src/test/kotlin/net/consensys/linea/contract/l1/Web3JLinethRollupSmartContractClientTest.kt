package net.consensys.linea.contract.l1

import linea.EthLogsSearcher
import linea.contract.LinethRollupV6
import linea.contract.l1.LinethRollupContractVersion
import linea.domain.createBlobRecord
import linea.web3j.transactionmanager.AsyncFriendlyTransactionManager
import net.consensys.linea.contract.Web3JContractAsyncHelper
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import org.mockito.kotlin.any
import org.mockito.kotlin.anyOrNull
import org.mockito.kotlin.doReturn
import org.mockito.kotlin.mock
import org.mockito.kotlin.spy
import org.mockito.kotlin.whenever
import org.web3j.protocol.Web3j
import tech.pegasys.teku.infrastructure.async.SafeFuture

class Web3JLinethRollupSmartContractClientTest {
  private val contractHelper = mock<Web3JContractAsyncHelper> {
    on { sendBlobCarryingTransactionAndGetTxHash(any(), any(), anyOrNull()) } doReturn
      SafeFuture.completedFuture("0xtx")
    on { executeBlobEthCall(any(), any(), anyOrNull()) } doReturn SafeFuture.completedFuture("0x")
  }
  private val client = spy(
    Web3JLinethRollupSmartContractClient(
      contractAddress = "0x0000000000000000000000000000000000000001",
      web3j = mock<Web3j>(),
      transactionManager = mock<AsyncFriendlyTransactionManager>(),
      web3jContractHelper = contractHelper,
      web3jLineaClient = mock<LinethRollupV6>(),
      ethLogsSearcher = mock<EthLogsSearcher>(),
    ),
  )

  @Test
  fun `submits compression-proven blobs`() {
    whenever(client.getVersion()).thenReturn(SafeFuture.completedFuture(LinethRollupContractVersion.V6))
    val blob = createBlobRecord(startBlockNumber = 1UL, endBlockNumber = 2UL).let {
      it.copy(blobCompressionProof = it.blobCompressionProof!!.copy(expectedY = ByteArray(32) { 1 }))
    }

    assertThat(client.submitBlobs(listOf(blob), null).get()).isEqualTo("0xtx")
    assertThat(client.submitBlobsEthCall(listOf(blob), null).get()).isEqualTo("0x")
  }

  @Test
  fun `rejects blob submission without a compression proof`() {
    whenever(client.getVersion()).thenReturn(SafeFuture.completedFuture(LinethRollupContractVersion.V6))
    val blob = createBlobRecord(startBlockNumber = 1UL, endBlockNumber = 2UL)
      .copy(blobCompressionProof = null)

    assertThatThrownBy { client.submitBlobs(listOf(blob), null).get() }
      .hasRootCauseMessage("submitBlobs: blob at index=0 is missing a compression proof")
    assertThatThrownBy { client.submitBlobsEthCall(listOf(blob), null).get() }
      .hasRootCauseMessage("submitBlobsEthCall: blob at index=0 is missing a compression proof")
  }
}

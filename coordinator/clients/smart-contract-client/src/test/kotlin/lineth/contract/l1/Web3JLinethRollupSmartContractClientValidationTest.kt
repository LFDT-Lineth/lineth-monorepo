package lineth.contract.l1

import linea.EthLogsSearcher
import linea.contract.LinethRollupV6
import linea.domain.createBlobRecord
import linea.web3j.transactionmanager.AsyncFriendlyTransactionManager
import net.consensys.linea.contract.Web3JContractAsyncHelper
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertDoesNotThrow
import org.mockito.kotlin.mock
import org.mockito.kotlin.verifyNoInteractions
import org.web3j.protocol.Web3j
import tech.pegasys.teku.infrastructure.async.SafeFuture

class Web3JLinethRollupSmartContractClientValidationTest {
  private val contractHelper = mock<Web3JContractAsyncHelper>()
  private val contract = mock<LinethRollupV6>()
  private val client = Web3JLinethRollupSmartContractClient(
    contractAddress = "0x0000000000000000000000000000000000000001",
    web3j = mock<Web3j>(),
    transactionManager = mock<AsyncFriendlyTransactionManager>(),
    web3jContractHelper = contractHelper,
    web3jLineaClient = contract,
    ethLogsSearcher = mock<EthLogsSearcher>(),
  )

  @Test
  fun `rejects missing compression proofs before contract calls`() {
    val blob = createBlobRecord(startBlockNumber = 1UL, endBlockNumber = 2UL)
      .copy(blobCompressionProof = null)
    val submissions = listOf<Pair<String, () -> SafeFuture<*>>>(
      "submitBlobs" to { client.submitBlobs(listOf(blob), null) },
      "submitBlobsEthCall" to { client.submitBlobsEthCall(listOf(blob), null) },
    )

    submissions.forEach { (operation, submit) ->
      val result = assertDoesNotThrow { submit() }
      assertThatThrownBy { result.get() }
        .hasRootCauseMessage("$operation: blob at index=0 is missing a compression proof")
    }
    verifyNoInteractions(contractHelper, contract)
  }
}

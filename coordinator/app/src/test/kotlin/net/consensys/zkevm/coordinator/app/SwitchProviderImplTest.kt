@file:Suppress("DEPRECATION")

package net.consensys.zkevm.coordinator.app

import build.linea.web3j.Web3JLogsClient
import io.vertx.junit5.Timeout
import io.vertx.junit5.VertxExtension
import io.vertx.junit5.VertxTestContext
import net.consensys.linea.contract.EIP1559GasProvider
import net.consensys.linea.contract.L2MessageService
import net.consensys.toULong
import net.consensys.zkevm.ethereum.coordination.conflation.upgrade.SwitchProvider
import net.consensys.zkevm.ethereum.signing.ECKeypairSigner
import net.consensys.zkevm.ethereum.signing.ECKeypairSignerAdapter
import net.consensys.zkevm.ethereum.submission.SwitchProviderImpl
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.Mockito
import org.mockito.kotlin.any
import org.mockito.kotlin.argumentCaptor
import org.mockito.kotlin.doReturn
import org.mockito.kotlin.mock
import org.mockito.kotlin.times
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import org.web3j.abi.EventEncoder
import org.web3j.abi.FunctionEncoder
import org.web3j.abi.TypeEncoder
import org.web3j.crypto.Credentials
import org.web3j.crypto.Keys
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.Request
import org.web3j.protocol.core.methods.request.EthFilter
import org.web3j.protocol.core.methods.response.EthBlock
import org.web3j.protocol.core.methods.response.EthLog
import org.web3j.protocol.core.methods.response.Log
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import java.util.concurrent.TimeUnit

@ExtendWith(VertxExtension::class)
class SwitchProviderImplTest {
  private val testContractAddress = "0x6d976c9b8ceee705d4fe8699b44e5eb58242f484"
  private val gasEstimationPercentile = 0.5
  private val gasLimit = 25_000_000uL
  private val feeHistoryBlockCount = 4u
  private val maxFeePerGasCap = 10000uL
  private val l2Web3j = mock<Web3j>(defaultAnswer = Mockito.RETURNS_DEEP_STUBS)
  private val eventBlockNumber = BigInteger.valueOf(123456L)
  private val earliestBlock = 0UL

  private val ethGetBlockByNumberMock: Request<Any, EthBlock> = mock {
    on { sendAsync() } doReturn SafeFuture.completedFuture(
      EthBlock().apply {
        result = EthBlock.Block()
          .apply {
            setNumber("0x3E8")
            hash = "0x1000000000000000000000000000000000000000000000000000000000000000"
          }
      }
    )
  }

  @Test
  @Timeout(20, timeUnit = TimeUnit.SECONDS)
  fun findSwitchBlockNumber_findsEventBlockIfItExists(testContext: VertxTestContext) {
    val l2MessageService = createL2MessageServiceContractWithSimpleKeypairSigner(
      l2Web3j
    )
    val web3JLogsClient = mock<Web3JLogsClient>()
    val switchBlockNumberProvider = SwitchProviderImpl(
      web3JLogsClient,
      l2MessageService,
      earliestBlock
    )
    whenever(l2Web3j.ethGetBlockByNumber(any(), any())).thenReturn(ethGetBlockByNumberMock)

    val ethEventLogs: List<Log> = listOf(createRandomServiceVersionMigratedEvent())
    whenever(web3JLogsClient.getLogs(any()))
      .thenAnswer { SafeFuture.completedFuture(ethEventLogs) }

    val captor = argumentCaptor<EthFilter>()

    switchBlockNumberProvider.getSwitch(SwitchProvider.ProtocolSwitches.DATA_COMPRESSION_PROOF_AGGREGATION)
      .thenApply {
        testContext
          .verify {
            verify(web3JLogsClient, times(1))
              .getLogs(captor.capture())
            assertThat(
              captor.allValues.all { ethFilter ->
                ethFilter.topics[1].value.equals(
                  "0x0000000000000000000000000000000000000000000000000000000000000002"
                )
              }
            ).isTrue()
            assertThat(it).isNotNull()
            assertThat(it!!).isEqualTo(eventBlockNumber.toULong())
          }.completeNow()
      }.whenException(testContext::failNow)
  }

  private fun createRandomServiceVersionMigratedEvent(): Log {
    val log = EthLog.LogObject()
    val eventSignature: String = EventEncoder.encode(L2MessageService.SERVICEVERSIONMIGRATED_EVENT)

    log.topics =
      listOf(
        eventSignature,
        TypeEncoder.encode(
          org.web3j.abi.datatypes.Int(
            BigInteger.valueOf(SwitchProvider.ProtocolSwitches.DATA_COMPRESSION_PROOF_AGGREGATION.int.toLong())
          )
        )
      )

    log.data = FunctionEncoder.encodeConstructor(emptyList())

    log.setBlockNumber(eventBlockNumber.toString())
    log.setLogIndex("0")

    return log
  }

  private fun createL2MessageServiceContractWithSimpleKeypairSigner(
    l2Web3jClient: Web3j
  ): L2MessageService {
    val keyPair = Keys.createEcKeyPair()
    val signer = ECKeypairSigner(keyPair)
    val signerAdapter = ECKeypairSignerAdapter(signer, keyPair.publicKey)
    val credentials = Credentials.create(signerAdapter)
    return L2MessageService.load(
      testContractAddress,
      l2Web3jClient,
      credentials,
      EIP1559GasProvider(
        l2Web3jClient,
        EIP1559GasProvider.Config(gasLimit, maxFeePerGasCap, feeHistoryBlockCount, gasEstimationPercentile)
      )
    )
  }
}

package lineth.coordination.riscv.execution

import linea.domain.Block
import linea.domain.BlockParameter
import linea.domain.BlocksConflation
import linea.domain.ConflationCalculationResult
import linea.domain.ConflationTrigger
import linea.domain.ExecutionPayload
import linea.domain.createBlock
import linea.domain.toExecutionPayload
import linea.ethapi.ExecutionPayloadClient
import linea.ethapi.ExecutionWitness
import linea.ethapi.ExecutionWitnessClient
import lineth.persistence.ftx.FakeForcedTransactionsDao
import net.consensys.linea.traces.TracesCountersV2
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import tech.pegasys.teku.infrastructure.async.SafeFuture

class L2ExecutionRequestBuilderImplTest {
  private val blocks = listOf(createBlock(number = 1UL), createBlock(number = 2UL))
  private val conflation = BlocksConflation(
    blocks,
    ConflationCalculationResult(1UL, 2UL, ConflationTrigger.BLOCKS_LIMIT, TracesCountersV2.EMPTY_TRACES_COUNT),
  )
  private val callOrder = mutableListOf<String>()
  private val payloadRequests = mutableListOf<Block>()
  private val witnessRequests = mutableListOf<BlockParameter>()
  private val payloadFutures = blocks.map { SafeFuture<ExecutionPayload>() }
  private val witnessFutures = blocks.map { SafeFuture<ExecutionWitness?>() }
  private val builder = L2ExecutionRequestBuilderImpl(
    executionPayloadClient = object : ExecutionPayloadClient {
      override fun getExecutionPayload(block: Block): SafeFuture<ExecutionPayload> {
        callOrder += "payload"
        payloadRequests += block
        return payloadFutures[(block.number - 1UL).toInt()]
      }
    },
    executionWitnessClient = object : ExecutionWitnessClient {
      override fun getExecutionWitness(block: BlockParameter): SafeFuture<ExecutionWitness?> {
        callOrder += "witness"
        witnessRequests += block
        return witnessFutures[witnessRequests.lastIndex]
      }
    },
    forcedTransactionsDao = FakeForcedTransactionsDao(),
    chainId = 59144UL,
  )

  @Test
  fun `matches payload and witness by hash and preserves block order despite completion order`() {
    val future = builder.build(conflation)
    assertThat(callOrder).containsExactly("witness", "payload", "witness", "payload")
    val payloads = blocks.map {
      it.toExecutionPayload(byteArrayOf(0xc0.toByte())).copy(slotNumber = it.number + 10UL)
    }
    val witnesses = blocks.map { ExecutionWitness(listOf(byteArrayOf(it.number.toByte())), emptyList(), emptyList()) }

    payloadFutures[1].complete(payloads[1])
    witnessFutures[0].complete(witnesses[0])
    assertThat(future).isNotDone()
    witnessFutures[1].complete(witnesses[1])
    payloadFutures[0].complete(payloads[0])

    val request = future.get()
    assertThat(request.executions.map { it.executionPayload }).containsExactlyElementsOf(payloads)
    assertThat(request.executions.map { it.executionWitness }).containsExactlyElementsOf(witnesses)
    assertThat(request.executions.map { it.blockNumber }).containsExactly(1UL, 2UL)
    assertThat(payloadRequests).containsExactlyElementsOf(blocks)
    assertThat(witnessRequests).containsExactlyElementsOf(blocks.map { BlockParameter.fromHash(it.hash) })
    assertThat(request.chainId).isEqualTo(59144UL)
    assertThat(request.parentFtxNumber).isEqualTo(0UL)
  }

  @Test
  fun `missing witness fails the request`() {
    val future = builder.build(conflation)
    blocks.forEachIndexed { index, block ->
      payloadFutures[index].complete(block.toExecutionPayload(ByteArray(0)))
      witnessFutures[index].complete(null)
    }
    assertThatThrownBy { future.get() }.hasStackTraceContaining("No execution witness available for block")
  }

  @Test
  fun `unavailable payload fails the request`() {
    val future = builder.build(conflation)
    payloadFutures.zip(witnessFutures).forEach { (payload, witness) ->
      payload.completeExceptionally(IllegalStateException("BAL unavailable"))
      witness.complete(ExecutionWitness(emptyList(), emptyList(), emptyList()))
    }
    assertThatThrownBy { future.get() }.hasStackTraceContaining("BAL unavailable")
  }
}

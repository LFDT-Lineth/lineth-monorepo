package linea.coordinator.clients.prover.riscv

import lineth.clients.BlobWitness
import lineth.clients.ChainConfig
import lineth.clients.ExecutionInfo
import lineth.clients.ExecutionPayload
import lineth.clients.ForcedTransaction
import lineth.clients.L2ExecutionProofRequestV1
import lineth.clients.RollupAggregationProofRequestV1
import lineth.clients.RollupProofRequestV1
import lineth.clients.Withdrawal
import lineth.domain.BlockIntervalProofIndex
import lineth.ethapi.ExecutionWitness
import lineth.forcedtx.ForcedTransactionInclusionResult
import lineth.kotlin.encodeHex
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import java.math.BigInteger
import kotlin.time.Instant

/**
 * Verifies that the RISC-V proof-request mappers encode every field of a domain request into its request DTO
 * (domain `ByteArray` -> DTO `String` (hex), domain `ULong` -> DTO `Long`) and assemble the request envelope
 * (`guestProgramId`, `metadata` block range, params) correctly. Covers the transport-free mappers; the file-based
 * mappers (which resolve inlined proofs through a transport) are exercised by the file-based client tests.
 */
class RiscVProofRequestDtoMapperTest {

  private val guestProgramId = "0x31139b3eaece046f5675fe237c36246e7bb2a5acc4cf4b358aef65c6d3771f4d"
  private val chainId = 59144L
  private val forkName = "Amsterdam"
  private val l2MessageServiceAddress = "0x508ca82df566dcd1b0019d2dedf7e3d6f7ad6dde"
  private val coinbase = "0x0000000000000000000000000000000000000000"

  @Test
  fun `L2ExecutionProofRequestDtoMapper encodes every field`() {
    val request = l2Request()
    val execution = request.executions.first()

    val dto = L2ExecutionProofRequestDtoMapper(
      guestProgramId,
      l2MessageServiceAddress,
      coinbase,
    ).invoke(request).get()

    assertThat(dto).isEqualTo(
      L2ExecutionProofRequestDto(
        guestProgramId = guestProgramId,
        proofRequest = L2ExecutionProofRequestParamsDto(
          parentFtxRollingHash = request.parentFtxRollingHash.encodeHex(),
          parentLastProcessedFtxNumber = request.parentFtxNumber.toLong(),
          payloads = listOf(
            PayloadInputDto(
              statelessInput = StatelessInputDto(
                newPayloadRequest = NewPayloadRequestDto(
                  executionPayload = expectedExecutionPayloadDto(execution.executionPayload),
                  versionedHashes = emptyList(),
                  parentBeaconBlockRoot = ByteArray(32).encodeHex(),
                  executionRequests = execution.executionRequests.map { it.encodeHex() },
                ),
                executionWitness = ExecutionWitnessDto(
                  state = execution.executionWitness.state.map { it.encodeHex() },
                  codes = execution.executionWitness.codes.map { it.encodeHex() },
                  headers = execution.executionWitness.headers.map { it.encodeHex() },
                ),
              ),
              rollupExtension = RollupExtensionDto(
                forcedTransactions = listOf(
                  ForcedTransactionDto(
                    number = 17,
                    deadline = 1000600,
                    signedTxRlp = byteArrayOf(0x02).encodeHex(),
                    acceptance = ForcedTransactionAcceptance.INCLUDED,
                  ),
                ),
              ),
            ),
          ),
          chainConfig = ChainConfigDto(
            l2MessageServiceAddress = l2MessageServiceAddress,
            coinbase = coinbase,
            chainId = chainId,
            forkName = forkName,
          ),
        ),
        metadata = MetaDataDto(startBlockNumber = 1000501, endBlockNumber = 1000501),
      ),
    )
  }

  @Test
  fun `L2ExecutionProofRequestDtoMapper throws on unsupported forced-transaction inclusion result`() {
    val badRequest = l2Request(acceptance = ForcedTransactionInclusionResult.BadPrecompile)

    assertThatThrownBy {
      L2ExecutionProofRequestDtoMapper(
        guestProgramId,
        l2MessageServiceAddress,
        coinbase,
      ).invoke(badRequest)
    }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("Unsupported FTX inclusion result")
  }

  @Test
  fun `RestfulRollupProofRequestDtoMapper encodes every field`() {
    val l2Executions = listOf(blockIntervalProofIndex(1000501UL, 1000510UL))
    val blob = BlobWitness(
      startBlockNumber = 1000501UL,
      endBlockNumber = 1000510UL,
      blobHash = ByteArray(32) { 0x1a },
      blobKzgProof = ByteArray(48) { 0x1b },
      blockRlps = listOf(byteArrayOf(0x0c), byteArrayOf(0x0d)),
    )
    val request = RollupProofRequestV1(
      blobs = listOf(blob),
      parentShnarf = ByteArray(32) { 0x1c },
      endShnarf = ByteArray(32) { 0x1d },
      l2Executions = l2Executions,
    )

    val dto = RestfulRollupProofRequestDtoMapper(guestProgramId, chainId).invoke(request).get()

    assertThat(dto).isEqualTo(
      RestfulRollupProofRequestDto(
        guestProgramId = guestProgramId,
        proofRequest = RestfulRollupProofRequestParamsDto(
          chainId = chainId,
          blobs = listOf(
            BlobWitnessDto(
              startBlockNumber = 1000501,
              endBlockNumber = 1000510,
              blobHash = blob.blobHash.encodeHex(),
              blobKzgProof = blob.blobKzgProof.encodeHex(),
              blockRlps = blob.blockRlps.map { it.encodeHex() },
            ),
          ),
          parentShnarf = request.parentShnarf.encodeHex(),
          l2ExecutionProofIndexes = l2Executions,
        ),
        metadata = MetaDataDto(startBlockNumber = 1000501, endBlockNumber = 1000510),
      ),
    )
  }

  @Test
  fun `RestfulRollupAggregationProofRequestDtoMapper encodes every field`() {
    val rollupProofs = listOf(blockIntervalProofIndex(1000501UL, 1000520UL))
    val request = RollupAggregationProofRequestV1(rollupProofs = rollupProofs)

    val dto = RestfulRollupAggregationProofRequestDtoMapper(guestProgramId).invoke(request).get()

    assertThat(dto).isEqualTo(
      RestfulRollupAggregationProofRequestDto(
        guestProgramId = guestProgramId,
        proofRequest = RestfulRollupAggregationProofRequestParamsDto(rollupProofIndexes = rollupProofs),
        metadata = MetaDataDto(startBlockNumber = 1000501, endBlockNumber = 1000520),
      ),
    )
  }

  @Test
  fun `FileBasedRollupProofRequestDtoMapper inlines transport-resolved l2-execution proofs`() {
    val l2Executions = listOf(
      blockIntervalProofIndex(1000501UL, 1000510UL),
      blockIntervalProofIndex(1000511UL, 1000520UL),
    )
    val blob = BlobWitness(
      startBlockNumber = 1000501UL,
      endBlockNumber = 1000520UL,
      blobHash = ByteArray(32) { 0x1a },
      blobKzgProof = ByteArray(48) { 0x1b },
      blockRlps = listOf(byteArrayOf(0x0c), byteArrayOf(0x0d)),
    )
    val request = RollupProofRequestV1(
      blobs = listOf(blob),
      parentShnarf = ByteArray(32) { 0x1c },
      endShnarf = ByteArray(32) { 0x1d },
      l2Executions = l2Executions,
    )
    val l2ExecutionProofTransport = FakeL2ExecutionProofTransport()

    val dto = FileBasedRollupProofRequestDtoMapper(guestProgramId, chainId, l2ExecutionProofTransport)
      .invoke(request).get()

    assertThat(dto).isEqualTo(
      FileBasedRollupProofRequestDto(
        guestProgramId = guestProgramId,
        proofRequest = FileBasedRollupProofRequestParamsDto(
          chainId = chainId,
          blobs = listOf(
            BlobWitnessDto(
              startBlockNumber = 1000501,
              endBlockNumber = 1000520,
              blobHash = blob.blobHash.encodeHex(),
              blobKzgProof = blob.blobKzgProof.encodeHex(),
              blockRlps = blob.blockRlps.map { it.encodeHex() },
            ),
          ),
          parentShnarf = request.parentShnarf.encodeHex(),
          l2ExecutionProofs = l2Executions.map { proofIndex ->
            val resolved = l2ExecutionProofTransport.findResponse(proofIndex).get()!!
            L2ExecutionProofDto(
              proof = resolved.proof,
              startBlockNumber = resolved.startBlockNumber,
              publicInputs = resolved.publicInputs,
              l2L1Messages = resolved.l2L1Messages,
              txFroms = resolved.txFroms,
              filteredAddresses = resolved.filteredAddresses,
            )
          },
        ),
        metadata = MetaDataDto(startBlockNumber = 1000501, endBlockNumber = 1000520),
      ),
    )
  }

  @Test
  fun `FileBasedRollupAggregationProofRequestDtoMapper inlines transport-resolved rollup proofs`() {
    val rollupProofs = listOf(
      blockIntervalProofIndex(1000501UL, 1000520UL),
      blockIntervalProofIndex(1000521UL, 1000567UL),
    )
    val request = RollupAggregationProofRequestV1(rollupProofs = rollupProofs)
    val rollupProofTransport = FakeRollupProofTransport()

    val dto = FileBasedRollupAggregationProofRequestDtoMapper(guestProgramId, rollupProofTransport)
      .invoke(request).get()

    assertThat(dto).isEqualTo(
      FileBasedRollupAggregationProofRequestDto(
        guestProgramId = guestProgramId,
        proofRequest = FileBasedRollupAggregationProofRequestParamsDto(
          rollupProofs = rollupProofs.map { proofIndex ->
            val resolved = rollupProofTransport.findResponse(proofIndex).get()!!
            RollupProofDto(
              proof = resolved.proof,
              startBlockNumber = resolved.startBlockNumber,
              publicInputs = resolved.publicInputs,
              l2L1Roots = resolved.l2L1Roots,
              filteredAddresses = resolved.filteredAddresses,
            )
          },
        ),
        metadata = MetaDataDto(startBlockNumber = 1000501, endBlockNumber = 1000567),
      ),
    )
  }

  private fun l2Request(
    acceptance: ForcedTransactionInclusionResult = ForcedTransactionInclusionResult.Included,
  ): L2ExecutionProofRequestV1 = L2ExecutionProofRequestV1(
    executions = listOf(
      ExecutionInfo(
        blockNumber = 1000501UL,
        executionPayload = executionPayload(),
        executionWitness = ExecutionWitness(
          state = listOf(byteArrayOf(0x11)),
          codes = listOf(byteArrayOf(0x22)),
          headers = listOf(byteArrayOf(0x33)),
        ),
        executionRequests = emptyList(),
        forcedTransactions = listOf(
          ForcedTransaction(
            ftxNumber = 17UL,
            deadlineBlockNumber = 1000600UL,
            signedTxRlp = byteArrayOf(0x02),
            acceptance = acceptance,
          ),
        ),
      ),
    ),
    chainConfig = ChainConfig(
      chainId = chainId.toULong(),
      forkName = forkName,
    ),
    parentFtxRollingHash = ByteArray(32) { 0x1a },
    parentFtxNumber = 8UL,
  )

  private fun executionPayload(): ExecutionPayload = ExecutionPayload(
    parentHash = ByteArray(32) { 0x1a },
    feeRecipient = ByteArray(20) { 0x1b },
    stateRoot = ByteArray(32) { 0x1c },
    receiptsRoot = ByteArray(32) { 0x1d },
    logsBloom = ByteArray(256) { 0x1e },
    prevRandao = ByteArray(32) { 0x1f },
    blockNumber = 1000501UL,
    gasLimit = 60_000_000UL,
    gasUsed = 30_000_000UL,
    timestamp = 1763000123UL,
    extraData = byteArrayOf(0x01),
    baseFeePerGas = BigInteger.valueOf(7),
    blockHash = ByteArray(32) { 0x2a },
    transactions = listOf(byteArrayOf(0xde.toByte(), 0xad.toByte())),
    withdrawals = listOf(
      Withdrawal(index = 1UL, validatorIndex = 7UL, address = ByteArray(20) { 0x2b }, amount = 32UL),
    ),
    blobGasUsed = 0UL,
    excessBlobGas = 0UL,
    blockAccessList = byteArrayOf(),
  )

  private fun expectedExecutionPayloadDto(payload: ExecutionPayload): ExecutionPayloadDto = ExecutionPayloadDto(
    parentHash = payload.parentHash.encodeHex(),
    feeRecipient = payload.feeRecipient.encodeHex(),
    stateRoot = payload.stateRoot.encodeHex(),
    receiptsRoot = payload.receiptsRoot.encodeHex(),
    logsBloom = payload.logsBloom.encodeHex(),
    prevRandao = payload.prevRandao.encodeHex(),
    blockNumber = payload.blockNumber.toLong(),
    gasLimit = payload.gasLimit.toLong(),
    gasUsed = payload.gasUsed.toLong(),
    timestamp = payload.timestamp.toLong(),
    extraData = payload.extraData.encodeHex(),
    baseFeePerGas = payload.baseFeePerGas,
    blockHash = payload.blockHash.encodeHex(),
    transactions = payload.transactions.map { it.encodeHex() },
    withdrawals = payload.withdrawals.map {
      WithdrawalDto(
        index = it.index.toLong(),
        validatorIndex = it.validatorIndex.toLong(),
        address = it.address.encodeHex(),
        amount = it.amount.toLong(),
      )
    },
    blobGasUsed = payload.blobGasUsed.toLong(),
    excessBlobGas = payload.excessBlobGas.toLong(),
    blockAccessList = payload.blockAccessList.encodeHex(),
  )

  private fun blockIntervalProofIndex(start: ULong, end: ULong): BlockIntervalProofIndex = BlockIntervalProofIndex(
    startBlockNumber = start,
    endBlockNumber = end,
    startBlockTimestamp = Instant.fromEpochSeconds(1763000000),
    hash = ByteArray(32) { 1 },
  )
}

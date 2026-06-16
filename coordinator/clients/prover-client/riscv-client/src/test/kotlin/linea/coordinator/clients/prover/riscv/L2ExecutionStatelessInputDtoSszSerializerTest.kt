package linea.coordinator.clients.prover.riscv

import linea.clients.ChainConfig
import linea.clients.ExecutionPayload
import linea.clients.ExecutionRequests
import linea.clients.ExecutionWitness
import linea.clients.Withdrawal
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import java.math.BigInteger

class L2ExecutionStatelessInputDtoSszSerializerTest {
  private val dtoEncoder = L2ExecutionStatelessInputDtoSszSerializer()
  private val domainEncoder = L2ExecutionStatelessInputSszSerializer()

  @Test
  fun `encodes a stateless input dto to non-empty deterministic ssz bytes`() {
    val dto = statelessInputDto()

    assertThat(dtoEncoder.getStatelessInputDtoSsz(dto))
      .isNotEmpty()
      .isEqualTo(dtoEncoder.getStatelessInputDtoSsz(dto))
  }

  @Test
  fun `produces the same bytes as the domain encoder for equivalent input`() {
    val dto = statelessInputDto()
    val payload = dto.newPayloadRequest.executionPayload

    val domainBytes = domainEncoder.getStatelessInputSsz(
      executionPayload = ExecutionPayload(
        parentHash = payload.parentHash.decodeHex(),
        feeRecipient = payload.feeRecipient.decodeHex(),
        stateRoot = payload.stateRoot.decodeHex(),
        receiptsRoot = payload.receiptsRoot.decodeHex(),
        logsBloom = payload.logsBloom.decodeHex(),
        prevRandao = payload.prevRandao.decodeHex(),
        blockNumber = payload.blockNumber.toULong(),
        gasLimit = payload.gasLimit.toULong(),
        gasUsed = payload.gasUsed.toULong(),
        timestamp = payload.timestamp.toULong(),
        extraData = payload.extraData.decodeHex(),
        baseFeePerGas = payload.baseFeePerGas,
        blockHash = payload.blockHash.decodeHex(),
        transactions = payload.transactions.map { it.decodeHex() },
        withdrawals = payload.withdrawals.map {
          Withdrawal(
            index = it.index.toULong(),
            validatorIndex = it.validatorIndex.toULong(),
            address = it.address.decodeHex(),
            amount = it.amount.toULong(),
          )
        },
        blobGasUsed = payload.blobGasUsed.toULong(),
        excessBlobGas = payload.excessBlobGas.toULong(),
        blockAccessList = payload.blockAccessList.decodeHex(),
      ),
      executionWitness = ExecutionWitness(
        blockNumber = payload.blockNumber.toULong(),
        state = dto.executionWitness.state.map { it.decodeHex() },
        keys = emptyList(),
        codes = dto.executionWitness.codes.map { it.decodeHex() },
        headers = dto.executionWitness.headers.map { it.decodeHex() },
      ),
      chainConfig = ChainConfig(
        l2MessageServiceContract = ByteArray(20),
        coinbase = ByteArray(20),
        chainId = dto.chainConfig.chainId.toULong(),
      ),
      versionedHashes = emptyList(),
      parentBeaconBlockRoot = ByteArray(32),
      executionRequests = ExecutionRequests(
        blockNumber = payload.blockNumber.toULong(),
        deposits = emptyList(),
        withdrawals = emptyList(),
        consolidations = emptyList(),
      ),
      publicKey = emptyList(),
    )

    assertThat(dtoEncoder.getStatelessInputDtoSsz(dto)).isEqualTo(domainBytes.encodeHex())
  }

  private fun statelessInputDto(): StatelessInputDto = StatelessInputDto(
    newPayloadRequest = NewPayloadRequestDto(
      executionPayload = ExecutionPayloadDto(
        parentHash = hex(0x0a, 32),
        feeRecipient = hex(0x0b, 20),
        stateRoot = hex(0x0c, 32),
        receiptsRoot = hex(0x0d, 32),
        logsBloom = hex(0x0e, 256),
        prevRandao = hex(0x0f, 32),
        blockNumber = 1000501,
        gasLimit = 60_000_000,
        gasUsed = 30_000_000,
        timestamp = 1763000123,
        extraData = hex(0x01, 8),
        baseFeePerGas = BigInteger.valueOf(7),
        blockHash = hex(0x10, 32),
        transactions = listOf("0xdeadbeef", "0xcafe"),
        withdrawals = listOf(
          WithdrawalDto(index = 1, validatorIndex = 7, address = hex(0x44, 20), amount = 32_000_000_000),
        ),
        blobGasUsed = 0,
        excessBlobGas = 0,
        blockAccessList = "0x",
      ),
      // assumed empty
      versionedHashes = emptyList(),
      parentBeaconBlockRoot = hex(0x00, 32),
      executionRequests = ExecutionRequestsDto(
        deposits = emptyList(),
        withdrawals = emptyList(),
        consolidations = emptyList(),
      ),
    ),
    executionWitness = ExecutionWitnessDto(
      state = listOf(hex(0x11, 4)),
      keys = listOf(hex(0x99, 4)), // dropped by the SSZ schema
      codes = listOf(hex(0x22, 8)),
      headers = listOf(hex(0x33, 16)),
    ),
    chainConfig = StatelessChainConfigDto(chainId = 59144, forkName = "amsterdam"),
    publicKeys = emptyList(),
  )

  private fun hex(byteValue: Int, length: Int): String =
    "0x" + "%02x".format(byteValue).repeat(length)
}

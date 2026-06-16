package linea.coordinator.clients.prover.riscv

import linea.clients.ChainConfig
import linea.clients.ExecutionPayload
import linea.clients.ExecutionRequests
import linea.clients.ExecutionWitness
import linea.clients.Withdrawal
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import java.math.BigInteger

class L2ExecutionStatelessInputSszSerializerTest {
  private val encoder = L2ExecutionStatelessInputSszSerializer()

  @Test
  fun `encodes a stateless input to non-empty deterministic ssz bytes`() {
    val first = encode(chainId = 59144UL)
    val second = encode(chainId = 59144UL)

    assertThat(first).isNotEmpty()
    assertThat(first).isEqualTo(second)
  }

  @Test
  fun `different inputs produce different ssz bytes`() {
    assertThat(encode(chainId = 59144UL)).isNotEqualTo(encode(chainId = 1UL))
  }

  @Test
  fun `withdrawals are encoded into the ssz bytes`() {
    val withoutWithdrawals = encode(chainId = 59144UL, withdrawals = emptyList())
    val withWithdrawals = encode(
      chainId = 59144UL,
      withdrawals = listOf(
        Withdrawal(index = 1UL, validatorIndex = 7UL, address = ByteArray(20) { 0x44 }, amount = 32_000_000_000UL),
      ),
    )

    assertThat(withWithdrawals).isNotEqualTo(withoutWithdrawals)
  }

  private fun encode(chainId: ULong, withdrawals: List<Withdrawal> = emptyList()): ByteArray {
    return encoder.getStatelessInputSsz(
      executionPayload = executionPayload(withdrawals),
      executionWitness = executionWitness(),
      chainConfig = ChainConfig(
        l2MessageServiceContract = ByteArray(20) { 1 },
        coinbase = ByteArray(20) { 2 },
        chainId = chainId,
      ),
      // assumed empty for now
      versionedHashes = emptyList(),
      parentBeaconBlockRoot = ByteArray(0),
      executionRequests = ExecutionRequests(
        blockNumber = 1000501UL,
        deposits = emptyList(),
        withdrawals = emptyList(),
        consolidations = emptyList(),
      ),
      publicKey = emptyList(),
    )
  }

  private fun executionPayload(withdrawals: List<Withdrawal> = emptyList()): ExecutionPayload = ExecutionPayload(
    parentHash = ByteArray(32) { 0x0a },
    feeRecipient = ByteArray(20) { 0x0b },
    stateRoot = ByteArray(32) { 0x0c },
    receiptsRoot = ByteArray(32) { 0x0d },
    logsBloom = ByteArray(256) { 0x0e },
    prevRandao = ByteArray(32) { 0x0f },
    blockNumber = 1000501UL,
    gasLimit = 60_000_000UL,
    gasUsed = 30_000_000UL,
    timestamp = 1763000123UL,
    extraData = ByteArray(8) { 0x01 },
    baseFeePerGas = BigInteger.valueOf(7),
    blockHash = ByteArray(32) { 0x10 },
    transactions = listOf("0xdeadbeef".hexToBytes(), "0xcafe".hexToBytes()),
    withdrawals = withdrawals,
    blobGasUsed = 0UL,
    excessBlobGas = 0UL,
    blockAccessList = ByteArray(0),
  )

  private fun executionWitness(): ExecutionWitness = ExecutionWitness(
    blockNumber = 1000501UL,
    state = listOf(ByteArray(4) { 0x11 }),
    keys = emptyList(),
    codes = listOf(ByteArray(8) { 0x22 }),
    headers = listOf(ByteArray(16) { 0x33 }),
  )

  private fun String.hexToBytes(): ByteArray {
    val clean = removePrefix("0x")
    return ByteArray(clean.length / 2) { clean.substring(it * 2, it * 2 + 2).toInt(16).toByte() }
  }
}

package linea.contract.l2

import linea.domain.BlockParameter
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test

class FakeL2MessageServiceTest {

  private lateinit var fakeL2MessageService: FakeL2MessageService
  private val testRollingHash = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa".decodeHex()
  private val testMessageHashes = listOf(
    "0x1111111111111111111111111111111111111111111111111111111111111111".decodeHex(),
    "0x2222222222222222222222222222222222222222222222222222222222222222".decodeHex(),
    "0x3333333333333333333333333333333333333333333333333333333333333333".decodeHex()
  )

  @BeforeEach
  fun setUp() {
    fakeL2MessageService = FakeL2MessageService()
  }

  @Test
  fun `should set and get last anchored L1 message`() {
    fakeL2MessageService.setLastAnchoredL1Message(1UL, testRollingHash)

    val lastMessageNumber = fakeL2MessageService.getLastAnchoredL1MessageNumber(BlockParameter.Tag.LATEST).get()
    val lastRollingHash = fakeL2MessageService.getLastAnchoredRollingHash()

    assertThat(lastMessageNumber).isEqualTo(1UL)
    assertThat(lastRollingHash).isEqualTo(testRollingHash)
  }

  @Test
  fun `should anchor L1-L2 message hashes`() {
    fakeL2MessageService.setLastAnchoredL1Message(0UL, ByteArray(0))

    fakeL2MessageService.anchorL1L2MessageHashes(
      messageHashes = testMessageHashes,
      startingMessageNumber = 1UL,
      finalMessageNumber = 3UL,
      finalRollingHash = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa".decodeHex()
    ).get()
      .also { txHash ->
        val lastMessageNumber = fakeL2MessageService.getLastAnchoredL1MessageNumber(BlockParameter.Tag.LATEST).get()
        val lastRollingHash = fakeL2MessageService.getLastAnchoredRollingHash()
        val anchoredMessages = fakeL2MessageService.getAnchoredMessageHashes()

        assertThat(txHash).isNotEmpty()
        assertThat(lastMessageNumber).isEqualTo(3UL)
        assertThat(lastRollingHash)
          .isEqualTo("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa".decodeHex())
        assertThat(anchoredMessages).isEqualTo(testMessageHashes)
      }

    fakeL2MessageService.anchorL1L2MessageHashes(
      messageHashes = listOf(
        "0x4444444444444444444444444444444444444444444444444444444444444444".decodeHex(),
        "0x5555555555555555555555555555555555555555555555555555555555555555".decodeHex()
      ),
      startingMessageNumber = 4UL,
      finalMessageNumber = 5UL,
      finalRollingHash = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaB".decodeHex()
    ).get()
      .also { txHash ->
        val lastMessageNumber = fakeL2MessageService.getLastAnchoredL1MessageNumber(BlockParameter.Tag.LATEST).get()
        val lastRollingHash = fakeL2MessageService.getLastAnchoredRollingHash()
        val anchoredMessages = fakeL2MessageService.getAnchoredMessageHashes()

        assertThat(txHash).isNotEmpty()
        assertThat(lastMessageNumber).isEqualTo(5UL)
        assertThat(lastRollingHash).isEqualTo(
          "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaB".decodeHex()
        )
        assertThat(anchoredMessages.map { it.encodeHex() }).isEqualTo(
          listOf(
            "0x1111111111111111111111111111111111111111111111111111111111111111",
            "0x2222222222222222222222222222222222222222222222222222222222222222",
            "0x3333333333333333333333333333333333333333333333333333333333333333",
            "0x4444444444444444444444444444444444444444444444444444444444444444",
            "0x5555555555555555555555555555555555555555555555555555555555555555"
          )
        )
      }
  }

  @Test
  fun `should get rolling hash by L1 message number`() {
    fakeL2MessageService.setLastAnchoredL1Message(1UL, testRollingHash)

    val rollingHash = fakeL2MessageService.getRollingHashByL1MessageNumber(BlockParameter.Tag.LATEST, 1UL).get()

    assertThat(rollingHash).isEqualTo(testRollingHash)
  }

  @Test
  fun `should return empty rolling hash for unknown L1 message number`() {
    val rollingHash = fakeL2MessageService.getRollingHashByL1MessageNumber(BlockParameter.Tag.LATEST, 99UL).get()

    assertThat(rollingHash).isEqualTo(ByteArray(32))
  }
}

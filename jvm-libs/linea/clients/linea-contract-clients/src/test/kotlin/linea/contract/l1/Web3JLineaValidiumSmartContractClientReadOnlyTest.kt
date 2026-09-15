package linea.contract.l1

import linea.domain.BlockParameter
import net.consensys.FakeFixedClock
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import org.mockito.kotlin.mock
import org.web3j.protocol.Web3j
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration.Companion.seconds

class Web3JLineaValidiumSmartContractClientReadOnlyTest {
  private val contractAddress = "0x" + "aa".repeat(20)

  private val client = Web3JLineaValidiumSmartContractClientReadOnly(
    web3j = mock<Web3j>(), // not used by parseContractVersion
    contractAddress = contractAddress,
  )

  @Test
  fun `parseContractVersion maps major version prefixes and rejects unsupported versions`() {
    assertThat(client.parseContractVersion("1.0")).isEqualTo(LineaValidiumContractVersion.V1)
    assertThat(client.parseContractVersion("2.0")).isEqualTo(LineaValidiumContractVersion.V2)
    assertThat(client.parseContractVersion("2.1.0-rc.1")).isEqualTo(LineaValidiumContractVersion.V2)

    assertThatThrownBy { client.parseContractVersion("3.0") }
      .isInstanceOf(IllegalStateException::class.java)
      .hasMessageContaining("Unsupported Validium contract version: 3.0")

    // "10.0" must not be misread as V1 by a bare "1" prefix. It is unsupported, so reject it.
    assertThatThrownBy { client.parseContractVersion("10.0") }
      .isInstanceOf(IllegalStateException::class.java)
      .hasMessageContaining("Unsupported Validium contract version: 10.0")
  }

  @Test
  fun `getVersion caches below-latest versions for the refresh interval and short-circuits at latest`() {
    val fakeClock = FakeFixedClock()
    var onChainVersion = LineaValidiumContractVersion.V1
    var fetchCount = 0
    val fakeClient = object : Web3JLineaValidiumSmartContractClientReadOnly(
      web3j = mock<Web3j>(),
      contractAddress = contractAddress,
      versionRefreshInterval = 30.seconds,
      clock = fakeClock,
    ) {
      override fun fetchSmartContractVersion(
        blockParameter: BlockParameter,
      ): SafeFuture<LineaValidiumContractVersion> {
        fetchCount++
        return SafeFuture.completedFuture(onChainVersion)
      }
    }

    // first call fetches and caches
    assertThat(fakeClient.getVersion().get()).isEqualTo(LineaValidiumContractVersion.V1)
    assertThat(fetchCount).isEqualTo(1)

    // within the refresh interval: served from cache, no RPC
    fakeClock.advanceBy(29.seconds)
    assertThat(fakeClient.getVersion().get()).isEqualTo(LineaValidiumContractVersion.V1)
    assertThat(fetchCount).isEqualTo(1)

    // after the refresh interval: refetches and detects the upgrade
    fakeClock.advanceBy(2.seconds)
    onChainVersion = LineaValidiumContractVersion.V2
    assertThat(fakeClient.getVersion().get()).isEqualTo(LineaValidiumContractVersion.V2)
    assertThat(fetchCount).isEqualTo(2)

    // at the latest known version: short-circuits forever, even after the interval elapses
    fakeClock.advanceBy(300.seconds)
    assertThat(fakeClient.getVersion().get()).isEqualTo(LineaValidiumContractVersion.V2)
    assertThat(fetchCount).isEqualTo(2)
  }

  @Test
  fun `getFinalizedStateData reports the finalized block with a zero forced-transaction number`() {
    // V1 has no forced transactions; V2 has them on-chain but the coordinator does not support them
    // on validium yet, so both report the initial value (0) until that support lands.
    LineaValidiumContractVersion.entries.forEach { version ->
      val fakeClient = object : Web3JLineaValidiumSmartContractClientReadOnly(
        web3j = mock<Web3j>(),
        contractAddress = contractAddress,
      ) {
        override fun fetchSmartContractVersion(
          blockParameter: BlockParameter,
        ): SafeFuture<LineaValidiumContractVersion> = SafeFuture.completedFuture(version)

        override fun finalizedL2BlockNumber(blockParameter: BlockParameter): SafeFuture<ULong> =
          SafeFuture.completedFuture(42UL)
      }

      assertThat(fakeClient.getFinalizedStateData(BlockParameter.Tag.LATEST).get())
        .isEqualTo(
          FinalizedStateDataProvider.FinalizedStateData(
            blockNumber = 42UL,
            forcedTransactionNumber = 0UL,
          ),
        )
    }
  }
}

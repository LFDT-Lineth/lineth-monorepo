package linea.web3j.domain

import linea.domain.BlockParameter
import linea.kotlin.toBigInteger
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import org.web3j.protocol.core.DefaultBlockParameter
import org.web3j.protocol.core.DefaultBlockParameterName

class BlockParameterExtensionsTest {

  @Test
  fun `toWeb3j should map tags and block numbers`() {
    assertThat(BlockParameter.Tag.LATEST.toWeb3j()).isEqualTo(DefaultBlockParameterName.LATEST)
    assertThat(BlockParameter.BlockNumber(42UL).toWeb3j().getValue())
      .isEqualTo(DefaultBlockParameter.valueOf(42.toBigInteger()).getValue())
  }

  @Test
  fun `toWeb3j should reject block hash`() {
    val blockHash = BlockParameter.fromHash(ByteArray(32) { 1 })
    assertThatThrownBy { blockHash.toWeb3j() }
      .isInstanceOf(UnsupportedOperationException::class.java)
      .hasMessageContaining("does not support block hash")
  }
}

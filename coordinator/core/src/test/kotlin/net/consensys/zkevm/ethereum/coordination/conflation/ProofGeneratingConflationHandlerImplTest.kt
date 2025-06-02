package net.consensys.zkevm.ethereum.coordination.conflation

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.getError
import linea.domain.createBlock
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class ProofGeneratingConflationHandlerImplTest {

  @Test fun `assertConsecutiveBlocksRange return error when empty`() {
    assertConsecutiveBlocksRange(emptyList()).let { result ->
      assertThat(result).isInstanceOf(Err::class.java)
      assertThat(result.getError()).isInstanceOf(IllegalArgumentException::class.java)
      assertThat(result.getError()!!.message).isEqualTo("Empty list of blocks")
    }
  }

  @Test fun `assertConsecutiveBlocksRange return error when there is gap in block numbers`() {
    val blocks = listOf(
      createBlock(15UL),
      createBlock(14UL),
      // // gap on 13
      createBlock(12UL),
      createBlock(11UL),
      createBlock(10UL),
    )
    assertConsecutiveBlocksRange(blocks).let { result ->
      assertThat(result).isInstanceOf(Err::class.java)
      assertThat(result.getError()).isInstanceOf(IllegalArgumentException::class.java)
      assertThat(result.getError()!!.message).isEqualTo("Conflated blocks list has non consecutive blocks!")
    }
  }

  @Test fun `assertConsecutiveBlocksRange returns sorted list when all blocks are consecutive`() {
    val blocks = listOf(
      createBlock(13UL),
      createBlock(12UL),
      createBlock(11UL),
      createBlock(10UL),
    )

    assertThat(assertConsecutiveBlocksRange(blocks)).isEqualTo(Ok(10UL..13UL))
    assertThat(assertConsecutiveBlocksRange(listOf(blocks[0]))).isEqualTo(Ok(13UL..13UL))
  }
}

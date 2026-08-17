/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.syncing.beaconchain.pipeline

import maru.consensus.blockimport.SealedBeaconBlockImporter
import maru.core.SealedBeaconBlock
import maru.core.ext.DataGenerators
import maru.p2p.MaruPeer
import maru.p2p.ValidationResult
import maru.p2p.ValidationResult.Companion.Ignore
import maru.p2p.ValidationResult.Companion.Invalid
import maru.p2p.ValidationResult.Companion.Valid
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.Mockito.mock
import org.mockito.Mockito.never
import org.mockito.Mockito.times
import org.mockito.kotlin.any
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CompletionException

class ImportBlocksStepTest {
  private lateinit var blockImporter: SealedBeaconBlockImporter<ValidationResult>
  private lateinit var importBlocksStep: ImportBlocksStep

  @BeforeEach
  fun setUp() {
    blockImporter = mock()
    importBlocksStep = ImportBlocksStep(blockImporter)
  }

  @Test
  fun `successfully imports all blocks when all are accepted`() {
    val block1 = DataGenerators.randomSealedBeaconBlock(
      1u,
    )
    val block2 = DataGenerators.randomSealedBeaconBlock(
      2u,
    )
    val block3 = DataGenerators.randomSealedBeaconBlock(
      3u,
    )
    val blocks = listOf(block1, block2, block3)

    whenever(blockImporter.importBlock(any())).thenReturn(SafeFuture.completedFuture(Valid))

    importBlocksStep.accept(createSealedBlocksWithPeers(blocks))

    verify(blockImporter, times(3)).importBlock(any())
    verify(blockImporter).importBlock(block1)
    verify(blockImporter).importBlock(block2)
    verify(blockImporter).importBlock(block3)
  }

  @Test
  fun `does nothing when empty list is provided`() {
    importBlocksStep.accept(emptyList())

    verify(blockImporter, never()).importBlock(any())
  }

  @Test
  fun `stops processing on REJECT result`() {
    val block1 = DataGenerators.randomSealedBeaconBlock(
      1u,
    )
    val block2 = DataGenerators.randomSealedBeaconBlock(
      2u,
    )
    val block3 = DataGenerators.randomSealedBeaconBlock(
      3u,
    )
    val blocks = listOf(block1, block2, block3)

    whenever(blockImporter.importBlock(block1)).thenReturn(SafeFuture.completedFuture(Valid))
    whenever(
      blockImporter.importBlock(block2),
    ).thenReturn(SafeFuture.completedFuture(Invalid("Block validation failed")))
    whenever(blockImporter.importBlock(block3)).thenReturn(SafeFuture.completedFuture(Valid))

    importBlocksStep.accept(createSealedBlocksWithPeers(blocks))

    verify(blockImporter).importBlock(block1)
    verify(blockImporter).importBlock(block2)
    verify(blockImporter, never()).importBlock(block3)
  }

  @Test
  fun `stops processing on IGNORE result`() {
    val block1 = DataGenerators.randomSealedBeaconBlock(
      1u,
    )
    val block2 = DataGenerators.randomSealedBeaconBlock(
      2u,
    )
    val block3 = DataGenerators.randomSealedBeaconBlock(
      3u,
    )
    val blocks = listOf(block1, block2, block3)

    whenever(blockImporter.importBlock(block1)).thenReturn(SafeFuture.completedFuture(Valid))
    whenever(blockImporter.importBlock(block2)).thenReturn(
      SafeFuture.completedFuture(
        Ignore(
          "Block validation ignored",
        ),
      ),
    )
    whenever(blockImporter.importBlock(block3)).thenReturn(SafeFuture.completedFuture(Valid))

    importBlocksStep.accept(createSealedBlocksWithPeers(blocks))

    verify(blockImporter).importBlock(block1)
    verify(blockImporter).importBlock(block2)
    verify(blockImporter, never()).importBlock(block3)
  }

  @Test
  fun `propagates exception when block import fails`() {
    val block1 = DataGenerators.randomSealedBeaconBlock(
      1u,
    )
    val blocks = listOf(block1)

    val expectedException = RuntimeException("Import failed")
    whenever(blockImporter.importBlock(block1)).thenReturn(
      SafeFuture.failedFuture(expectedException),
    )

    assertThatThrownBy { importBlocksStep.accept(createSealedBlocksWithPeers(blocks)) }
      .isInstanceOf(CompletionException::class.java)
      .hasCauseInstanceOf(RuntimeException::class.java)
      .hasRootCauseMessage("Import failed")

    verify(blockImporter).importBlock(block1)
  }

  @Test
  fun `processes blocks sequentially in order`() {
    val block1 = DataGenerators.randomSealedBeaconBlock(
      1u,
    )
    val block2 = DataGenerators.randomSealedBeaconBlock(
      2u,
    )
    val block3 = DataGenerators.randomSealedBeaconBlock(
      3u,
    )
    val blocks = listOf(block1, block2, block3)

    val acceptResult = Valid
    val completedBlocks = mutableListOf<SealedBeaconBlock>()

    whenever(blockImporter.importBlock(any())).thenAnswer { invocation ->
      val block = invocation.getArgument<SealedBeaconBlock>(0)
      completedBlocks.add(block)
      SafeFuture.completedFuture(acceptResult)
    }

    importBlocksStep.accept(createSealedBlocksWithPeers(blocks))

    assertThat(completedBlocks).containsExactly(block1, block2, block3)
  }

  @Test
  fun `handles mixed results correctly`() {
    val block1 = DataGenerators.randomSealedBeaconBlock(
      1u,
    )
    val block2 = DataGenerators.randomSealedBeaconBlock(
      2u,
    )
    val block3 = DataGenerators.randomSealedBeaconBlock(
      3u,
    )
    val block4 = DataGenerators.randomSealedBeaconBlock(
      4u,
    )
    val blocks = listOf(block1, block2, block3, block4)

    whenever(blockImporter.importBlock(block1)).thenReturn(SafeFuture.completedFuture(Valid))
    whenever(blockImporter.importBlock(block2)).thenReturn(SafeFuture.completedFuture(Valid))
    whenever(
      blockImporter.importBlock(block3),
    ).thenReturn(SafeFuture.completedFuture(Ignore("Block ignored for mixed results test")))
    whenever(blockImporter.importBlock(block4)).thenReturn(SafeFuture.completedFuture(Valid))

    importBlocksStep.accept(createSealedBlocksWithPeers(blocks))

    verify(blockImporter).importBlock(block1)
    verify(blockImporter).importBlock(block2)
    verify(blockImporter).importBlock(block3)
    verify(blockImporter, never()).importBlock(block4)
  }

  private fun createSealedBlocksWithPeers(blocks: List<SealedBeaconBlock>): List<SealedBlockWithPeer> =
    blocks
      .stream()
      .map({
        SealedBlockWithPeer(it, mock(MaruPeer::class.java))
      })
      .toList()
}

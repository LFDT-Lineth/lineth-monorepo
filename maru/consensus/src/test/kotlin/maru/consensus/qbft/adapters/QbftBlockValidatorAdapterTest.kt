/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft.adapters

import com.github.michaelbull.result.Result
import com.github.michaelbull.result.getError
import maru.consensus.validation.BlockValidator
import maru.core.BeaconBlock
import maru.core.ext.DataGenerators
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlockValidator
import org.junit.jupiter.api.Test
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.Optional

class QbftBlockValidatorAdapterTest {
  private val newBlock = DataGenerators.randomBeaconBlock(
    10u,
  )
  private lateinit var blockValidator: BlockValidator

  @Test
  fun `validateBlock should return false when block validation error`() {
    val blockValidatorError = BlockValidator.error("Error")
    blockValidator = object : BlockValidator {
      override fun validateBlock(block: BeaconBlock): SafeFuture<Result<Unit, BlockValidator.BlockValidationError>> =
        SafeFuture.completedFuture(blockValidatorError)
    }
    val qbftBlockValidatorAdapter =
      QbftBlockValidatorAdapter(
        blockValidator = blockValidator,
      )
    val expectedResult =
      QbftBlockValidator.ValidationResult(
        false,
        Optional.of(blockValidatorError.getError().toString()),
      )
    assertThat(
      qbftBlockValidatorAdapter.validateBlock(QbftBlockAdapter(newBlock), Optional.empty()),
    ).isEqualTo(expectedResult)
  }

  @Test
  fun `validateBlock should return true when valid block`() {
    blockValidator = object : BlockValidator {
      override fun validateBlock(block: BeaconBlock): SafeFuture<Result<Unit, BlockValidator.BlockValidationError>> =
        SafeFuture.completedFuture(BlockValidator.ok())
    }

    val qbftBlockValidatorAdapter =
      QbftBlockValidatorAdapter(
        blockValidator = blockValidator,
      )
    val expectedResult = QbftBlockValidator.ValidationResult(true, Optional.empty())
    assertThat(
      qbftBlockValidatorAdapter.validateBlock(QbftBlockAdapter(newBlock), Optional.empty()),
    ).isEqualTo(expectedResult)
  }
}

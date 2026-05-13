/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus

import maru.core.Validator
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.SequencedSet

/**
 * Provides access to the set of validators for a given block.
 */
interface ValidatorProvider {
  fun getValidatorsAfterBlock(blockNumber: ULong): SafeFuture<SequencedSet<Validator>> =
    getValidatorsForBlock(
      blockNumber + 1u,
    )

  fun getValidatorsForBlock(blockNumber: ULong): SafeFuture<SequencedSet<Validator>>
}

/**
 * A [ValidatorProvider] that always returns the same [Validator] instance.
 * This is useful for the single validator case.
 */
class StaticValidatorProvider(
  validators: Set<Validator>,
) : ValidatorProvider {
  private val validators: SequencedSet<Validator> = validators.toSortedSet()

  override fun getValidatorsForBlock(blockNumber: ULong): SafeFuture<SequencedSet<Validator>> =
    SafeFuture.completedFuture(validators)
}

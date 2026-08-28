/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import linea.kotlin.encodeHex
import maru.config.QbftConfig
import maru.config.ValidatorSignerType
import maru.consensus.DifficultyAwareQbftConfig
import maru.consensus.ForkSpec
import maru.consensus.ForksSchedule
import maru.consensus.QbftConsensusConfig
import maru.core.Validator

internal object ValidatorSetMembership {
  fun validate(
    qbftConfig: QbftConfig,
    beaconGenesisConfig: ForksSchedule,
    validator: Validator,
  ) {
    val validatorsFromAllForks: Set<Validator> =
      beaconGenesisConfig.forks
        .flatMap<ForkSpec, Validator> {
          when (val configuration = it.configuration) {
            is DifficultyAwareQbftConfig -> configuration.postTtdConfig.validatorSet
            is QbftConsensusConfig -> configuration.validatorSet
            else ->
              throw IllegalArgumentException(
                "Unsupported consensus configuration: ${configuration::class.qualifiedName}",
              )
          }
        }.toSet()

    if (validator !in validatorsFromAllForks) {
      val signerDescription =
        when (qbftConfig.validatorSigner.type) {
          ValidatorSignerType.LOCAL -> "Local validator signer"
          ValidatorSignerType.CUSTOM ->
            "Custom validator signer '${qbftConfig.validatorSigner.name}'"
        }
      throw IllegalArgumentException(
        "$signerDescription address ${validator.address.encodeHex()} " +
          "is not present in any configured validator set",
      )
    }
  }
}

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
import org.apache.logging.log4j.LogManager

internal object ValidatorSetMembership {
  private val log = LogManager.getLogger(ValidatorSetMembership::class.java)

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
      if (qbftConfig.validatorSigner.type == ValidatorSignerType.CUSTOM) {
        throw IllegalArgumentException(
          "Custom validator signer '${qbftConfig.validatorSigner.name}' address " +
            "${validator.address.encodeHex()} is not present in any configured validator set",
        )
      }
      log.warn(
        "Local validator={} is not present in any validator set in the genesis file",
        validator,
      )
    }
  }
}

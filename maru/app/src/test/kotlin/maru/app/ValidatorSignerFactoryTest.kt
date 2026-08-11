/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import linea.crypto.withCloseAction
import maru.config.QbftConfig
import maru.config.ValidatorSignerConfig
import maru.config.ValidatorSignerType
import maru.consensus.ChainFork
import maru.consensus.ClFork
import maru.consensus.ElFork
import maru.consensus.ForkSpec
import maru.consensus.ForksSchedule
import maru.consensus.QbftConsensusConfig
import maru.core.Validator
import maru.crypto.LocalValidatorSigner
import maru.crypto.SecpCrypto
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test

class ValidatorSignerFactoryTest {
  private val privateKey = ByteArray(32).also { it[it.lastIndex] = 1 }
  private val localSignerConfig = ValidatorSignerConfig()
  private val customSignerConfig =
    ValidatorSignerConfig(ValidatorSignerType.CUSTOM, "maru-validator")

  @Test
  fun `missing custom signer factory reports the logical name`() {
    assertThatThrownBy {
      MissingCustomValidatorSignerFactory.create(customSignerConfig)
    }.isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("maru-validator")
  }

  @Test
  fun `factory creates a local signer without invoking the custom factory`() {
    val factory =
      ValidatorSignerFactory(
        CustomValidatorSignerFactory { error("must not be called") },
      )
    val expectedValidator = SecpCrypto.privateKeyToValidator(privateKey)

    val signer =
      factory.create(
        qbftConfig = qbftConfig(localSignerConfig),
        beaconGenesisConfig = forkSchedule(setOf(expectedValidator)),
        privateKey = privateKey,
      )

    assertThat(signer.toValidator()).isEqualTo(expectedValidator)
  }

  @Test
  fun `factory creates a custom signer matching the validator set`() {
    var receivedConfig: ValidatorSignerConfig? = null
    var closeCalls = 0
    val localSigner = LocalValidatorSigner(privateKey)
    val factory =
      ValidatorSignerFactory(
        CustomValidatorSignerFactory { config ->
          receivedConfig = config
          localSigner.withCloseAction { closeCalls++ }
        },
      )

    val signer =
      factory.create(
        qbftConfig = qbftConfig(customSignerConfig),
        beaconGenesisConfig = forkSchedule(setOf(SecpCrypto.privateKeyToValidator(privateKey))),
        privateKey = ByteArray(0),
      )

    assertThat(receivedConfig).isEqualTo(customSignerConfig)
    assertThat(signer.toValidator()).isEqualTo(SecpCrypto.privateKeyToValidator(privateKey))

    signer.close()
    signer.close()
    assertThat(closeCalls).isEqualTo(1)
  }

  @Test
  fun `factory closes a custom signer rejected by validator set validation`() {
    var closeCalls = 0
    val factory =
      ValidatorSignerFactory(
        CustomValidatorSignerFactory {
          LocalValidatorSigner(privateKey).withCloseAction { closeCalls++ }
        },
      )
    val differentValidator =
      SecpCrypto.privateKeyToValidator(
        Bytes32
          .fromHexString("0x02")
          .toArray(),
      )

    assertThatThrownBy {
      factory.create(
        qbftConfig = qbftConfig(customSignerConfig),
        beaconGenesisConfig = forkSchedule(setOf(differentValidator)),
        privateKey = ByteArray(0),
      )
    }.isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("maru-validator")
      .hasMessageContaining("is not present in any configured validator set")

    assertThat(closeCalls).isEqualTo(1)
  }

  @Test
  fun `factory preserves warning-only behavior for a local signer outside the validator set`() {
    val factory =
      ValidatorSignerFactory(
        CustomValidatorSignerFactory { error("must not be called") },
      )
    val differentValidator =
      SecpCrypto.privateKeyToValidator(
        Bytes32
          .fromHexString("0x02")
          .toArray(),
      )

    val signer =
      factory.create(
        qbftConfig = qbftConfig(localSignerConfig),
        beaconGenesisConfig = forkSchedule(setOf(differentValidator)),
        privateKey = privateKey,
      )

    assertThat(signer.toValidator()).isEqualTo(SecpCrypto.privateKeyToValidator(privateKey))
  }

  private fun qbftConfig(signerConfig: ValidatorSignerConfig) =
    QbftConfig(
      feeRecipient = ByteArray(20),
      validatorSigner = signerConfig,
    )

  private fun forkSchedule(validators: Set<Validator>) =
    ForksSchedule(
      chainId = 1u,
      forks =
      setOf(
        ForkSpec(
          timestampSeconds = 0uL,
          blockTimeSeconds = 1u,
          configuration =
          QbftConsensusConfig(
            validatorSet = validators,
            fork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Prague),
          ),
        ),
      ),
    )
}

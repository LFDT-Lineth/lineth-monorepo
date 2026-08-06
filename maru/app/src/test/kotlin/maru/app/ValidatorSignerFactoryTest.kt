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
import maru.config.ApiConfig
import maru.config.ApiEndpointConfig
import maru.config.FollowersConfig
import maru.config.ForkTransition
import maru.config.MaruConfig
import maru.config.ObservabilityConfig
import maru.config.Persistence
import maru.config.QbftConfig
import maru.config.SyncingConfig
import maru.config.ValidatorElNode
import maru.config.ValidatorSignerConfig
import maru.config.ValidatorSignerType
import maru.consensus.ChainFork
import maru.consensus.ClFork
import maru.consensus.DifficultyAwareQbftConfig
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
import org.junit.jupiter.api.io.TempDir
import java.net.URI
import java.nio.file.Path
import java.time.Clock
import java.time.Instant
import java.time.ZoneOffset
import kotlin.time.Duration.Companion.seconds

class ValidatorSignerFactoryTest {
  @TempDir
  lateinit var tempDir: Path

  private val privateKey = ByteArray(32).also { it[it.lastIndex] = 1 }
  private val customSignerConfig =
    ValidatorSignerConfig(ValidatorSignerType.CUSTOM, "maru-validator")

  @Test
  fun `default factory rejects a custom signer with its logical name`() {
    assertThatThrownBy {
      DefaultValidatorSignerFactory.create(
        customSignerConfig,
      )
    }.isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("maru-validator")
  }

  @Test
  fun `app factory composes a custom signer matching the validator set`() {
    var receivedConfig: ValidatorSignerConfig? = null
    var closeCalls = 0
    val localSigner = LocalValidatorSigner(privateKey)
    val factory =
      MaruAppFactory(
        ValidatorSignerFactory { config ->
          receivedConfig = config
          localSigner.withCloseAction { closeCalls++ }
        },
      )

    val signer =
      factory.createValidatorSigner(
        qbftConfig = customQbftConfig(),
        beaconGenesisConfig = forkSchedule(setOf(SecpCrypto.privateKeyToValidator(privateKey))),
        privateKey = ByteArray(0),
      )

    assertThat(receivedConfig).isEqualTo(customSignerConfig)
    assertThat(ValidatorIdentityValidator.validatorFor(signer!!))
      .isEqualTo(SecpCrypto.privateKeyToValidator(privateKey))

    signer.close()
    signer.close()
    assertThat(closeCalls).isEqualTo(1)
  }

  @Test
  fun `app factory closes a custom signer rejected by validator identity validation`() {
    var closeCalls = 0
    val factory =
      MaruAppFactory(
        ValidatorSignerFactory {
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
      factory.createValidatorSigner(
        qbftConfig = customQbftConfig(),
        beaconGenesisConfig = forkSchedule(setOf(differentValidator)),
        privateKey = ByteArray(0),
      )
    }.isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("maru-validator")
      .hasMessageContaining("is not present in any configured validator set")

    assertThat(closeCalls).isEqualTo(1)
  }

  @Test
  fun `app factory closes a custom signer when later construction fails`() {
    var closeCalls = 0
    val localSigner = LocalValidatorSigner(privateKey)
    val validator = SecpCrypto.privateKeyToValidator(privateKey)
    val factory =
      MaruAppFactory(
        ValidatorSignerFactory {
          localSigner.withCloseAction { closeCalls++ }
        },
      )
    val now = 1_000_000L
    val schedule =
      ForksSchedule(
        chainId = 1u,
        forks =
        setOf(
          qbftFork(timestampSeconds = now.toULong(), validators = setOf(validator)),
          ForkSpec(
            timestampSeconds = (now + 10).toULong(),
            blockTimeSeconds = 1u,
            configuration =
            DifficultyAwareQbftConfig(
              postTtdConfig = qbftConsensusConfig(setOf(validator)),
              terminalTotalDifficulty = 42u,
            ),
          ),
        ),
      )

    assertThatThrownBy {
      factory.create(
        config = maruConfig(),
        beaconGenesisConfig = schedule,
        clock = Clock.fixed(Instant.ofEpochSecond(now), ZoneOffset.UTC),
      )
    }.isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("future fork enables DifficultyAwareQbft")

    assertThat(closeCalls).isEqualTo(1)
  }

  private fun customQbftConfig() =
    QbftConfig(
      feeRecipient = ByteArray(20),
      validatorSigner = customSignerConfig,
    )

  private fun forkSchedule(validators: Set<Validator>) =
    ForksSchedule(
      chainId = 1u,
      forks = setOf(qbftFork(0uL, validators)),
    )

  private fun qbftFork(
    timestampSeconds: ULong,
    validators: Set<Validator>,
  ) = ForkSpec(
    timestampSeconds = timestampSeconds,
    blockTimeSeconds = 1u,
    configuration = qbftConsensusConfig(validators),
  )

  private fun qbftConsensusConfig(validators: Set<Validator>) =
    QbftConsensusConfig(
      validatorSet = validators,
      fork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Prague),
    )

  private fun maruConfig(): MaruConfig {
    val endpoint = ApiEndpointConfig(URI.create("http://localhost:8545").toURL())
    return MaruConfig(
      persistence = Persistence(tempDir),
      qbft = customQbftConfig(),
      p2p = null,
      validatorElNode = ValidatorElNode(endpoint, payloadValidationEnabled = true),
      followers = FollowersConfig(emptyMap()),
      observability = ObservabilityConfig(),
      api = ApiConfig(),
      syncing =
      SyncingConfig(
        peerChainHeightPollingInterval = 1.seconds,
        syncTargetSelection = SyncingConfig.SyncTargetSelection.Highest,
      ),
      forkTransition = ForkTransition(),
    )
  }
}

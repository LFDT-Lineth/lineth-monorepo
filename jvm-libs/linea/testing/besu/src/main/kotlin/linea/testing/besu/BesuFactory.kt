/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.testing.besu

import linea.kotlin.encodeHex
import org.hyperledger.besu.consensus.qbft.QbftExtraDataCodec
import org.hyperledger.besu.crypto.KeyPairUtil
import org.hyperledger.besu.ethereum.api.jsonrpc.JsonRpcConfiguration
import org.hyperledger.besu.ethereum.core.AddressHelpers
import org.hyperledger.besu.ethereum.core.ImmutableMiningConfiguration
import org.hyperledger.besu.ethereum.core.ImmutableMiningConfiguration.MutableInitValues
import org.hyperledger.besu.ethereum.core.Util
import org.hyperledger.besu.ethereum.eth.sync.SynchronizerConfiguration
import org.hyperledger.besu.ethereum.storage.keyvalue.KeyValueSegmentIdentifier
import org.hyperledger.besu.plugin.services.storage.KeyValueStorageFactory
import org.hyperledger.besu.plugin.services.storage.rocksdb.RocksDBKeyValueStorageFactory
import org.hyperledger.besu.plugin.services.storage.rocksdb.RocksDBMetricsFactory
import org.hyperledger.besu.plugin.services.storage.rocksdb.configuration.RocksDBCLIOptions
import org.hyperledger.besu.tests.acceptance.dsl.node.BesuNode
import org.hyperledger.besu.tests.acceptance.dsl.node.configuration.BesuNodeConfigurationBuilder
import org.hyperledger.besu.tests.acceptance.dsl.node.configuration.BesuNodeFactory
import org.hyperledger.besu.tests.acceptance.dsl.node.configuration.genesis.GenesisConfigurationFactory
import java.util.Collections.singletonList
import java.util.Optional
import kotlin.jvm.optionals.getOrDefault
import kotlin.random.Random

object BesuFactory {
  private const val PRAGUE_GENESIS = "/el_prague.json"

  const val MIN_BLOCK_TIME = 1L
  const val BLOCK_REBUILD_TIME = 15L

  fun buildTestBesu(
    genesisFile: String = GenesisConfigurationFactory.readGenesisFile(PRAGUE_GENESIS),
    validator: Boolean = true,
    engineRpcPort: Optional<Int> = Optional.empty(),
    jsonRpcPort: Optional<Int> = Optional.empty(),
    nodeName: String = "miner-${Random.nextBytes(4).encodeHex(false)}",
    syncMinimumPeerCount: Int = 1,
  ): BesuNode =
    BesuNodeFactory().createNode(nodeName) { builder: BesuNodeConfigurationBuilder ->
      val persistentStorageFactory: KeyValueStorageFactory =
        RocksDBKeyValueStorageFactory(
          RocksDBCLIOptions.create()::toDomainObject,
          KeyValueSegmentIdentifier.entries,
          RocksDBMetricsFactory.PUBLIC_ROCKS_DB_METRICS,
        )

      val engineRpcConfig = JsonRpcConfiguration.createEngineDefault()
      engineRpcConfig.setEnabled(true)
      engineRpcConfig.setPort(engineRpcPort.getOrDefault(0))
      engineRpcConfig.host = "127.0.0.1"
      engineRpcConfig.setHostsAllowlist(singletonList("*"))
      engineRpcConfig.setAuthenticationEnabled(false)

      val jsonRpcConfig = JsonRpcConfiguration.createDefault()
      jsonRpcConfig.setEnabled(true)
      jsonRpcConfig.setPort(jsonRpcPort.getOrDefault(0))
      jsonRpcConfig.setHostsAllowlist(singletonList("*"))

      builder
        .storageImplementation(persistentStorageFactory)
        .genesisConfigProvider {
          val defaultSigner = KeyPairUtil.loadKeyPairFromResource("default-signer-key")
          val validatorAddress = Util.publicKeyToAddress(defaultSigner.publicKey)
          val extraDataHex = QbftExtraDataCodec.createGenesisExtraDataString(listOf(validatorAddress))
          Optional.of(
            genesisFile.replace("%extraData%", extraDataHex),
          )
        }.devMode(false)
        .discoveryEnabled(true)
        .discoveryV5Enabled(false)
        .engineJsonRpcConfiguration(engineRpcConfig)
        .jsonRpcConfiguration(jsonRpcConfig)
        .synchronizerConfiguration(
          SynchronizerConfiguration
            .builder()
            .syncMinimumPeerCount(syncMinimumPeerCount)
            .build(),
        )

      // Cap the inter-rebuild sleep to BLOCK_REBUILD_TIME (15 ms) for ALL nodes.
      // Without this, Besu's default of 500 ms makes engine_getPayload's empty-block
      // path (awaitCurrentBuildCompletion) block for up to 500 ms instead of 15 ms.
      val miningConfigBuilder =
        ImmutableMiningConfiguration
          .builder()
          .unstable(
            ImmutableMiningConfiguration.Unstable
              .builder()
              .posBlockFinalizationTimeoutMs(10)
              .posBlockCreationRepetitionMinDuration(BLOCK_REBUILD_TIME)
              .build(),
          )

      if (validator) {
        val defaultSigner = KeyPairUtil.loadKeyPairFromResource("default-signer-key")
        miningConfigBuilder.mutableInitValues(
          MutableInitValues
            .builder()
            .coinbase(AddressHelpers.ofValue(1))
            .isMiningEnabled(true)
            .build(),
        )
        builder
          .miningConfiguration(miningConfigBuilder.build())
          .keyPair(defaultSigner)
      } else {
        builder.miningConfiguration(miningConfigBuilder.build())
      }
    }

  fun buildSwitchableBesuQbft(
    pragueTimestamp: ULong = 0UL,
    cancunTimestamp: ULong = pragueTimestamp,
    shanghaiTimestamp: ULong = cancunTimestamp,
    ttd: ULong = 0UL,
    validator: Boolean,
  ): BesuNode {
    val genesisContent =
      GenesisConfigurationFactory.readGenesisFile(PRAGUE_GENESIS)

    val genesisWithForks =
      genesisContent
        .replace("\"shanghaiTime\": 0", "\"shanghaiTime\": $shanghaiTimestamp")
        .replace("\"cancunTime\": 0", "\"cancunTime\": $cancunTimestamp")
        .replace("\"pragueTime\": 0", "\"pragueTime\": $pragueTimestamp")
        .replace("\"terminalTotalDifficulty\": 0", "\"terminalTotalDifficulty\": $ttd")

    val defaultSigner = KeyPairUtil.loadKeyPairFromResource("default-signer-key")
    val validatorAddress = Util.publicKeyToAddress(defaultSigner.publicKey)
    val extraDataHex = QbftExtraDataCodec.createGenesisExtraDataString(listOf(validatorAddress))
    val genesisFile = genesisWithForks.replace("%extraData%", extraDataHex)

    return buildTestBesu(
      genesisFile = genesisFile,
      validator = validator,
      syncMinimumPeerCount = 0,
    )
  }
}

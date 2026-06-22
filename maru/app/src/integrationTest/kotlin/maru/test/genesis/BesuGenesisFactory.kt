/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.test.genesis

import com.fasterxml.jackson.databind.ObjectMapper
import com.fasterxml.jackson.databind.node.ObjectNode
import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import linea.kotlin.isSortedBy
import maru.consensus.DifficultyAwareQbftConfig
import maru.consensus.ElFork
import maru.consensus.ForkSpec
import maru.consensus.ForksSchedule
import org.hyperledger.besu.consensus.qbft.QbftExtraDataCodec
import org.hyperledger.besu.crypto.KeyPairUtil
import org.hyperledger.besu.ethereum.core.Util
import org.hyperledger.besu.tests.acceptance.dsl.node.configuration.genesis.GenesisConfigurationFactory

class BesuGenesisFactory(
  val genesisTemplate: String = genesisTemplateLondonWithoutConsensus,
  val blockPeriodSeconds: UInt = 1u,
  val createEmptyBlocks: Boolean = true,
) {
  private var forkSchedule: ForksSchedule? = null

  fun setForkSchedule(value: ForksSchedule) {
    this.forkSchedule = value
  }

  fun create(): String {
    if (this.forkSchedule == null) {
      throw IllegalStateException("forkSchedule must be initialized before calling creating genesis")
    }

    return createGenesisWithQBFT(
      genesisTemplate,
      blockPeriodSeconds,
      createEmptyBlocks,
      forkSchedule!!,
    )
  }

  companion object {
    val genesisTemplateLondonWithoutConsensus: String =
      GenesisConfigurationFactory.readGenesisFile("/besu-genesis-template.json")
    private val jsonObjectMapper: ObjectMapper = jacksonObjectMapper()

    fun createGenesisWithQBFT(
      genesisTemplate: String = genesisTemplateLondonWithoutConsensus,
      chainId: ULong,
      blockPeriodSeconds: UInt,
      terminalTotalDifficulty: ULong? = null,
      createEmptyBlocks: Boolean = true,
      shanghaiTimestamp: ULong? = null,
      cancunTimestamp: ULong? = null,
      pragueTimestamp: ULong? = null,
      osakaTimestamp: ULong? = null,
    ): String {
      require(blockPeriodSeconds in 1u..60u) { "blockPeriodSeconds must be between 1 and 60 seconds" }
      var updatedGenesis = genesisTemplate
      updatedGenesis = setGenesisConfigProperty(updatedGenesis, "chainId", chainId)
      updatedGenesis = applyQbftConsensusToGenesisJson(updatedGenesis, blockPeriodSeconds, createEmptyBlocks)

      if (terminalTotalDifficulty != null) {
        updatedGenesis = setGenesisConfigProperty(updatedGenesis, "terminalTotalDifficulty", terminalTotalDifficulty)
      }
      if (shanghaiTimestamp != null) {
        require(
          terminalTotalDifficulty != null,
        ) { "terminalTotalDifficulty must be defined when shanghaiTimestamp is defined" }
        updatedGenesis = setGenesisConfigProperty(updatedGenesis, "shanghaiTime", shanghaiTimestamp)
      }
      if (cancunTimestamp != null) {
        updatedGenesis = setGenesisConfigProperty(updatedGenesis, "cancunTime", cancunTimestamp)
      }
      if (pragueTimestamp != null) {
        updatedGenesis = setGenesisConfigProperty(updatedGenesis, "pragueTime", pragueTimestamp)
      }
      if (osakaTimestamp != null) {
        updatedGenesis = setGenesisConfigProperty(updatedGenesis, "osakaTime", osakaTimestamp)
      }
      return updatedGenesis
    }

    fun createGenesisWithQBFT(
      genesisTemplate: String = genesisTemplateLondonWithoutConsensus,
      blockPeriodSeconds: UInt,
      createEmptyBlocks: Boolean = true,
      forks: ForksSchedule,
    ): String {
      val terminalTotalDifficulty: ULong =
        forks.forks
          .firstOrNull { it.configuration is DifficultyAwareQbftConfig }
          ?.let {
            (it.configuration as DifficultyAwareQbftConfig).terminalTotalDifficulty
          } ?: 0UL

      var shanghaiTimestamp: ULong? = null
      var cancunTimestamp: ULong? = null
      var pragueTimestamp: ULong? = null
      var osakaTimestamp: ULong? = null
      val forksInAscendingOrder = forks.forks.sortedBy { it.timestampSeconds }
      val forksInDescendingOrder = forksInAscendingOrder.reversed()
      forksInDescendingOrder
        .forEach { forkSpec ->
          when (forkSpec.configuration.fork.elFork) {
            ElFork.Osaka -> {
              shanghaiTimestamp = calculateForkTimestampOrNull(forksInAscendingOrder, ElFork.Shanghai) ?: 0UL
              cancunTimestamp = calculateForkTimestampOrNull(forksInAscendingOrder, ElFork.Cancun) ?: 0UL
              pragueTimestamp = calculateForkTimestampOrNull(forksInAscendingOrder, ElFork.Prague) ?: 0UL
              osakaTimestamp = forkSpec.timestampSeconds
            }

            ElFork.Prague -> {
              shanghaiTimestamp = calculateForkTimestampOrNull(forksInAscendingOrder, ElFork.Shanghai) ?: 0UL
              cancunTimestamp = calculateForkTimestampOrNull(forksInAscendingOrder, ElFork.Cancun) ?: 0UL
              pragueTimestamp = forkSpec.timestampSeconds
            }

            ElFork.Cancun -> {
              shanghaiTimestamp = calculateForkTimestampOrNull(forksInAscendingOrder, ElFork.Shanghai) ?: 0UL
              cancunTimestamp = forkSpec.timestampSeconds
            }

            ElFork.Shanghai -> {
              shanghaiTimestamp = forkSpec.timestampSeconds
            }

            ElFork.Paris -> {} // nothing to do, terminalTotalDifficulty already set
          }
        }

      return createGenesisWithQBFT(
        genesisTemplate,
        forks.chainId.toULong(),
        blockPeriodSeconds,
        terminalTotalDifficulty,
        createEmptyBlocks,
        shanghaiTimestamp,
        cancunTimestamp,
        pragueTimestamp,
        osakaTimestamp,
      )
    }

    private fun calculateForkTimestampOrNull(
      forks: List<ForkSpec>,
      elFork: ElFork,
    ): ULong? {
      require(forks.isSortedBy { it.timestampSeconds }) {
        "forks must be sorted in ascending order by timestampSeconds"
      }
      val forkTimestamp =
        forks
          .firstOrNull { it.configuration.fork.elFork == elFork }
          ?.timestampSeconds
      if (forkTimestamp != null) {
        return forkTimestamp
      }

      val prevFork =
        forks
          .firstOrNull { it.configuration.fork.elFork.version < elFork.version }
      val nextFork =
        forks
          .firstOrNull { it.configuration.fork.elFork.version > elFork.version }
      // when not explicitly set, is in between explicitly set forks, shall be same as the next fork
      if (nextFork != null && prevFork != null) {
        return nextFork.timestampSeconds
      }

      return null
    }

    private fun applyQbftConsensusToGenesisJson(
      genesis: String,
      blockPeriodSeconds: UInt,
      createEmptyBlocks: Boolean,
    ): String {
      val rootNode = jsonObjectMapper.readTree(genesis) as ObjectNode
      val configNode = rootNode.get("config") as ObjectNode
      configNode.remove("clique")
      val qbftNode = jsonObjectMapper.createObjectNode()
      qbftNode.put("blockperiodseconds", blockPeriodSeconds.toLong())
      qbftNode.put("epochlength", 30000L)
      qbftNode.put("requesttimeoutseconds", 5L)
      qbftNode.put("blockreward", "5000000000000000000")
      qbftNode.put("xemptyblockperiodseconds", if (createEmptyBlocks) 0 else Int.MAX_VALUE)
      configNode.set<ObjectNode>("qbft", qbftNode)
      val defaultSigner = KeyPairUtil.loadKeyPairFromResource("default-signer-key")
      val validatorAddress = Util.publicKeyToAddress(defaultSigner.publicKey)
      val extraData = QbftExtraDataCodec.createGenesisExtraDataString(listOf(validatorAddress))
      rootNode.put("extraData", extraData)
      return jsonObjectMapper.writeValueAsString(rootNode)
    }

    private fun setGenesisConfigProperty(
      genesis: String,
      key: String,
      value: ULong,
    ): String {
      val rootNode = jsonObjectMapper.readTree(genesis) as ObjectNode
      val configNode = rootNode.get("config") as ObjectNode
      configNode.put(key, value.toLong())
      return jsonObjectMapper.writeValueAsString(rootNode)
    }
  }
}

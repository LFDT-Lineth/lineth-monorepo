/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.config.consensus

import com.sksamuel.hoplite.ArrayNode
import com.sksamuel.hoplite.ConfigFailure
import com.sksamuel.hoplite.ConfigResult
import com.sksamuel.hoplite.DecoderContext
import com.sksamuel.hoplite.MapNode
import com.sksamuel.hoplite.Node
import com.sksamuel.hoplite.decoder.Decoder
import com.sksamuel.hoplite.fp.NonEmptyList
import com.sksamuel.hoplite.fp.invalid
import com.sksamuel.hoplite.fp.valid
import com.sksamuel.hoplite.valueOrNull
import maru.consensus.ChainFork
import maru.consensus.ClFork
import maru.consensus.ConsensusConfig
import maru.consensus.DifficultyAwareQbftConfig
import maru.consensus.ElFork
import maru.consensus.ForkSpec
import maru.consensus.ForksSchedule
import maru.consensus.QbftConsensusConfig
import maru.core.Validator
import maru.extensions.fromHexToByteArray
import kotlin.collections.component1
import kotlin.collections.component2
import kotlin.collections.map
import kotlin.collections.toSet
import kotlin.reflect.KType

object ForkConfigDecoder : Decoder<JsonFriendlyForksSchedule> {
  override fun decode(
    node: Node,
    type: KType,
    context: DecoderContext,
  ): ConfigResult<JsonFriendlyForksSchedule> {
    val chainId = node.getString("chainid").toUInt()
    val config = node["config"]
    val forkSpecs =
      (config as MapNode).map.map { (k, v) ->
        val type = v.getString("type")
        val blockTimeSeconds = v.getString("blocktimeseconds").toInt()
        mapObjectToConfiguration(type, v).map {
          ForkSpec(
            k.toULong(),
            blockTimeSeconds.toUInt(),
            it,
          )
        }
      }
    return if (forkSpecs.all { it.isValid() }) {
      JsonFriendlyForksSchedule(chainId, forkSpecs.map { it.getUnsafe() }.toSet()).valid()
    } else {
      val failures = forkSpecs.filter { it.isInvalid() }.map { it.getInvalidUnsafe() }
      ConfigFailure.MultipleFailures(NonEmptyList(failures)).invalid()
    }
  }

  private fun Node.getString(key: String): String = this[key].valueOrNull()!!

  private fun mapObjectToConfiguration(
    type: String,
    obj: Node,
  ): ConfigResult<ConsensusConfig> =
    when (type) {
      "difficultyAwareQbft" -> {
        val terminalTotalDifficulty = obj.getString("terminaltotaldifficulty").toULong()
        val postTtdQbftSpec = mapObjectToConfiguration("qbft", obj["postttdconfig"]).getUnsafe()
        require(postTtdQbftSpec is QbftConsensusConfig) {
          "DifficultyAwareQbft only supports QBFT as the post TTD protocol"
        }
        // Override CLFork to Phase0
        DifficultyAwareQbftConfig(
          postTtdConfig = postTtdQbftSpec.copy(fork = postTtdQbftSpec.fork.copy(clFork = ClFork.QBFT_PHASE0)),
          terminalTotalDifficulty,
        ).valid()
      }

      "qbft" ->
        QbftConsensusConfig(
          validatorSet = (obj["validatorset"] as ArrayNode)
            .elements
            .map {
              Validator(
                it.valueOrNull()!!.fromHexToByteArray(),
              )
            }.toSet(),
          fork = ChainFork(
            clFork = ClFork.QBFT_PHASE0,
            elFork = ElFork.valueOf(obj.getString("elfork")),
          ),
        ).valid()

      else -> (ConfigFailure.UnsupportedCollectionType(obj, "Unsupported fork type $type!") as ConfigFailure).invalid()
    }

  override fun supports(type: KType): Boolean = type.classifier == JsonFriendlyForksSchedule::class
}

data class JsonFriendlyForksSchedule(
  val chainId: UInt,
  val config: Set<ForkSpec>,
) {
  override fun equals(other: Any?): Boolean {
    if (other !is JsonFriendlyForksSchedule) {
      return false
    }
    return config.containsAll(other.config) && config.size == other.config.size
  }

  fun domainFriendly(): ForksSchedule = ForksSchedule(chainId, config)

  override fun hashCode(): Int = config.hashCode()
}

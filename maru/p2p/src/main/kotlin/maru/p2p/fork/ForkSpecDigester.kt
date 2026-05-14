/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.fork

import linea.kotlin.encodeHex
import maru.consensus.DifficultyAwareQbftConfig
import maru.consensus.ForkSpec
import maru.consensus.QbftConsensusConfig
import maru.serialization.Serializer
import java.nio.ByteBuffer

object QbftConsensusConfigDigester : Serializer<QbftConsensusConfig> {
  override fun serialize(value: QbftConsensusConfig): ByteArray {
    // Sort validators deterministically by address hex
    val validatorsSorted = value.validatorSet.sortedBy { it.address.encodeHex(prefix = false) }
    // Allocate buffer: 20 bytes per validator + 2 bytes (ClFork + ELFork)
    val buffer = ByteBuffer.allocate(validatorsSorted.size * 20 + 2)
    for (validator in validatorsSorted) {
      buffer.put(validator.address)
    }
    buffer.put(value.fork.clFork.version)
    buffer.put(value.fork.elFork.version)
    return buffer.array()
  }
}

object DifficultyAwareQbftConfigDigester : Serializer<DifficultyAwareQbftConfig> {
  override fun serialize(value: DifficultyAwareQbftConfig): ByteArray {
    val postTtdConfigBytes = QbftConsensusConfigDigester.serialize(value.postTtdConfig)
    val buffer = ByteBuffer.allocate(postTtdConfigBytes.size + 8)
    buffer.putLong(value.terminalTotalDifficulty.toLong())
    buffer.put(postTtdConfigBytes)
    return buffer.array()
  }
}

object ForkSpecDigester : Serializer<ForkSpec> {
  override fun serialize(value: ForkSpec): ByteArray {
    val serializedConsensusConfig =
      when (value.configuration) {
        is QbftConsensusConfig ->
          QbftConsensusConfigDigester.serialize(value.configuration as QbftConsensusConfig)

        is DifficultyAwareQbftConfig ->
          DifficultyAwareQbftConfigDigester.serialize(value.configuration as DifficultyAwareQbftConfig)

        else -> throw IllegalArgumentException("${value.configuration.javaClass.simpleName} is not supported!")
      }

    return ByteBuffer
      .allocate(8 + serializedConsensusConfig.size)
      .putLong(value.timestampSeconds.toLong())
      .put(serializedConsensusConfig)
      .array()
  }
}

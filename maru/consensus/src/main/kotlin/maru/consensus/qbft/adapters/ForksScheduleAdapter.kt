/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft.adapters

import maru.config.QbftConfig
import maru.consensus.ForkSpec
import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.config.BftConfigOptions
import org.hyperledger.besu.datatypes.Address
import java.math.BigInteger
import java.util.Optional
import org.hyperledger.besu.consensus.common.ForkSpec as BesuForkSpec
import org.hyperledger.besu.consensus.common.ForksSchedule as BesuForksSchedule

class ForksScheduleAdapter(
  currentSpec: ForkSpec,
  config: QbftConfig,
) : BesuForksSchedule<BftConfigOptions>(maruForkSpecsToBesu(currentSpec, config)) {
  companion object {
    fun maruForkSpecsToBesu(
      currentSpec: ForkSpec,
      config: QbftConfig,
    ): MutableCollection<BesuForkSpec<BftConfigOptions>> =
      mutableListOf(BesuForkSpec(0, createBftConfig(currentSpec, config)))

    private fun createBftConfig(
      spec: ForkSpec,
      config: QbftConfig,
    ): BftConfigOptions =
      object : BftConfigOptions {
        override fun getEpochLength(): Long = 0

        override fun getBlockPeriodSeconds(): Int = spec.blockTimeSeconds.toInt()

        override fun getEmptyBlockPeriodSeconds(): Int = 0

        override fun getBlockPeriodMilliseconds(): Long = 0

        override fun getRequestTimeoutSeconds(): Int = 0

        override fun getGossipedHistoryLimit(): Int = 0

        override fun getMessageQueueLimit(): Int = config.messageQueueLimit

        override fun getDuplicateMessageLimit(): Int = config.duplicateMessageLimit

        override fun getFutureMessagesLimit(): Int = config.futureMessagesLimit.toInt()

        override fun getFutureMessagesMaxDistance(): Int = config.futureMessageMaxDistance.toInt()

        override fun getMiningBeneficiary(): Optional<Address> =
          Optional.of(Address.wrap(Bytes.wrap(config.feeRecipient)))

        override fun getBlockRewardWei(): BigInteger = BigInteger.ZERO

        override fun asMap(): Map<String, Any> = emptyMap()
      }
  }
}

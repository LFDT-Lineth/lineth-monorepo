/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txselection.selectors;

import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.evm.log.LogTopic;

public record TransactionEventFilter(
    Address contractAddress, LogTopic topic0, LogTopic topic1, LogTopic topic2, LogTopic topic3) {
  /**
   * Checks whether the supplied contract address and log topics matches this TransactionEventFilter
   *
   * @param contractAddress the contract address of the log to be checked
   * @param topics the LogTopics to be checked, supplied in order
   * @return true if the supplied contract address and log topics matches this
   *     TransactionEventFilter, false otherwise
   */
  public boolean matches(final Address contractAddress, final LogTopic... topics) {
    return contractAddress.equals(this.contractAddress)
        && (topic0 == null || (topics.length >= 1 && topics[0].equals(topic0)))
        && (topic1 == null || (topics.length >= 2 && topics[1].equals(topic1)))
        && (topic2 == null || (topics.length >= 3 && topics[2].equals(topic2)))
        && (topic3 == null || (topics.length >= 4 && topics[3].equals(topic3)));
  }
}

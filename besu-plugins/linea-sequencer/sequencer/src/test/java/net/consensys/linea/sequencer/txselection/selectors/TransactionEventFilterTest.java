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
import org.hyperledger.besu.datatypes.LogTopic;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.Test;

public class TransactionEventFilterTest {
  private static final LogTopic WILDCARD_LOGTOPIC = null;

  private static final Address CONTRACT_ADDRESS =
      Address.fromHexString("0x1234567890123456789012345678901234567890");
  private static final Address OTHER_ADDRESS =
      Address.fromHexString("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd");

  private static final LogTopic TOPIC_0 = LogTopic.fromHexString("0x01");
  private static final LogTopic TOPIC_1 = LogTopic.fromHexString("0x02");
  private static final LogTopic TOPIC_2 = LogTopic.fromHexString("0x03");
  private static final LogTopic TOPIC_3 = LogTopic.fromHexString("0x04");
  private static final LogTopic OTHER_TOPIC = LogTopic.fromHexString("0xff");

  @Test
  public void testMatchesWithWildcardTopic() {
    TransactionEventFilter transactionEventFilter =
        new TransactionEventFilter(CONTRACT_ADDRESS, TOPIC_0, WILDCARD_LOGTOPIC, TOPIC_2, TOPIC_3);

    Assertions.assertTrue(
        transactionEventFilter.matches(CONTRACT_ADDRESS, TOPIC_0, TOPIC_1, TOPIC_2, TOPIC_3));
  }

  @Test
  public void testMatchesWithIncorrectContractAddress() {
    TransactionEventFilter transactionEventFilter =
        new TransactionEventFilter(
            CONTRACT_ADDRESS,
            WILDCARD_LOGTOPIC,
            WILDCARD_LOGTOPIC,
            WILDCARD_LOGTOPIC,
            WILDCARD_LOGTOPIC);

    Assertions.assertFalse(transactionEventFilter.matches(OTHER_ADDRESS));
  }

  @Test
  public void testMatchesWithIncorrectTopics() {
    TransactionEventFilter transactionEventFilter =
        new TransactionEventFilter(
            CONTRACT_ADDRESS, TOPIC_0, WILDCARD_LOGTOPIC, TOPIC_2, OTHER_TOPIC);

    Assertions.assertFalse(
        transactionEventFilter.matches(CONTRACT_ADDRESS, TOPIC_0, TOPIC_1, TOPIC_2, TOPIC_3));
  }

  @Test
  public void testMatchesWithDuplicateTopic() {
    TransactionEventFilter transactionEventFilter =
        new TransactionEventFilter(
            CONTRACT_ADDRESS, WILDCARD_LOGTOPIC, WILDCARD_LOGTOPIC, WILDCARD_LOGTOPIC, TOPIC_0);

    Assertions.assertTrue(
        transactionEventFilter.matches(CONTRACT_ADDRESS, TOPIC_0, TOPIC_0, TOPIC_0, TOPIC_0));
  }
}

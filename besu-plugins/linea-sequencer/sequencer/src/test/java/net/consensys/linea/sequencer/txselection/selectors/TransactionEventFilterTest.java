/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txselection.selectors;

import java.util.List;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.evm.log.Log;
import org.hyperledger.besu.evm.log.LogTopic;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;

public class TransactionEventFilterTest {
  private static final LogTopic WILDCARD_LOGTOPIC = null;

  @Test
  public void testMatchesWithWildcardTopic() {
    Address contractAddress = Mockito.mock(Address.class);
    LogTopic topic0 = Mockito.mock(LogTopic.class);
    LogTopic topic1 = Mockito.mock(LogTopic.class);
    LogTopic topic2 = Mockito.mock(LogTopic.class);
    LogTopic topic3 = Mockito.mock(LogTopic.class);

    TransactionEventFilter transactionEventFilter =
        new TransactionEventFilter(contractAddress, topic0, WILDCARD_LOGTOPIC, topic2, topic3);

    Assertions.assertTrue(
        transactionEventFilter.matches(contractAddress, topic0, topic1, topic2, topic3));
  }

  @Test
  public void testMatchesWithIncorrectContractAddress() {
    Address contractAddress = Mockito.mock(Address.class);
    TransactionEventFilter transactionEventFilter =
        new TransactionEventFilter(
            contractAddress,
            WILDCARD_LOGTOPIC,
            WILDCARD_LOGTOPIC,
            WILDCARD_LOGTOPIC,
            WILDCARD_LOGTOPIC);

    Assertions.assertFalse(transactionEventFilter.matches(Mockito.mock(Address.class)));
  }

  @Test
  public void testMatchesWithIncorrectTopics() {
    Address contractAddress = Mockito.mock(Address.class);
    LogTopic topic0 = Mockito.mock(LogTopic.class);
    LogTopic topic1 = Mockito.mock(LogTopic.class);
    LogTopic topic2 = Mockito.mock(LogTopic.class);
    LogTopic topic3 = Mockito.mock(LogTopic.class);

    TransactionEventFilter transactionEventFilter =
        new TransactionEventFilter(
            contractAddress, topic0, WILDCARD_LOGTOPIC, topic2, Mockito.mock(LogTopic.class));

    Assertions.assertFalse(
        transactionEventFilter.matches(contractAddress, topic0, topic1, topic2, topic3));
  }

  @Test
  public void testMatchesWithDuplicateTopic() {
    Address contractAddress = Mockito.mock(Address.class);
    LogTopic topic = Mockito.mock(LogTopic.class);

    TransactionEventFilter transactionEventFilter =
        new TransactionEventFilter(
            contractAddress, WILDCARD_LOGTOPIC, WILDCARD_LOGTOPIC, WILDCARD_LOGTOPIC, topic);

    Log log = Mockito.mock(Log.class);
    Mockito.when(log.getLogger()).thenReturn(contractAddress);
    List<LogTopic> logTopics = List.of(topic, topic, topic, topic);
    Mockito.when(log.getTopics()).thenReturn(logTopics);

    Assertions.assertTrue(
        transactionEventFilter.matches(contractAddress, topic, topic, topic, topic));
  }
}

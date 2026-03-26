/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txselection.selectors;

import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.DENIED_LOG_TOPIC;

import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.atomic.AtomicReference;
import net.consensys.linea.bundles.TransactionBundle;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.Log;
import org.hyperledger.besu.datatypes.LogTopic;
import org.hyperledger.besu.datatypes.PendingTransaction;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.plugin.data.TransactionProcessingResult;
import org.hyperledger.besu.plugin.data.TransactionSelectionResult;
import org.hyperledger.besu.plugin.services.txselection.TransactionEvaluationContext;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;

public class TransactionEventSelectorTest {
  private final AtomicReference<Map<Address, Set<TransactionEventFilter>>> deniedEvents =
      new AtomicReference<>(Collections.emptyMap());
  private final AtomicReference<Map<Address, Set<TransactionEventFilter>>> deniedBundleEvents =
      new AtomicReference<>(Collections.emptyMap());

  private TransactionEventSelector selector;

  @BeforeEach
  public void beforeTest() {
    selector = new TransactionEventSelector(deniedEvents, deniedBundleEvents);
  }

  @AfterEach
  public void afterTest() {
    deniedEvents.set(Collections.emptyMap());
    deniedBundleEvents.set(Collections.emptyMap());
  }

  @Test
  public void testEvaluateTransactionPostProcessingForSingleTransactionWithEmptyDenyList() {
    Address address = Mockito.mock(Address.class);
    TransactionEvaluationContext evaluationContext =
        Mockito.mock(TransactionEvaluationContext.class);
    TransactionProcessingResult processingResult = Mockito.mock(TransactionProcessingResult.class);

    Log log = Mockito.mock(Log.class);
    Mockito.when(processingResult.getLogs()).thenReturn(List.of(log));
    Mockito.when(log.getLogger()).thenReturn(address);

    mockTransactionType(false, evaluationContext);

    TransactionSelectionResult actualResult =
        selector.evaluateTransactionPostProcessing(evaluationContext, processingResult);

    Mockito.verify(evaluationContext).getPendingTransaction();
    Mockito.verify(processingResult).getLogs();
    Mockito.verifyNoMoreInteractions(evaluationContext, processingResult);

    Assertions.assertEquals(TransactionSelectionResult.SELECTED, actualResult);
  }

  @Test
  public void testEvaluateTransactionPostProcessingForSingleTransactionWithDenyListButSelected() {
    Address address = Mockito.mock(Address.class);
    TransactionEventFilter transactionEventFilter = Mockito.mock(TransactionEventFilter.class);
    deniedEvents.set(Map.of(address, Set.of(transactionEventFilter)));
    TransactionEvaluationContext evaluationContext =
        Mockito.mock(TransactionEvaluationContext.class);
    TransactionProcessingResult processingResult = Mockito.mock(TransactionProcessingResult.class);

    mockTransactionType(false, evaluationContext);
    Log log = Mockito.mock(Log.class);
    Mockito.when(processingResult.getLogs()).thenReturn(List.of(log));
    Mockito.when(log.getLogger()).thenReturn(address);
    LogTopic logTopic = LogTopic.fromHexString("0x01");
    Mockito.when(log.getTopics()).thenReturn(List.of(logTopic));
    Mockito.when(transactionEventFilter.matches(Mockito.eq(address), Mockito.any(LogTopic[].class)))
        .thenReturn(false);

    TransactionSelectionResult actualResult =
        selector.evaluateTransactionPostProcessing(evaluationContext, processingResult);

    Mockito.verify(evaluationContext).getPendingTransaction();
    Mockito.verify(processingResult).getLogs();
    Mockito.verify(transactionEventFilter)
        .matches(Mockito.eq(address), Mockito.any(LogTopic[].class));
    Mockito.verifyNoMoreInteractions(evaluationContext, processingResult, transactionEventFilter);

    Assertions.assertEquals(TransactionSelectionResult.SELECTED, actualResult);
  }

  @Test
  public void testEvaluateTransactionPostProcessingForSingleTransactionWithDenyListButInvalid() {
    Address address = Mockito.mock(Address.class);
    TransactionEventFilter transactionEventFilter = Mockito.mock(TransactionEventFilter.class);
    deniedEvents.set(Map.of(address, Set.of(transactionEventFilter)));
    TransactionEvaluationContext evaluationContext =
        Mockito.mock(TransactionEvaluationContext.class);
    TransactionProcessingResult processingResult = Mockito.mock(TransactionProcessingResult.class);

    mockTransactionType(false, evaluationContext);
    Log log = Mockito.mock(Log.class);
    Mockito.when(processingResult.getLogs()).thenReturn(List.of(log));
    Mockito.when(log.getLogger()).thenReturn(address);
    LogTopic logTopic = LogTopic.fromHexString("0x01");
    Mockito.when(log.getTopics()).thenReturn(List.of(logTopic));
    Mockito.when(transactionEventFilter.matches(Mockito.eq(address), Mockito.any(LogTopic[].class)))
        .thenReturn(true);

    TransactionSelectionResult actualResult =
        selector.evaluateTransactionPostProcessing(evaluationContext, processingResult);

    Mockito.verify(evaluationContext).getPendingTransaction();
    Mockito.verify(processingResult).getLogs();
    Mockito.verify(transactionEventFilter)
        .matches(Mockito.eq(address), Mockito.any(LogTopic[].class));
    Mockito.verifyNoMoreInteractions(evaluationContext, processingResult, transactionEventFilter);

    Assertions.assertEquals(DENIED_LOG_TOPIC, actualResult);
  }

  @Test
  public void testEvaluateTransactionPostProcessingForBundleTransactionWithEmptyDenyList() {
    Address address = Mockito.mock(Address.class);
    TransactionEvaluationContext evaluationContext =
        Mockito.mock(TransactionEvaluationContext.class);
    TransactionProcessingResult processingResult = Mockito.mock(TransactionProcessingResult.class);
    Log log = Mockito.mock(Log.class);
    Mockito.when(processingResult.getLogs()).thenReturn(List.of(log));
    Mockito.when(log.getLogger()).thenReturn(address);

    mockTransactionType(true, evaluationContext);

    TransactionSelectionResult actualResult =
        selector.evaluateTransactionPostProcessing(evaluationContext, processingResult);

    Mockito.verify(evaluationContext).getPendingTransaction();
    Mockito.verify(processingResult).getLogs();
    Mockito.verifyNoMoreInteractions(evaluationContext, processingResult);

    Assertions.assertEquals(TransactionSelectionResult.SELECTED, actualResult);
  }

  @Test
  public void testEvaluateTransactionPostProcessingForBundleTransactionWithDenyListButSelected() {
    Address address = Mockito.mock(Address.class);
    TransactionEventFilter transactionEventFilter = Mockito.mock(TransactionEventFilter.class);
    deniedBundleEvents.set(Map.of(address, Set.of(transactionEventFilter)));
    TransactionEvaluationContext evaluationContext =
        Mockito.mock(TransactionEvaluationContext.class);
    TransactionProcessingResult processingResult = Mockito.mock(TransactionProcessingResult.class);

    mockTransactionType(true, evaluationContext);
    Log log = Mockito.mock(Log.class);
    Mockito.when(processingResult.getLogs()).thenReturn(List.of(log));
    Mockito.when(log.getLogger()).thenReturn(address);
    LogTopic logTopic = LogTopic.fromHexString("0x01");
    Mockito.when(log.getTopics()).thenReturn(List.of(logTopic));
    Mockito.when(transactionEventFilter.matches(Mockito.eq(address), Mockito.any(LogTopic[].class)))
        .thenReturn(false);

    TransactionSelectionResult actualResult =
        selector.evaluateTransactionPostProcessing(evaluationContext, processingResult);

    Mockito.verify(evaluationContext).getPendingTransaction();
    Mockito.verify(processingResult).getLogs();
    Mockito.verify(transactionEventFilter)
        .matches(Mockito.eq(address), Mockito.any(LogTopic[].class));
    Mockito.verifyNoMoreInteractions(evaluationContext, processingResult, transactionEventFilter);

    Assertions.assertEquals(TransactionSelectionResult.SELECTED, actualResult);
  }

  @Test
  public void testEvaluateTransactionPostProcessingForBundleTransactionWithDenyListButInvalid() {
    Address address = Mockito.mock(Address.class);
    TransactionEventFilter transactionEventFilter = Mockito.mock(TransactionEventFilter.class);
    deniedBundleEvents.set(Map.of(address, Set.of(transactionEventFilter)));
    TransactionEvaluationContext evaluationContext =
        Mockito.mock(TransactionEvaluationContext.class);
    TransactionProcessingResult processingResult = Mockito.mock(TransactionProcessingResult.class);

    mockTransactionType(true, evaluationContext);
    Log log = Mockito.mock(Log.class);
    Mockito.when(processingResult.getLogs()).thenReturn(List.of(log));
    Mockito.when(log.getLogger()).thenReturn(address);
    LogTopic logTopic = LogTopic.fromHexString("0x01");
    Mockito.when(log.getTopics()).thenReturn(List.of(logTopic));
    Mockito.when(transactionEventFilter.matches(Mockito.eq(address), Mockito.any(LogTopic[].class)))
        .thenReturn(true);

    TransactionSelectionResult actualResult =
        selector.evaluateTransactionPostProcessing(evaluationContext, processingResult);

    Mockito.verify(evaluationContext).getPendingTransaction();
    Mockito.verify(processingResult).getLogs();
    Mockito.verify(transactionEventFilter)
        .matches(Mockito.eq(address), Mockito.any(LogTopic[].class));
    Mockito.verifyNoMoreInteractions(evaluationContext, processingResult, transactionEventFilter);

    Assertions.assertEquals(DENIED_LOG_TOPIC, actualResult);
  }

  private void mockTransactionType(
      final boolean isBundle, final TransactionEvaluationContext evaluationContext) {
    PendingTransaction mockPendingTransaction;
    if (isBundle) {
      mockPendingTransaction = Mockito.mock(TransactionBundle.PendingBundleTx.class);
    } else {
      mockPendingTransaction = Mockito.mock(PendingTransaction.class);
    }
    Mockito.when(evaluationContext.getPendingTransaction()).thenReturn(mockPendingTransaction);
    Transaction transaction = Mockito.mock(Transaction.class);
    Mockito.when(mockPendingTransaction.getTransaction()).thenReturn(transaction);
    Mockito.when(transaction.getHash())
        .thenReturn(Hash.fromHexStringLenient("deadbeefdeadbeefdeadbeefdeadbeef"));
  }
}

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

    Log log = Mockito.mock(Log.class);
    Mockito.when(log.getLogger()).thenReturn(contractAddress);
    List<LogTopic> logTopics = List.of(topic0, topic1, topic2, topic3);
    Mockito.when(log.getTopics()).thenReturn(logTopics);

    Assertions.assertTrue(transactionEventFilter.matches(log));
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

    Log log = Mockito.mock(Log.class);
    Mockito.when(log.getLogger()).thenReturn(Mockito.mock(Address.class));

    Assertions.assertFalse(transactionEventFilter.matches(log));
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

    Log log = Mockito.mock(Log.class);
    Mockito.when(log.getLogger()).thenReturn(contractAddress);
    List<LogTopic> logTopics = List.of(topic0, topic1, topic2, topic3);
    Mockito.when(log.getTopics()).thenReturn(logTopics);

    Assertions.assertFalse(transactionEventFilter.matches(log));
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

    Assertions.assertTrue(transactionEventFilter.matches(log));
  }
}

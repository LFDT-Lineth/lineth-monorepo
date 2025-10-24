package net.consensys.linea.sequencer.txselection.selectors;

import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.evm.log.Log;
import org.hyperledger.besu.evm.log.LogTopic;

public record TransactionEventFilter(
    Address contractAddress, LogTopic topic0, LogTopic topic1, LogTopic topic2, LogTopic topic3) {
  /**
   * Checks whether the supplied log matches this TransactionEventFilter
   *
   * @param log the Log to be checked
   * @return true if the supplied log matches this TransactionEventFilter, false otherwise
   */
  public boolean matches(final Log log) {
    return log.getLogger().equals(contractAddress)
        && (topic0 == null
            || (log.getTopics().size() >= 1 && log.getTopics().get(0).equals(topic0)))
        && (topic1 == null
            || (log.getTopics().size() >= 2 && log.getTopics().get(1).equals(topic1)))
        && (topic2 == null
            || (log.getTopics().size() >= 3 && log.getTopics().get(2).equals(topic2)))
        && (topic3 == null
            || (log.getTopics().size() >= 4 && log.getTopics().get(3).equals(topic3)));
  }
}

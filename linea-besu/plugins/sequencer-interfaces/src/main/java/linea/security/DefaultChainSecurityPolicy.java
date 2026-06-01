package linea.security;

import org.hyperledger.besu.plugin.services.txselection.TransactionEvaluationContext;

/** Default implementation of ChainSecurityPolicy that allows all transactions. */
public class DefaultChainSecurityPolicy implements ChainSecurityPolicy {
  @Override
  public boolean shallForceIncludeTransaction(TransactionEvaluationContext txContext) {
    return false;
  }
}

package linea.security;

import org.hyperledger.besu.plugin.services.BesuService;
import org.hyperledger.besu.plugin.services.txselection.TransactionEvaluationContext;

/**
 * Service to provide security-related functionalities for the chain. This includes identifying
 * forced inclusion transactions, which are transactions that should be included in blocks
 * regardless of other security checks.
 *
 * <p>3Rd party plugins shall inject this ChainSecurityProvider which will be provided by Linea Besu
 * at runtime
 */
public interface ChainSecurityPolicy extends BesuService {
  /**
   * Checks if the transaction shall be forced inclued in the current block.
   *
   * <p>PluginTransactionSelector implementations should check if the transaction is a forced
   * inclusion and allow it to be included in the block regardless of other checks.
   *
   * @param txContext
   * @return true if the transaction shall be forced included in the block, false otherwise
   */
  boolean shallForceIncludeTransaction(final TransactionEvaluationContext txContext);
}

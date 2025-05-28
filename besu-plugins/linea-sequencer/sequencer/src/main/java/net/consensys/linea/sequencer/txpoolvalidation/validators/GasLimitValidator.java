/*
 * Copyright Consensys Software Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
 */
package net.consensys.linea.sequencer.txpoolvalidation.validators;

import java.util.Optional;

import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.config.LineaTransactionPoolValidatorConfiguration;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.plugin.services.txvalidator.PluginTransactionPoolValidator;

/**
 * Validator that checks if the gas limit is below the configured max amount. This means that max
 * gas limit of a transaction could be less than the block gas limit.
 */
@Slf4j
@RequiredArgsConstructor
public class GasLimitValidator implements PluginTransactionPoolValidator {
  final LineaTransactionPoolValidatorConfiguration txPoolValidatorConf;

  @Override
  public Optional<String> validateTransaction(
      final Transaction transaction, final boolean isLocal, final boolean hasPriority) {
    if (transaction.getGasLimit() > txPoolValidatorConf.maxTxGasLimit()) {
      final String errMsg =
          "Gas limit of transaction is greater than the allowed max of "
              + txPoolValidatorConf.maxTxGasLimit();
      log.debug(errMsg);
      return Optional.of(errMsg);
    }
    return Optional.empty();
  }
}

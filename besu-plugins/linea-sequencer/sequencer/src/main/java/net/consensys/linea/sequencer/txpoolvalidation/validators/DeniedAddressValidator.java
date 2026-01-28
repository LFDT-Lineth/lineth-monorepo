/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txpoolvalidation.validators;

import java.util.Optional;
import java.util.Set;
import java.util.concurrent.atomic.AtomicReference;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.plugin.services.txvalidator.PluginTransactionPoolValidator;

/**
 * Validator that checks if the sender or recipient address is on the deny list (e.g., SDN or other
 * legally prohibited addresses).
 */
@Slf4j
@RequiredArgsConstructor
public class DeniedAddressValidator implements PluginTransactionPoolValidator {

  private final AtomicReference<Set<Address>> denied;

  @Override
  public Optional<String> validateTransaction(
      final Transaction transaction, final boolean isLocal, final boolean hasPriority) {
    return checkDenied(transaction.getSender(), "sender")
        .or(() -> transaction.getTo().flatMap(to -> checkDenied(to, "recipient")));
  }

  private Optional<String> checkDenied(final Address address, final String role) {
    if (denied.get().contains(address)) {
      final String errMsg =
          String.format(
              "%s %s is blocked as appearing on the SDN or other legally prohibited list",
              role, address);
      log.debug(errMsg);
      return Optional.of(errMsg);
    }
    return Optional.empty();
  }
}

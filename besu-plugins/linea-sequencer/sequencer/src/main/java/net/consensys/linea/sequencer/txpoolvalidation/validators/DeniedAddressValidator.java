/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txpoolvalidation.validators;

import java.util.List;
import java.util.Optional;
import java.util.Set;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.CodeDelegation;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.plugin.services.txvalidator.PluginTransactionPoolValidator;

/**
 * Validator that checks sender, recipient, and EIP-7702 authorization list entries (authority and
 * delegation target address) against the deny list (e.g., SDN or other legally prohibited
 * addresses). When authority recovery fails for a delegation tuple, logs a warning, skips the
 * authority check, and still checks the delegation target address.
 */
@Slf4j
@RequiredArgsConstructor
public class DeniedAddressValidator implements PluginTransactionPoolValidator {

  private final Set<Address> denied;

  @Override
  public Optional<String> validateTransaction(
      final Transaction transaction, final boolean isLocal, final boolean hasPriority) {
    return checkDenied(transaction.getSender(), "sender")
        .or(() -> transaction.getTo().flatMap(to -> checkDenied(to, "recipient")))
        .or(() -> transaction.getCodeDelegationList().flatMap(this::checkCodeDelegationList));
  }

  private Optional<String> checkCodeDelegationList(final List<CodeDelegation> codeDelegations) {
    for (final CodeDelegation delegation : codeDelegations) {
      final Optional<Address> maybeAuthority = delegation.authorizer();
      if (maybeAuthority.isEmpty()) {
        log.atInfo()
            .setMessage("action=skip_unrecoverable_authority delegationAddress={}")
            .addArgument(delegation::address)
            .log();
      } else {
        final Optional<String> authorityResult =
            checkDenied(maybeAuthority.get(), "authorization authority");
        if (authorityResult.isPresent()) {
          return authorityResult;
        }
      }

      final Optional<String> addressResult =
          checkDenied(delegation.address(), "authorization address");
      if (addressResult.isPresent()) {
        return addressResult;
      }
    }
    return Optional.empty();
  }

  private Optional<String> checkDenied(final Address address, final String role) {
    if (denied.contains(address)) {
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

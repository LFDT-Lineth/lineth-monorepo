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
import lombok.extern.slf4j.Slf4j;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.plugin.services.txvalidator.PluginTransactionPoolValidator;

/** Validator that rejects transactions sent to precompile addresses. */
@Slf4j
public class PrecompileAddressValidator implements PluginTransactionPoolValidator {

  private static final Set<Address> PRECOMPILES =
      Set.of(
          Address.fromHexString("0x0000000000000000000000000000000000000001"),
          Address.fromHexString("0x0000000000000000000000000000000000000002"),
          Address.fromHexString("0x0000000000000000000000000000000000000003"),
          Address.fromHexString("0x0000000000000000000000000000000000000004"),
          Address.fromHexString("0x0000000000000000000000000000000000000005"),
          Address.fromHexString("0x0000000000000000000000000000000000000006"),
          Address.fromHexString("0x0000000000000000000000000000000000000007"),
          Address.fromHexString("0x0000000000000000000000000000000000000008"),
          Address.fromHexString("0x0000000000000000000000000000000000000009"),
          Address.fromHexString("0x000000000000000000000000000000000000000a"),
          Address.fromHexString("0x000000000000000000000000000000000000000b"),
          Address.fromHexString("0x000000000000000000000000000000000000000c"),
          Address.fromHexString("0x000000000000000000000000000000000000000d"),
          Address.fromHexString("0x000000000000000000000000000000000000000e"),
          Address.fromHexString("0x000000000000000000000000000000000000000f"),
          Address.fromHexString("0x0000000000000000000000000000000000000010"),
          Address.fromHexString("0x0000000000000000000000000000000000000011"),
          Address.fromHexString("0x0000000000000000000000000000000000000100"));

  @Override
  public Optional<String> validateTransaction(
      final Transaction transaction, final boolean isLocal, final boolean hasPriority) {
    if (transaction.getTo().isPresent()) {
      final Address to = transaction.getTo().get();
      if (PRECOMPILES.contains(to)) {
        final String errMsg =
            "destination address is a precompile address and cannot receive transactions";
        log.debug(errMsg);
        return Optional.of(errMsg);
      }
    }
    return Optional.empty();
  }
}

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
package net.consensys.linea.zktracer.module.hub;

import static graphql.com.google.common.base.Preconditions.checkState;
import static net.consensys.linea.zktracer.types.AddressUtils.isPrecompile;

import java.util.Optional;
import net.consensys.linea.zktracer.Fork;
import net.consensys.linea.zktracer.types.Bytecode;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.evm.account.Account;
import org.hyperledger.besu.evm.worldstate.WorldView;

/**
 * The {@link ExecutionType} class keeps a record of data of the target of a message call, whether
 * in the context of a <b>CALL</b>-type instruction or at the initialization of a transaction. It
 * remember in particular whether the target address is delegated or not, and whether the delegate
 * address, if present, points to executable code.
 *
 * @param address
 * @param addressType
 * @param delegate
 * @param delegateType
 */
public record ExecutionType(
    Address address,
    AccountType addressType,
    Optional<Address> delegate,
    Optional<AccountType> delegateType) {

  public Address executionAddress() {
    return delegate.orElse(address);
  }

  public static ExecutionType getExecutionType(Fork fork, WorldView world, Address address) {

    final AccountType accountType = AccountType.getAccountType(fork, world, address);
    if (!accountType.isDelegated()) {
      return new ExecutionType(address, accountType, Optional.empty(), Optional.empty());
    }

    // at this point we know that the recipient is delegated
    // we need to find out whether the delegate has empty code or is delegated
    final Optional<Account> account = Optional.ofNullable(world.get(address));
    checkState(account.isPresent(), "Account should be present for delegated execution type");

    final Bytecode bytecode = new Bytecode(account.get().getCode());
    final Optional<Address> delegateAddress = bytecode.getDelegateAddress();

    checkState(bytecode.isDelegated(), "Bytecode  be delegated for delegated execution type");
    checkState(
        delegateAddress.isPresent(),
        "Delegate address should be present for delegated execution type");

    final AccountType delegateType = AccountType.getAccountType(fork, world, delegateAddress.get());
    return new ExecutionType(address, accountType, delegateAddress, Optional.of(delegateType));
  }

  public static ExecutionType getExecutionType(
      Fork fork, AccountSnapshot account, Optional<AccountSnapshot> delegate) {

    final AccountType accountType = AccountType.getAccountType(fork, account);
    if (!accountType.isDelegated()) {
      return new ExecutionType(account.address(), accountType, Optional.empty(), Optional.empty());
    }

    checkState(delegate.isPresent(), "Delegate should be present for delegated execution type");
    checkState(account.delegationAddress().isPresent(), "Account snapshot should be delegated");
    final AccountSnapshot delegateAccount = delegate.get();
    checkState(
        account.delegationAddress().get().equals(delegateAccount.address()),
        "Inconsistent delegate addresses:"
            + "\n\taccount              address: "
            + account.address()
            + "\n\taccount's delegation address: "
            + account.delegationAddress()
            + "\n\tdelegate             address: "
            + delegateAccount.address());

    final AccountType delegateType = AccountType.getAccountType(fork, delegateAccount);
    return new ExecutionType(
        account.address(),
        accountType,
        Optional.of(delegateAccount.address()),
        Optional.of(delegateType));
  }

  public boolean isSmartContract() {
    return addressType.isSmartContract();
  }

  public boolean isDelegated() {
    return addressType.isDelegated();
  }

  public boolean pointsToExecutableCode() {
    if (isSmartContract()) {
      return true;
    }
    if (isDelegated()) {
      return delegateType.get().isSmartContract() || delegateType.get().isDelegated();
    }
    return false;
  }

  public enum AccountType {
    PRECOMPILE,
    INEXISTENT,
    EMPTY_CODE,
    DELEGATED,
    SMART_CONTRACT;

    public boolean isSmartContract() {
      return this == SMART_CONTRACT;
    }

    public boolean isDelegated() {
      return this == DELEGATED;
    }

    public static AccountType getAccountType(Fork fork, WorldView world, Address address) {
      if (isPrecompile(fork, address)) {
        return PRECOMPILE;
      }

      final Optional<Account> account = Optional.ofNullable(world.get(address));
      if (account.isEmpty()) {
        return INEXISTENT;
      }

      final Bytecode bytecode = new Bytecode(account.get().getCode());
      if (bytecode.isEmpty()) {
        return EMPTY_CODE;
      }

      if (bytecode.isDelegated()) {
        return DELEGATED;
      }

      return SMART_CONTRACT;
    }

    public static AccountType getAccountType(Fork fork, AccountSnapshot accountSnapshot) {
      if (isPrecompile(fork, accountSnapshot.address())) {
        return PRECOMPILE;
      }

      if (accountSnapshot.code().isEmpty()) {
        return EMPTY_CODE;
      }

      if (accountSnapshot.isDelegated()) {
        return DELEGATED;
      }

      return SMART_CONTRACT;
    }
  }
}

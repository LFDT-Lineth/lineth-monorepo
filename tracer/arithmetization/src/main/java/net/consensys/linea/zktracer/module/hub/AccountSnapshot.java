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

import static com.google.common.base.Preconditions.checkArgument;
import static com.google.common.base.Preconditions.checkState;
import static net.consensys.linea.zktracer.Trace.EIP_7702_DELEGATION_INDICATOR;
import static net.consensys.linea.zktracer.types.AddressUtils.isAddressWarm;
import static net.consensys.linea.zktracer.types.AddressUtils.isPrecompile;

import java.util.Optional;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.Setter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.module.hub.transients.Conflation;
import net.consensys.linea.zktracer.module.hub.transients.DeploymentInfo;
import net.consensys.linea.zktracer.module.romlex.ContractMetadata;
import net.consensys.linea.zktracer.module.romlex.RomOperation;
import net.consensys.linea.zktracer.types.Bytecode;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.evm.account.Account;
import org.hyperledger.besu.evm.worldstate.WorldView;

@AllArgsConstructor
@Getter
@Setter
@Accessors(chain = true, fluent = true)
public class AccountSnapshot {

  public static final Bytes EIP_7702_DELEGATION_INDICATOR_BYTES =
      Bytes.minimalBytes(EIP_7702_DELEGATION_INDICATOR);

  private Address address;
  private long nonce;
  private Wei balance;
  private boolean isWarm;
  private Bytecode code;
  private int deploymentNumber;
  private boolean deploymentStatus;
  private int delegationNumber;
  private boolean checkForDelegation;

  static boolean DEFAULT_CHECK_FOR_DELEGATION = false;

  /**
   * {@link AccountSnapshot#canonical} way of creating an {@link AccountSnapshot} for an address.
   *
   * <p><b>Note.</b> We set {@link AccountSnapshot#checkForDelegation} to the default value of
   * {@link AccountSnapshot#DEFAULT_CHECK_FOR_DELEGATION}.
   *
   * @param hub
   * @param address
   * @return
   */
  public static AccountSnapshot canonical(Hub hub, Address address) {
    return fromArguments(
        hub.messageFrame().getWorldUpdater(),
        address,
        hub.transients.conflation(),
        isAddressWarm(hub.fork, hub.messageFrame(), address));
  }

  public static AccountSnapshot canonical(Hub hub, Address address, boolean warmth) {
    return canonical(hub, address).setWarmthTo(warmth);
  }

  public static AccountSnapshot canonicalWithoutFrame(Hub hub, WorldView world, Address address) {
    return fromArguments(
        world, address, hub.transients.conflation(), isPrecompile(hub.fork, address));
  }

  public static AccountSnapshot canonical(Hub hub, WorldView world, Address address) {
    return fromArguments(
        world,
        address,
        hub.transients.conflation(),
        isAddressWarm(hub.fork, hub.messageFrame(), address));
  }

  public static AccountSnapshot canonical(
      Hub hub, WorldView world, Address address, boolean warmth) {
    return fromArguments(world, address, hub.transients.conflation(), warmth);
  }

  private static AccountSnapshot fromArguments(
      final WorldView worldView,
      final Address address,
      final Conflation conflationInfo,
      final boolean warmth) {

    final Account account = worldView.get(address);
    final Bytecode bytecode =
        conflationInfo.deploymentInfo().getDeploymentStatus(address)
            ? new Bytecode(conflationInfo.deploymentInfo().getInitializationCode(address))
            : (account == null) ? new Bytecode(Bytes.EMPTY) : new Bytecode(account.getCode());
    if (account != null) {
      return new AccountSnapshot(
          account.getAddress(),
          account.getNonce(),
          account.getBalance(),
          warmth,
          bytecode,
          conflationInfo.deploymentInfo().deploymentNumber(address),
          conflationInfo.deploymentInfo().getDeploymentStatus(address),
          conflationInfo.getDelegationNumber(address),
          DEFAULT_CHECK_FOR_DELEGATION);
    } else {
      return new AccountSnapshot(
          address,
          0,
          Wei.ZERO,
          warmth,
          bytecode,
          conflationInfo.deploymentInfo().deploymentNumber(address),
          conflationInfo.deploymentInfo().getDeploymentStatus(address),
          conflationInfo.getDelegationNumber(address),
          DEFAULT_CHECK_FOR_DELEGATION);
    }
  }

  public static AccountSnapshot fromAccount(
      Account account,
      boolean isWarm,
      int deploymentNumber,
      boolean deploymentStatus,
      int delegationNumber) {
    return fromAccount(
        Optional.ofNullable(account), isWarm, deploymentNumber, deploymentStatus, delegationNumber);
  }

  public static AccountSnapshot empty(
      boolean isWarm, int deploymentNumber, boolean deploymentStatus) {
    return new AccountSnapshot(
        Address.ZERO,
        0,
        Wei.ZERO,
        isWarm,
        Bytecode.EMPTY,
        deploymentNumber,
        deploymentStatus,
        0,
        DEFAULT_CHECK_FOR_DELEGATION);
  }

  public static AccountSnapshot fromAddress(
      Address address,
      boolean isWarm,
      int deploymentNumber,
      boolean deploymentStatus,
      int delegationNumber) {
    return new AccountSnapshot(
        address,
        0,
        Wei.ZERO,
        isWarm,
        Bytecode.EMPTY,
        deploymentNumber,
        deploymentStatus,
        delegationNumber,
        DEFAULT_CHECK_FOR_DELEGATION);
  }

  public static AccountSnapshot fromAccount(
      Optional<Account> account,
      boolean isWarm,
      int deploymentNumber,
      boolean deploymentStatus,
      int delegationNumber) {

    return account
        .map(
            a ->
                new AccountSnapshot(
                    a.getAddress(),
                    a.getNonce(),
                    a.getBalance().copy(),
                    isWarm,
                    new Bytecode(a.getCode().copy()),
                    deploymentNumber,
                    deploymentStatus,
                    delegationNumber,
                    DEFAULT_CHECK_FOR_DELEGATION))
        .orElseGet(() -> AccountSnapshot.empty(isWarm, deploymentNumber, deploymentStatus));
  }

  /**
   * Creates deep copy of {@code this} {@link AccountSnapshot}.
   *
   * <p><b>Note.</b> The deep copy doesn't copy {@link AccountSnapshot#checkForDelegation}, it uses
   * the default value of {@link AccountSnapshot#DEFAULT_CHECK_FOR_DELEGATION} instead.
   *
   * @return deep copy of {@code this}
   */
  public AccountSnapshot deepCopy() {
    return new AccountSnapshot(
        address,
        nonce,
        balance,
        isWarm,
        code,
        deploymentNumber,
        deploymentStatus,
        delegationNumber,
        DEFAULT_CHECK_FOR_DELEGATION);
  }

  public void wipe(DeploymentInfo deploymentInfo) {
    final boolean deploymentStatus = deploymentInfo.getDeploymentStatus(address);
    checkArgument(!deploymentStatus, "Cannot wipe an account that is under deployment");
    this.nonce(0).balance(Wei.ZERO).code(Bytecode.EMPTY).setDeploymentInfo(deploymentInfo);
  }

  /**
   * Decrements the balance by {@code quantity}. <b>WARNING:</b> this modifies the underlying {@link
   * AccountSnapshot}. Be sure to work with a {@link AccountSnapshot#deepCopy} if necessary.
   *
   * @param quantity
   * @return {@code this} with decremented balance
   */
  public AccountSnapshot decrementBalanceBy(Wei quantity) {
    checkState(
        balance.greaterOrEqualThan(quantity),
        "Insufficient balance"
            + String.format("\n\t\tAddress: %s", address)
            + String.format("\n\t\tBalance: %s", balance)
            + String.format("\n\t\tValue:   %s", quantity));

    balance = balance.subtract(quantity);
    return this;
  }

  /**
   * Increments the balance by {@code quantity}. <b>WARNING:</b> this modifies the underlying {@link
   * AccountSnapshot}. Be sure to work with a {@link AccountSnapshot#deepCopy} if necessary.
   *
   * @param quantity
   * @return {@code this} with incremented balance
   */
  public AccountSnapshot incrementBalanceBy(Wei quantity) {
    balance = balance.add(quantity);
    return this;
  }

  /**
   * {@link AccountSnapshot#setBalanceToZero()} changes the balance of the AccountSnapshot to be
   * zero. <b>WARNING:</b> this modifies the underlying {@link AccountSnapshot}. Be sure to work
   * with a {@link AccountSnapshot#deepCopy} if necessary.
   *
   * @return
   */
  public AccountSnapshot setBalanceToZero() {
    balance = Wei.ZERO;
    return this;
  }

  /**
   * Set the warmth to true. <b>WARNING:</b> this modifies the underlying {@link AccountSnapshot}.
   * Be sure to work with a {@link AccountSnapshot#deepCopy} if necessary.
   *
   * @return {@code this} with warmth = true
   */
  public AccountSnapshot turnOnWarmth() {
    return this.setWarmthTo(true);
  }

  /**
   * Set the warmth to {@code newWarmth}. <b>WARNING:</b> this modifies the underlying {@link
   * AccountSnapshot}. Be sure to work with a {@link AccountSnapshot#deepCopy} if necessary.
   *
   * @param newWarmth
   * @return {@code this} with updated warmth
   */
  public AccountSnapshot setWarmthTo(boolean newWarmth) {
    isWarm(newWarmth);
    return this;
  }

  public AccountSnapshot incrementNonceByOne() {
    return this.nonce(nonce + 1);
  }

  public AccountSnapshot decrementNonceByOne() {
    checkState(
        nonce > 0,
        "AccountSnapshot: attempting to decrement nonce by one when nonce is %s ≤ 0",
        nonce);
    return this.nonce(nonce - 1);
  }

  public AccountSnapshot setDeploymentNumber(Hub hub) {
    return this.setDeploymentNumber(hub.transients.conflation().deploymentInfo());
  }

  public AccountSnapshot setDeploymentNumber(DeploymentInfo deploymentInfo) {
    return this.deploymentNumber(deploymentInfo.deploymentNumber(address));
  }

  public void decrementDeploymentNumberByOne() {
    checkState(
        deploymentNumber > 0,
        "Attempting to decrement deployment number by one when deployment number is %s ≤ 0",
        deploymentNumber);
    this.deploymentNumber(deploymentNumber - 1);
  }

  public AccountSnapshot setDeploymentInfo(Hub hub) {
    return this.setDeploymentInfo(hub.transients.conflation().deploymentInfo());
  }

  public AccountSnapshot setDeploymentInfo(DeploymentInfo deploymentInfo) {
    this.deploymentNumber(deploymentInfo.deploymentNumber(address));
    this.deploymentStatus(deploymentInfo.getDeploymentStatus(address));
    return this;
  }

  public AccountSnapshot setDeploymentStatus(boolean deploymentStatus) {
    this.deploymentStatus(deploymentStatus);
    return this;
  }

  public AccountSnapshot deployByteCode(Bytecode code) {
    checkState(deploymentStatus, "Deployment status should be true before deploying byte code.");

    return new AccountSnapshot(
        address,
        nonce,
        balance,
        true,
        code,
        deploymentNumber,
        false,
        delegationNumber,
        DEFAULT_CHECK_FOR_DELEGATION);
  }

  public EWord tracedCodeHash() {
    return EWord.of(this.deploymentStatus() ? Hash.EMPTY : this.code().getCodeHash());
  }

  public boolean tracedHasCode() {
    return !this.deploymentStatus() && !this.code().isEmpty();
  }

  public boolean isUndergoingDeployment() {
    return this.deploymentStatus();
  }

  public boolean exists() {
    return nonce() != 0 || tracedHasCode() || !balance().isZero();
  }

  public boolean isDelegated() {
    return !isUndergoingDeployment() && code().isDelegated();
  }

  public Optional<Address> delegationAddress() {
    return isDelegated() ? code().getDelegateAddress() : Optional.empty();
  }

  // Set CHECK_FOR_DELEGATION and, if true, trigger the ROM_LEX module
  public AccountSnapshot conditionallyCheckForDelegation(Hub hub, boolean condition) {
    this.checkForDelegation(condition);
    this.actOnDelegationCheckRequest(hub);
    return this;
  }

  public AccountSnapshot checkForDelegationIfAccountHasCode(Hub hub) {
    return this.conditionallyCheckForDelegation(hub, this.tracedHasCode());
  }

  public AccountSnapshot dontCheckForDelegation(Hub hub) {
    return this.conditionallyCheckForDelegation(hub, false);
  }

  public AccountSnapshot incrementDelegationNumberByOne() {
    return this.delegationNumber(delegationNumber + 1);
  }

  public boolean accountHasEmptyCodeOrIsDelegated() {
    return !tracedHasCode() || code().isDelegated();
  }

  private void actOnDelegationCheckRequest(Hub hub) {
    if (checkForDelegation) {
      checkState(
          !code.isEmpty(),
          "Illegal delegation check request: account snapshot at address %s has empty byte code",
          address);
      hub.romLex()
          .operations()
          .add(new RomOperation(this.contractMetadata(), this.code().bytecode(), hub.opCodes()));
    }
  }

  public ContractMetadata contractMetadata() {
    return new ContractMetadata(
        this.address(),
        this.deploymentNumber(),
        this.isUndergoingDeployment(),
        this.delegationNumber());
  }

  /**
   * Delegation checks require that the account
   *
   * <ul>
   *   <li>isn't undergoing deployment
   *   <li>has nonempty code
   *   <li>has code size == 23
   * </ul>
   */
  public void delegationSanityCheck() {
    if (checkForDelegation) {
      checkState(
          !isUndergoingDeployment(),
          "Illegal delegation check request: address %s is undergoing deployment",
          address);
      checkState(
          tracedHasCode(),
          "Illegal delegation check request: address %s has empty byte code",
          address);
      // checkState(
      //     code.getSize() == EOA_DELEGATED_CODE_LENGTH,
      //     "Illegal delegation check request: address %s has code size %s ≠ %s",
      //     address,
      //     code.getSize(),
      //     EOA_DELEGATED_CODE_LENGTH);
    }
  }
}

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
package net.consensys.linea.zktracer.module.hub.section;

import static graphql.com.google.common.base.Preconditions.checkArgument;
import static graphql.com.google.common.base.Preconditions.checkState;
import static net.consensys.linea.zktracer.Trace.GAS_CONST_PER_AUTH_BASE_COST;
import static net.consensys.linea.zktracer.Trace.GAS_CONST_PER_EMPTY_ACCOUNT;
import static net.consensys.linea.zktracer.module.hub.section.TupleAnalysis.*;
import static org.hyperledger.besu.evm.account.Account.MAX_NONCE;

import graphql.com.google.common.base.Preconditions;
import java.math.BigInteger;
import java.util.*;
import lombok.Getter;
import net.consensys.linea.zktracer.module.hub.AccountSnapshot;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.TransactionProcessingType;
import net.consensys.linea.zktracer.module.hub.fragment.AuthorizationFragment;
import net.consensys.linea.zktracer.module.hub.fragment.DomSubStampsSubFragment;
import net.consensys.linea.zktracer.module.hub.fragment.TraceFragment;
import net.consensys.linea.zktracer.module.hub.fragment.account.AccountFragment;
import net.consensys.linea.zktracer.module.hub.fragment.transaction.UserTransactionFragment;
import net.consensys.linea.zktracer.types.Bytecode;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.CodeDelegation;
import org.hyperledger.besu.evm.worldstate.WorldView;

public class TxAuthorizationMacroSection {

  /**
   * <b>latestAccountSnapshots</b> tracks the latest account snapshots of accounts touched during
   * the TX_WARM phase (provided the transaction requires EVM execution) and the TX_AUTH phase.
   *
   * <ul>
   *   <li>(initially) prewarmed addresses if the transaction requires EVM execution
   *   <li>(over time) the successful delegation authorities
   * </ul>
   *
   * <p>Since we don't perform Ethereum state / accrued state updates ourselves, we need to track:
   *
   * <ul>
   *   <li>nonces
   *   <li>warmths
   *   <li>latest delegation addresses
   * </ul>
   *
   * <p>After the present phase these data may get used in the TX_INIT / TX_SKIP phases.
   */
  @Getter public final Map<Address, AccountSnapshot> latestAccountSnapshots;

  public TxAuthorizationMacroSection(
      Hub hub,
      WorldView world,
      TransactionProcessingMetadata txMetadata,
      Map<Address, AccountSnapshot> initialAccountSnapshots) {

    checkArgument(
        txMetadata.requiresAuthorizationPhase(), "Transaction does not require TX_AUTH phase");
    checkArgument(
        txMetadata.getBesuTransaction().codeDelegationListSize() > 0,
        "Transaction has empty delegation list");

    this.latestAccountSnapshots = initialAccountSnapshots;

    final Address senderAddress = txMetadata.getBesuTransaction().getSender();
    int tupleIndex = 0;
    int successfulDelegationsAcc = 0;
    int validSenderIsAuthorityDelegationsAcc = 0;

    /**
     * For each delegation tuple insert an {@link AuthorizationFragment}. If the tuple's signature
     * manages to recover an address, insert an {@link
     * net.consensys.linea.zktracer.module.hub.fragment.account.AccountFragment}
     */
    for (CodeDelegation delegation :
        txMetadata.getBesuTransaction().getCodeDelegationList().get()) {

      tupleIndex++;

      // We use ``hub.stamp() + 1'' since the hub stamp only gets updated in the TraceSection
      // constructor
      final int hubStampPlusOne = hub.stamp() + 1;

      final AuthorizationFragment authorizationFragment =
          new AuthorizationFragment(
              hubStampPlusOne,
              tupleIndex,
              delegation,
              hub.blockdata().getChain().id,
              txMetadata,
              false,
              validSenderIsAuthorityDelegationsAcc,
              0,
              false,
              false);

      // call the RLP_AUTH module for this delegation tuple
      hub.rlpAuth().callRlpAuth(authorizationFragment);

      final BigInteger networkChainId = hub.blockdata().getChain().id;
      final TupleAnalysis preliminaryAnalysis = runPreliminaryAnalysis(delegation, networkChainId);

      // preliminary checks fail
      if (preliminaryAnalysis.failsPreliminaryChecks()) {
        new TxAuthorizationSection(hub, false, authorizationFragment);
        authorizationFragment.tupleAnalysis(preliminaryAnalysis);
        continue;
      }

      // no address could be recovered
      if (delegation.authorizer().isEmpty()) {
        new TxAuthorizationSection(hub, false, authorizationFragment);
        // authorizer is empty only if ec recover fails
        authorizationFragment.tupleAnalysis(TUPLE_FAILS_TO_RECOVER_AUTHORITY_ADDRESS);
        continue;
      }

      final Address authorityAddress = delegation.authorizer().get();

      // senderIsAuthority update
      authorizationFragment.senderIsAuthority(senderAddress.equals(authorityAddress));

      AccountSnapshot authoritySnapshot;
      if (latestAccountSnapshots.containsKey(authorityAddress)) {
        authoritySnapshot = latestAccountSnapshots.get(authorityAddress);
      } else {
        final boolean isWarm = false;
        final int deploymentNumber =
            hub.transients().conflation().deploymentInfo().deploymentNumber(authorityAddress);
        final boolean deploymentStatus =
            hub.transients().conflation().deploymentInfo().getDeploymentStatus(authorityAddress);
        final int delegationNumber = hub.delegationNumberOf(authorityAddress);

        checkState(
            !deploymentStatus, "Addresses in the TX_AUTH phase cannot be undergoing deployment");

        authoritySnapshot =
            world.get(authorityAddress) == null
                ? AccountSnapshot.fromAddress(
                    authorityAddress, isWarm, deploymentNumber, deploymentStatus, delegationNumber)
                : AccountSnapshot.fromAccount(
                    world.get(authorityAddress),
                    isWarm,
                    deploymentNumber,
                    deploymentStatus,
                    delegationNumber);
      }

      /// more updates
      /// ////////////
      authorizationFragment.authorityNonce(authoritySnapshot.nonce());
      authorizationFragment.authorityHasEmptyCodeOrIsDelegated(
          authoritySnapshot.accountHasEmptyCodeOrIsDelegated());

      AccountSnapshot authoritySnapshotNew = authoritySnapshot.deepCopy().turnOnWarmth();

      /// for invalid tuples
      /// //////////////////
      TupleAnalysis secondaryAnalysis =
          runSecondaryAnalysis(delegation, authoritySnapshot, senderAddress, networkChainId);
      authorizationFragment.tupleAnalysis(secondaryAnalysis);
      if (secondaryAnalysis.isInvalid()) {
        AccountFragment authorityAccountFragment =
            hub.factories()
                .accountFragment()
                .makeWithTrmDuringTxAuth(
                    authoritySnapshot,
                    authoritySnapshotNew,
                    authoritySnapshot.address(),
                    DomSubStampsSubFragment.standardDomSubStamps(hubStampPlusOne, 0),
                    TransactionProcessingType.USER)
                .checkForDelegationAuthorizationPhase(hub);
        new TxAuthorizationSection(hub, false, authorizationFragment, authorityAccountFragment);
        latestAccountSnapshots.put(authorityAddress, authoritySnapshotNew);
        continue;
      }

      /// promised update to the authorization fragment
      /// /////////////////////////////////////////////
      authorizationFragment.authorizationTupleIsValid(true);

      if (authoritySnapshot.exists()) {
        successfulDelegationsAcc++;
      }
      Bytecode newCode = authorizationFragment.getBytecode();
      hub.transients().conflation().incrementDelegationNumber(authorityAddress);
      authoritySnapshotNew
          .incrementNonceByOne()
          .delegationNumber(hub.transients().conflation().getDelegationNumber(authorityAddress))
          .code(newCode);

      if (senderIsAuthorityTuple(delegation, senderAddress)) {
        validSenderIsAuthorityDelegationsAcc++;
        authorizationFragment.validSenderIsAuthorityAcc(validSenderIsAuthorityDelegationsAcc);
      }

      AccountFragment authorityAccountFragment =
          hub.factories()
              .accountFragment()
              .makeWithTrmDuringTxAuth(
                  authoritySnapshot,
                  authoritySnapshotNew,
                  authoritySnapshot.address(),
                  DomSubStampsSubFragment.standardDomSubStamps(hubStampPlusOne, 0),
                  TransactionProcessingType.USER)
              .checkForDelegationAuthorizationPhase(hub);

      new TxAuthorizationSection(
          hub, authoritySnapshot.exists(), authorizationFragment, authorityAccountFragment);

      // updates
      latestAccountSnapshots.put(authorityAddress, authoritySnapshotNew);
    }

    txMetadata.setNumberOfSuccessfulDelegations(successfulDelegationsAcc);
    txMetadata.setNumberOfSuccessfulSenderDelegations(validSenderIsAuthorityDelegationsAcc);

    // we finish by including a PEEK_AT_TRANSACTION row
    // this is expected to raise the hub.stamp() so we make it its own TraceSection.
    new TxAuthorizationSection(hub);
  }

  boolean senderIsAuthorityTuple(CodeDelegation delegation, Address senderAddress) {
    return delegation.authorizer().isPresent()
        && delegation.authorizer().get().equals(senderAddress);
  }

  /**
   * Logic shamelessly stolen from <a
   * href=https://github.com/hyperledger/besu/blob/bba22edc005cabab975efe39d98977b666f2bc83/ethereum/core/src/main/java/org/hyperledger/besu/ethereum/mainnet/CodeDelegationProcessor.java#L86">CodeDelegationProcessor.java</a>
   *
   * <p>Documentation taken from <a href="https://eips.ethereum.org/EIPS/eip-7702">the EIP</a>.
   */
  TupleAnalysis runSecondaryAnalysis(
      CodeDelegation delegation,
      AccountSnapshot latestAccountSnapshot,
      Address senderAddress,
      BigInteger networkChainId) {
    // sanity check
    Preconditions.checkArgument(
        runPreliminaryAnalysis(delegation, networkChainId).passesPreliminaryChecks());

    // we duplicate the remaining logic of CodeDelegationProcessor.java
    // 3. Let authority = ecrecover(msg, y_parity, r, s)
    final Optional<Address> authority = delegation.authorizer();
    if (authority.isEmpty()) {
      return TUPLE_FAILS_TO_RECOVER_AUTHORITY_ADDRESS;
    }

    // sanity check
    checkState(
        authority.get().equals(latestAccountSnapshot.address()),
        "Account snapshot / delegation authority mismatch:"
            + "snapshot address:  %s,"
            + "authority address: %s",
        latestAccountSnapshot.address(),
        authority.get());

    // 4: noop
    // 5. Verify the code of authority is empty or already delegated
    if (!latestAccountSnapshot.accountHasEmptyCodeOrIsDelegated()) {
      return TUPLE_FAILS_DUE_TO_AUTHORITY_NEITHER_HAVING_EMPTY_CODE_NOR_BEING_DELEGATED;
    }

    // 6. Verify the nonce of authority is equal to nonce
    if (delegation.nonce()
        != latestAccountSnapshot.nonce()
            + (senderIsAuthorityTuple(delegation, senderAddress) ? 1 : 0)) {
      return TUPLE_FAILS_DUE_TO_NONCE_MISMATCH;
    }

    // 7: noop
    // 8: noop
    // 9: noop

    return TUPLE_IS_VALID;
  }

  TupleAnalysis runPreliminaryAnalysis(CodeDelegation delegation, BigInteger networkChainId) {
    /*
     * NOTE: this seems to be the correct definition of <b>halfCurveOrder</b>, compare with
     * https://github.com/ethereum/execution-specs/blob/a32148175b3ea1db5a34caba939627af5be60c9a/tests/prague/eip7702_set_code_tx/test_set_code_txs.py#L2485
     */
    BigInteger halfCurveOrder =
        new BigInteger("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)
            .shiftRight(1);

    // 1. Verify the chain ID is 0 or the ID of the current chain.
    final boolean delegationTupleChainIdIsZeroOrMatchesNetworkChainId =
        (delegation.chainId().equals(BigInteger.ZERO)
            || delegation.chainId().equals(networkChainId));
    if (!delegationTupleChainIdIsZeroOrMatchesNetworkChainId) {
      return TupleAnalysis.TUPLE_FAILS_CHAIN_ID_CHECK;
    }

    // 2. Verify the nonce is less than 2**64 - 1.
    if (delegation.nonce() == MAX_NONCE) {
      return TupleAnalysis.TUPLE_FAILS_NONCE_RANGE_CHECK;
    }

    // 3.i: noop
    // 3.ii Verify s is less than or equal to secp256k1n/2
    if (delegation.signature().getS().compareTo(halfCurveOrder) > 0) {
      return TupleAnalysis.TUPLE_FAILS_S_RANGE_CHECK;
    }

    // the tuple may still be invalid, however it is not invalid due to preliminary checks, so we
    // return TUPLE_PASSES_PRELIMINARY_CHECKS
    return TupleAnalysis.TUPLE_PASSES_PRELIMINARY_CHECKS;
  }

  public static class TxAuthorizationSection extends TraceSection {
    public TxAuthorizationSection(
        Hub hub, boolean authorityTupleIsValidAndAuthorityExists, TraceFragment... fragment) {
      super(hub, (short) 2);

      // valid authority tuples whose underlying authority account exists accrue refunds
      if (authorityTupleIsValidAndAuthorityExists) {
        commonValues.refundDelta(GAS_CONST_PER_EMPTY_ACCOUNT - GAS_CONST_PER_AUTH_BASE_COST);
      }
      this.addFragments(fragment);
    }

    /**
     * Adds the final TX_AUTH-phase fragment (which is a single PEEK_AT_TRANSACTION row)
     *
     * @param hub
     */
    public TxAuthorizationSection(Hub hub) {
      super(hub, (short) 1);
      UserTransactionFragment currentTransaction =
          hub.txStack().current().userTransactionFragment();
      this.addFragments(currentTransaction);
    }
  }
}

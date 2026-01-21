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

package net.consensys.linea.zktracer.module.hub.fragment.account;

import static com.google.common.base.Preconditions.checkArgument;
import static com.google.common.base.Preconditions.checkState;
import static net.consensys.linea.zktracer.Trace.Hub.MULTIPLIER___DOM_SUB_STAMPS;
import static net.consensys.linea.zktracer.module.hub.TransactionProcessingType.USER;
import static net.consensys.linea.zktracer.module.hub.TransactionProcessingType.isUserTransaction;
import static net.consensys.linea.zktracer.types.AddressUtils.*;

import java.util.Map;
import java.util.Optional;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.Setter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Fork;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.AccountSnapshot;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.TransactionProcessingType;
import net.consensys.linea.zktracer.module.hub.defer.DeferRegistry;
import net.consensys.linea.zktracer.module.hub.defer.EndTransactionDefer;
import net.consensys.linea.zktracer.module.hub.defer.PostBlockDefer;
import net.consensys.linea.zktracer.module.hub.defer.PostConflationDefer;
import net.consensys.linea.zktracer.module.hub.fragment.DomSubStampsSubFragment;
import net.consensys.linea.zktracer.module.hub.fragment.TraceFragment;
import net.consensys.linea.zktracer.module.hub.section.halt.EphemeralAccount;
import net.consensys.linea.zktracer.types.EWord;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.evm.worldstate.WorldView;

@Accessors(fluent = true)
public final class AccountFragment
    implements TraceFragment, EndTransactionDefer, PostBlockDefer, PostConflationDefer {
  private final Fork fork;
  private final TransactionProcessingMetadata tx;
  private final TransactionProcessingType txType;
  @Getter private final AccountSnapshot oldState;
  @Getter private final AccountSnapshot newState;
  @Setter private boolean requiresRomlex;
  private int codeFragmentIndex;
  private final Optional<Bytes> addressToTrim;
  @Getter private final DomSubStampsSubFragment domSubStampsSubFragment;
  @Setter private RlpAddrSubFragment rlpAddrSubFragment;
  final int hubStamp;
  @Getter final TransactionProcessingMetadata transactionProcessingMetadata;
  protected boolean markedForDeletion;
  protected boolean markedForDeletionNew;

  /**
   * {@link AccountFragment} creation requires access to a {@link DeferRegistry} for post-conflation
   * data gathering, which is provided by this factory.
   */
  @RequiredArgsConstructor
  public static class AccountFragmentFactory {
    private final Hub hub;

    public AccountFragment make(
        AccountSnapshot oldState,
        AccountSnapshot newState,
        DomSubStampsSubFragment domSubStampsSubFragment,
        TransactionProcessingType txProcessingType) {
      return new AccountFragment(
          hub, oldState, newState, Optional.empty(), domSubStampsSubFragment, txProcessingType);
    }

    public AccountFragment makeWithTrm(
        AccountSnapshot oldState,
        AccountSnapshot newState,
        Bytes toTrim,
        DomSubStampsSubFragment domSubStampsSubFragment,
        TransactionProcessingType txProcessingType) {
      hub.trm().callTrimming(toTrim);
      return new AccountFragment(
          hub, oldState, newState, Optional.of(toTrim), domSubStampsSubFragment, txProcessingType);
    }
  }

  public AccountFragment(
      Hub hub,
      AccountSnapshot oldState,
      AccountSnapshot newState,
      Optional<Bytes> addressToTrim,
      DomSubStampsSubFragment domSubStampsSubFragment,
      TransactionProcessingType txProcessingType) {
    checkArgument(
        oldState.address().equals(newState.address()),
        "AccountFragment: address mismatch in constructor");
    fork = hub.fork;
    transactionProcessingMetadata = txProcessingType == USER ? hub.txStack().current() : null;
    hubStamp = hub.stamp();
    this.oldState = oldState;
    this.newState = newState;
    this.addressToTrim = addressToTrim;
    this.domSubStampsSubFragment = domSubStampsSubFragment;
    txType = txProcessingType;
    if (isUserTransaction(txType)) {
      tx = hub.txStack().current();
      tx.updateHadCodeInitially(
          oldState.address(),
          domSubStampsSubFragment.domStamp(),
          domSubStampsSubFragment.subStamp(),
          oldState().tracedHasCode());
    } else {
      tx = null;
    }

    // This allows us to properly fill EXISTS_INFTY, DEPLOYMENT_NUMBER_INFTY and CODE_FRAGMENT_INDEX
    hub.defers().scheduleForPostConflation(this);

    // This allows us to properly fill MARKED_FOR_SELFDESTRUCT/DELETION(_NEW), among other things
    if (txProcessingType == USER) {
      hub.defers().scheduleForEndTransaction(this);
    }

    // This allows us to keep track of account that are accessed by the HUB during the execution of
    // the block
    hub.defers().scheduleForPostBlock(this);
  }

  @Override
  public Trace.Hub trace(Trace.Hub trace) {

    // tracing
    domSubStampsSubFragment.traceHub(trace);
    if (rlpAddrSubFragment != null) {
      rlpAddrSubFragment.traceHub(trace);
    }

    final boolean hasCode = oldState.tracedHasCode();
    final boolean hasCodeNew = newState.tracedHasCode();

    trace
        .peekAtAccount(true)
        .pAccountAddressHi(highPart(oldState.address()))
        .pAccountAddressLo(lowPart(oldState.address()))
        .pAccountNonce(Bytes.ofUnsignedLong(oldState.nonce()))
        .pAccountNonceNew(Bytes.ofUnsignedLong(newState.nonce()))
        .pAccountBalance(oldState.balance())
        .pAccountBalanceNew(newState.balance())
        .pAccountCodeSize(oldState.code().getSize())
        .pAccountCodeSizeNew(newState.code().getSize())
        .pAccountCodeHashHi(oldState.tracedCodeHash().hi())
        .pAccountCodeHashLo(oldState.tracedCodeHash().lo())
        .pAccountCodeHashHiNew(newState.tracedCodeHash().hi())
        .pAccountCodeHashLoNew(newState.tracedCodeHash().lo())
        .pAccountHasCode(hasCode)
        .pAccountHasCodeNew(hasCodeNew)
        .pAccountCodeFragmentIndex(codeFragmentIndex)
        .pAccountRomlexFlag(requiresRomlex)
        .pAccountExists(oldState.nonce() > 0 || hasCode || !oldState.balance().isZero())
        .pAccountExistsNew(newState.nonce() > 0 || hasCodeNew || !newState.balance().isZero())
        .pAccountWarmth(oldState.isWarm())
        .pAccountWarmthNew(newState.isWarm())
        .pAccountDeploymentNumber(oldState.deploymentNumber())
        .pAccountDeploymentStatus(oldState.deploymentStatus())
        .pAccountDeploymentNumberNew(newState.deploymentNumber())
        .pAccountDeploymentStatusNew(newState.deploymentStatus())
        .pAccountTrmFlag(addressToTrim.isPresent())
        .pAccountTrmRawAddressHi(addressToTrim.map(a -> EWord.of(a).hi()).orElse(Bytes.EMPTY))
        .pAccountIsPrecompile(isPrecompile(fork, oldState().address()))
        .pAccountMarkedForDeletion(markedForDeletion)
        .pAccountMarkedForDeletionNew(markedForDeletionNew);
    traceHadCodeInitially(trace);

    return trace;
  }

  private void traceHadCodeInitially(Trace.Hub trace) {
    trace.pAccountHadCodeInitially(
        isUserTransaction(txType)
            ? tx.hadCodeInitiallyMap().get(oldState().address()).hadCode()
            : oldState().tracedHasCode());
  }

  private boolean shouldBeMarkedForDeletion() {
    return !transactionProcessingMetadata.hadCodeInitiallyMap().get(oldState().address()).hadCode();
  }

  @Override
  public void resolveAtEndTransaction(
      Hub hub, WorldView state, Transaction tx, boolean isSuccessful) {
    final Map<EphemeralAccount, Integer> effectiveSelfDestructMap =
        transactionProcessingMetadata.getEffectiveSelfDestructMap();
    final EphemeralAccount ephemeralAccount =
        new EphemeralAccount(oldState().address(), oldState().deploymentNumber());
    if (shouldBeMarkedForDeletion() && effectiveSelfDestructMap.containsKey(ephemeralAccount)) {
      final int selfDestructTime = effectiveSelfDestructMap.get(ephemeralAccount);
      markedForDeletion =
          domSubStampsSubFragment().domStamp() > MULTIPLIER___DOM_SUB_STAMPS * selfDestructTime;
      markedForDeletionNew = hubStamp >= selfDestructTime;
    } else {
      markedForDeletion = false;
      markedForDeletionNew = false;
    }
  }

  @Override
  public void resolvePostBlock(Hub hub) {
    hub.blockStack().currentBlock().addAddressSeenByHub(oldState.address());
  }

  @Override
  public void resolvePostConflation(Hub hub, WorldView world) {
    // Note: we DO need the CFI, even if we don't have the romlex flag on, for CFI consistency
    // argument.
    try {
      codeFragmentIndex =
          hub.getCodeFragmentIndexByMetaData(
              newState.address(),
              newState.deploymentNumber(),
              newState.deploymentStatus(),
              newState.delegationNumber());
    } catch (RuntimeException e) {
      // getCfi should NEVER throw en exception when requiresRomLex ≡ true
      checkState(
          !requiresRomlex,
          "AccountFragment: can't get an exception to get CFI when RomLex is required");
      codeFragmentIndex = 0;
    }
  }
}

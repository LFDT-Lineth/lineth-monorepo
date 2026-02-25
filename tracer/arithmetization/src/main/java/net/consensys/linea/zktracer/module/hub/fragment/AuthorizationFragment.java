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
package net.consensys.linea.zktracer.module.hub.fragment;

import static graphql.com.google.common.base.Preconditions.checkState;
import static net.consensys.linea.zktracer.Trace.*;

import java.math.BigInteger;
import lombok.Getter;
import lombok.Setter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.section.TupleAnalysis;
import net.consensys.linea.zktracer.types.Bytecode;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.CodeDelegation;

/**
 * The <b>RLP_AUTH</b> module will consume an {@link AuthorizationFragment}. These are created in
 * the main {@link net.consensys.linea.zktracer.module.hub.section.TxAuthorizationMacroSection}
 * loop. They contain most of the ``outside data'' that is required.
 *
 * <ul>
 *   <li>[x] <b>delegation tuple</b>
 *   <li>[x] tuple index
 *   <li>[x] authority nonce (defaults to 0)
 *   <li>[x] isValidSenderIsAuthorityTuple (defaults to false)
 *   <li>[x] authorityHasEmptyCodeOrIsDelegated (defaults to false)
 *   <li>[x] <b>txMetadata</b>
 *   <li>[x] networkChainId
 *   <li>[x] hubStamp
 * </ul>
 *
 * <p><b>delegation tuple</b> (ok, derived from delegation tuple)
 *
 * <ul>
 *   <li>[x] tuple nonce
 *   <li>[x] tuple chain id
 *   <li>[x] tuple address
 *   <li>[x] Optional(authority address)
 * </ul>
 *
 * <p>transaction processing metadata (ok, derived from <b>txMetadata</b>)
 *
 * <ul>
 *   <li>[x] block number
 *   <li>[x] user transaction number
 *   <li>[x] from address
 * </ul>
 *
 * <p><b>Note:</b> validSenderIsAuthorityAcc isn't required by <b>RLP_AUTH</b>
 */
@Accessors(fluent = true)
@Setter
@Getter
public class AuthorizationFragment implements TraceFragment {

  final int hubStamp;
  final int tupleIndex;
  final CodeDelegation delegation;
  final TransactionProcessingMetadata txMetadata;
  final BigInteger networkChainId;

  // fields below require successful authority recovery and are often updated post fragment creation
  boolean senderIsAuthority;
  int validSenderIsAuthorityAcc;
  long authorityNonce;
  boolean authorityHasEmptyCodeOrIsDelegated;
  boolean authorizationTupleIsValid;

  // field to specify the potential reason for invalidity of the tuple
  TupleAnalysis tupleAnalysis;

  public AuthorizationFragment(
      int hubStamp,
      int tupleIndex,
      CodeDelegation delegation,
      BigInteger networkChainId,
      TransactionProcessingMetadata txMetadata,
      boolean senderIsAuthority,
      int validSenderIsAuthorityAcc,
      long authorityNonce,
      boolean authorityHasEmptyCodeOrIsDelegated,
      boolean authorizationTupleIsValid) {

    this.hubStamp = hubStamp;
    this.tupleIndex = tupleIndex;
    this.delegation = delegation;
    this.txMetadata = txMetadata;
    this.networkChainId = networkChainId;

    this.senderIsAuthority = senderIsAuthority;
    this.validSenderIsAuthorityAcc = validSenderIsAuthorityAcc;
    this.authorityNonce = authorityNonce;
    this.authorityHasEmptyCodeOrIsDelegated = authorityHasEmptyCodeOrIsDelegated;
    this.authorizationTupleIsValid = authorizationTupleIsValid;
  }

  public Bytecode getBytecode() {
    if (delegation.address().equals(Address.ZERO)) {
      return Bytecode.EMPTY;
    }

    String bytecodeHexString =
        Bytes.ofUnsignedLong(EIP_7702_DELEGATION_INDICATOR)
                .trimLeadingZeros()
                .toHexString()
                .substring(2)
            + delegation.address().toHexString().substring(2);

    checkState(
        bytecodeHexString.length() == 2 * EOA_DELEGATED_CODE_LENGTH,
        "Invalid delegated bytecode length");

    return new Bytecode(Bytes.fromHexString(bytecodeHexString));
  }

  @Override
  public Trace.Hub trace(Trace.Hub trace) {

    final Address authorityAddressOrZero = delegation.authorizer().orElse(Address.ZERO);

    return trace
        .peekAtAuthorization(true)
        .pAuthTupleIndex(tupleIndex)
        .pAuthAuthorityEcrecoverSuccess(tracedEcRecoverSuccess())
        .pAuthSenderIsAuthority(tracedSenderIsAuthority())
        .pAuthSenderIsAuthorityAcc(
            validSenderIsAuthorityAcc) // this column is always present in hub.auth/
        .pAuthAuthorityAddressHi(tracedAuthorityAddress().slice(0, 4).toLong())
        .pAuthAuthorityAddressLo(tracedAuthorityAddress().slice(4, LLARGE))
        .pAuthAuthorityNonce(Bytes.ofUnsignedLong(tracedAuthorityNonce()))
        .pAuthAuthorityHasEmptyCodeOrIsDelegated(
            tracedAuthorityHasEmptyCodeOrIsDelegated()) // don't simplify
        .pAuthAuthorizationTupleIsValid(authorizationTupleIsValid)
        .pAuthDelegationAddressHi(delegation.address().slice(0, 4).toLong())
        .pAuthDelegationAddressLo(delegation.address().slice(4, LLARGE))
        .pAuthDelegationAddressIsZero(delegation.address().equals(Address.ZERO));
  }

  private boolean traceAccountData() {
    return tupleAnalysis.passesPreliminaryChecks() && delegation().authorizer().isPresent();
  }

  public boolean tracedSenderIsAuthority() {
    return traceAccountData() && senderIsAuthority;
  }

  public boolean tracedEcRecoverSuccess() {
    return traceAccountData() && delegation().authorizer().isPresent();
  }

  public long tracedAuthorityNonce() {
    return traceAccountData() ? authorityNonce : 0;
  }

  public boolean tracedAuthorityHasEmptyCodeOrIsDelegated() {
    return traceAccountData() && authorityHasEmptyCodeOrIsDelegated;
  }

  public Address tracedAuthorityAddress() {
    return traceAccountData() ? delegation().authorizer().get() : Address.ZERO;
  }
}

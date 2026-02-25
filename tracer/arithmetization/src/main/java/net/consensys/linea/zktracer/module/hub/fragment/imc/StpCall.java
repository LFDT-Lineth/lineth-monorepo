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

package net.consensys.linea.zktracer.module.hub.fragment.imc;

import static com.google.common.base.Preconditions.*;
import static net.consensys.linea.zktracer.Trace.GAS_CONST_G_CALL_STIPEND;
import static net.consensys.linea.zktracer.Trace.GAS_CONST_G_CALL_VALUE;
import static net.consensys.linea.zktracer.Trace.GAS_CONST_G_COLD_ACCOUNT_ACCESS;
import static net.consensys.linea.zktracer.Trace.GAS_CONST_G_CREATE;
import static net.consensys.linea.zktracer.Trace.GAS_CONST_G_NEW_ACCOUNT;
import static net.consensys.linea.zktracer.Trace.GAS_CONST_G_WARM_ACCESS;
import static net.consensys.linea.zktracer.types.AddressUtils.isAddressWarm;
import static net.consensys.linea.zktracer.types.EWord.ZERO;

import java.math.BigInteger;
import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.Setter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.AccountSnapshot;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.TraceSubFragment;
import net.consensys.linea.zktracer.module.hub.signals.Exceptions;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.evm.account.Account;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.internal.Words;

@Getter
@Setter
@Accessors(fluent = true)
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class StpCall implements TraceSubFragment {
  @EqualsAndHashCode.Include final long memoryExpansionGas;
  @EqualsAndHashCode.Include OpCode opCode;
  @EqualsAndHashCode.Include long gasActual;
  @EqualsAndHashCode.Include EWord gas; // for CALL's only
  @EqualsAndHashCode.Include EWord value;
  @EqualsAndHashCode.Include boolean exists;
  @EqualsAndHashCode.Include boolean warm;
  @EqualsAndHashCode.Include boolean isDelegated = false;
  @EqualsAndHashCode.Include boolean isDelegatedToSelf = false;
  @EqualsAndHashCode.Include boolean delegateWarmth = false;
  @EqualsAndHashCode.Include long upfrontGasCost;
  @EqualsAndHashCode.Include boolean outOfGasException;
  @EqualsAndHashCode.Include long gasPaidOutOfPocket;
  @EqualsAndHashCode.Include long stipend;
  private final OpCodeData opCodeData;

  public StpCall(Hub hub, MessageFrame frame, long memoryExpansionGas) {
    opCodeData = hub.opCodeData();
    opCode = opCodeData.mnemonic();

    checkArgument(
        opCodeData.isCall() || opCodeData.isCreate(),
        "STP: opCode %s is not a CALL or CREATE opcode",
        opCode);

    this.memoryExpansionGas = memoryExpansionGas;
    this.gasActual = frame.getRemainingGas();

    if (this.opCodeData.isCall()) {
      this.stpCallForCalls(hub, frame, opCode);
    } else {
      this.stpCallForCreates(frame);
    }
  }

  private void stpCallForCalls(Hub hub, MessageFrame frame, OpCode opCode) {

    final Address calleeAddress = Words.toAddress(frame.getStackItem(1));
    final Account calleeAccount = frame.getWorldUpdater().get(calleeAddress);
    this.gas = EWord.of(frame.getStackItem(0));
    this.value = opCodeData.callHasValueArgument() ? EWord.of(frame.getStackItem(2)) : ZERO;
    this.exists =
        switch (opCode) {
          case CALL -> calleeAccount != null && !calleeAccount.isEmpty();
          case CALLCODE, DELEGATECALL, STATICCALL -> false;
          default ->
              throw new IllegalArgumentException(
                  "STP module triggered for a non CALL-type instruction");
        };
    this.warm = isAddressWarm(hub.fork, frame, calleeAddress);

    AccountSnapshot callee = AccountSnapshot.canonical(hub, calleeAddress);
    if (callee.isDelegated()) {
      checkState(
          callee.delegationAddress().isPresent(),
          "STP: delegated callee should have a delegation address");
      this.isDelegated = true;
      final Address delegateAddress = callee.delegationAddress().get();
      this.isDelegatedToSelf = delegateAddress.equals(calleeAddress);
      // if the instruction raises an exception then we keep the same warmth
      // otherwise we have to use the updated value of the delegate's warmth
      // at this point the only account that may have changed warmth is the callee,
      // thus we set:
      // - if delegated to self,  then delegateWarmth = true
      // - if delegated to other, then delegateWarmth = isAddressWarm(delegateAddress)
      this.delegateWarmth =
          Exceptions.any(hub.pch().exceptions())
              ? isAddressWarm(hub.fork, frame, delegateAddress)
              : this.isDelegatedToSelf || isAddressWarm(hub.fork, frame, delegateAddress);
    }

    this.upfrontGasCost = upfrontGasCostForCalls();
    this.outOfGasException = gasActual < upfrontGasCost;
    this.gasPaidOutOfPocket = gasPaidOutOfPocketForCalls();
    this.stipend = !outOfGasException && nonzeroValueTransfer() ? GAS_CONST_G_CALL_STIPEND : 0;
  }

  private boolean nonzeroValueTransfer() {
    return opCodeData.callHasValueArgument() && !value.isZero();
  }

  private boolean callWouldLeadToAccountCreation() {
    return (opCode == OpCode.CALL) && nonzeroValueTransfer() && !exists;
  }

  private long gasPaidOutOfPocketForCalls() {
    if (outOfGasException) {
      return gasPaidOutOfPocket = 0;
    } else {
      long gasMinusUpfront = gasActual - upfrontGasCost;
      long oneSixtyFourths = gasMinusUpfront >> 6;
      long maxGasAllowance = gasMinusUpfront - oneSixtyFourths;
      return gas().toUnsignedBigInteger().compareTo(BigInteger.valueOf(maxGasAllowance)) > 0
          ? maxGasAllowance
          : gas.toLong();
    }
  }

  private void stpCallForCreates(MessageFrame frame) {

    this.gas = ZERO; // irrelevant
    this.value = EWord.of(frame.getStackItem(0));
    this.exists = false; // irrelevant
    this.warm = false; // irrelevant
    this.upfrontGasCost = GAS_CONST_G_CREATE + memoryExpansionGas;
    this.outOfGasException = gasActual < upfrontGasCost;
    this.gasPaidOutOfPocket = computeGasPaidOutOfPocketForCreates();
    this.stipend = 0; // irrelevant
  }

  private long computeGasPaidOutOfPocketForCreates() {
    if (outOfGasException) {
      return 0;
    } else {
      long gasMinusUpfrontCost = gasActual - upfrontGasCost;
      return gasMinusUpfrontCost - gasMinusUpfrontCost / 64;
    }
  }

  private long upfrontGasCostForCalls() {

    long upfrontGasCost = memoryExpansionGas;
    upfrontGasCost += nonzeroValueTransfer() ? GAS_CONST_G_CALL_VALUE : 0;
    upfrontGasCost += warm ? GAS_CONST_G_WARM_ACCESS : GAS_CONST_G_COLD_ACCOUNT_ACCESS;
    upfrontGasCost += callWouldLeadToAccountCreation() ? GAS_CONST_G_NEW_ACCOUNT : 0;
    if (isDelegated) {
      upfrontGasCost +=
          isDelegatedToSelf || delegateWarmth
              ? GAS_CONST_G_WARM_ACCESS
              : GAS_CONST_G_COLD_ACCOUNT_ACCESS;
    }

    return upfrontGasCost;
  }

  public long effectiveChildContextGasAllowance() {
    return gasPaidOutOfPocket + stipend;
  }

  @Override
  public Trace.Hub traceHub(Trace.Hub trace) {
    return trace
        .pMiscStpFlag(true)
        .pMiscStpInstruction(opCode.unsignedByteValue())
        .pMiscStpGasHi(gas.hi())
        .pMiscStpGasLo(gas.lo())
        .pMiscStpValueHi(value.hi())
        .pMiscStpValueLo(value.lo())
        .pMiscStpExists(exists)
        .pMiscStpWarmth(warm)
        .pMiscStpOogx(outOfGasException)
        .pMiscStpGasMxp(Bytes.ofUnsignedLong(memoryExpansionGas))
        .pMiscStpGasUpfrontGasCost(Bytes.ofUnsignedLong(upfrontGasCost))
        .pMiscStpGasPaidOutOfPocket(Bytes.ofUnsignedLong(gasPaidOutOfPocket))
        .pMiscStpGasStipend(stipend)
        .pMiscStpCalleeIsDelegated(isDelegated)
        .pMiscStpCalleeIsDelegatedToSelf(isDelegatedToSelf)
        .pMiscStpDelegateWarmth(delegateWarmth);
  }

  public int compareTo(StpCall stpCall) {
    final int opCodeComp = opCode.compareTo(stpCall.opCode);
    if (opCodeComp != 0) {
      return opCodeComp;
    }

    final int gasActualComp = Long.compare(gasActual, stpCall.gasActual);
    if (gasActualComp != 0) {
      return gasActualComp;
    }

    final int memoryExpansionGasComp = Long.compare(memoryExpansionGas, stpCall.memoryExpansionGas);
    if (memoryExpansionGasComp != 0) {
      return memoryExpansionGasComp;
    }

    final int gasPaidOutOfPocketComp = Long.compare(gasPaidOutOfPocket, stpCall.gasPaidOutOfPocket);
    if (gasPaidOutOfPocketComp != 0) {
      return gasPaidOutOfPocketComp;
    }

    final int stipendComp = Long.compare(stipend, stpCall.stipend);
    if (stipendComp != 0) {
      return stipendComp;
    }

    final int upfrontGasCostComp = Long.compare(upfrontGasCost, stpCall.upfrontGasCost);
    if (upfrontGasCostComp != 0) {
      return upfrontGasCostComp;
    }

    final boolean existsComp = exists == stpCall.exists;
    if (!existsComp) {
      return exists ? 1 : -1;
    }

    final boolean warmComp = warm == stpCall.warm;
    if (!warmComp) {
      return warm ? 1 : -1;
    }

    final boolean outOfGasExceptionComp = outOfGasException == stpCall.outOfGasException;
    if (!outOfGasExceptionComp) {
      return outOfGasException ? 1 : -1;
    }

    final int valueComp = value.compareTo(stpCall.value);
    if (valueComp != 0) {
      return valueComp;
    }

    return gas.compareTo(stpCall.gas);
  }
}

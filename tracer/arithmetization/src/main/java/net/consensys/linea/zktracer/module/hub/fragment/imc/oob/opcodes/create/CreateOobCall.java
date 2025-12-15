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

package net.consensys.linea.zktracer.module.hub.fragment.imc.oob.opcodes.create;

import static net.consensys.linea.zktracer.Trace.*;
import static net.consensys.linea.zktracer.module.hub.fragment.imc.oob.opcodes.CallOobCall.MAX_CALL_STACK_DEPTH_BYTES;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToIsZero;
import static net.consensys.linea.zktracer.module.oob.OobExoCall.callToLT;
import static net.consensys.linea.zktracer.types.AddressUtils.getDeploymentAddress;
import static net.consensys.linea.zktracer.types.Conversions.*;

import java.math.BigInteger;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.Setter;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.add.Add;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.OobCall;
import net.consensys.linea.zktracer.module.mod.Mod;
import net.consensys.linea.zktracer.module.oob.OobExoCall;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.evm.account.Account;
import org.hyperledger.besu.evm.frame.MessageFrame;

@Getter
@Setter
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public abstract class CreateOobCall extends OobCall {

  private static final Bytes EIP2681_MAX_NONCE_BYTES = bigIntegerToBytes(EIP2681_MAX_NONCE);

  // Inputs
  @EqualsAndHashCode.Include EWord value;
  @EqualsAndHashCode.Include Bytes balance;
  @EqualsAndHashCode.Include Bytes nonce;
  @EqualsAndHashCode.Include boolean hasCode;
  @EqualsAndHashCode.Include Bytes callStackDepth;
  @EqualsAndHashCode.Include Bytes creatorNonce;
  @EqualsAndHashCode.Include long codeSize;

  // Outputs
  boolean abortingCondition;
  boolean failureCondition;

  public CreateOobCall() {
    super();
  }

  @Override
  public void setInputData(MessageFrame frame, Hub hub) {
    final Account creatorAccount = frame.getWorldUpdater().get(frame.getRecipientAddress());
    final Address deploymentAddress = getDeploymentAddress(frame, hub.opCodeData(frame));
    final Account deployedAccount = frame.getWorldUpdater().get(deploymentAddress);

    final boolean unaborted = hub.pch().abortingConditions().snapshot().none();
    final boolean unabortedCreateAndDeploymentAccountExists =
        ((deployedAccount != null) && unaborted);

    final long nonce = unabortedCreateAndDeploymentAccountExists ? deployedAccount.getNonce() : 0;
    final boolean hasCode = unabortedCreateAndDeploymentAccountExists && deployedAccount.hasCode();

    setValue(EWord.of(frame.getStackItem(0)));
    setBalance(bigIntegerToBytes(creatorAccount.getBalance().toUnsignedBigInteger()));
    setNonce(Bytes.minimalBytes(nonce));
    setHasCode(hasCode);
    setCallStackDepth(bigIntegerToBytes(BigInteger.valueOf(frame.getDepth())));
    setCreatorNonce(Bytes.minimalBytes(creatorAccount.getNonce()));

    codeSizeSnapshot(frame);
  }

  protected abstract void codeSizeSnapshot(final MessageFrame frame);

  @Override
  public void callExoModulesAndSetOutputs(Add add, Mod mod, Wcp wcp) {
    // row i
    final OobExoCall insufficientBalanceCall = callToLT(wcp, balance, value);
    exoCalls.add(insufficientBalanceCall);
    final boolean insufficientBalanceAbort = bytesToBoolean(insufficientBalanceCall.result());

    // row i + 1
    final OobExoCall callStackDepthAbortCall =
        callToLT(wcp, callStackDepth, MAX_CALL_STACK_DEPTH_BYTES);
    exoCalls.add(callStackDepthAbortCall);
    final boolean callStackDepthAbort = !bytesToBoolean(callStackDepthAbortCall.result());

    // row i + 2
    final OobExoCall nonzeroNonceCall = callToIsZero(wcp, nonce);
    exoCalls.add(nonzeroNonceCall);
    final boolean nonzeroNonce = !bytesToBoolean(nonzeroNonceCall.result());

    // row i + 3
    final OobExoCall creatorNonceAbortCall = callToLT(wcp, creatorNonce, EIP2681_MAX_NONCE_BYTES);
    exoCalls.add(creatorNonceAbortCall);
    final boolean creatorNonceAbort = !bytesToBoolean(creatorNonceAbortCall.result());

    // row i +  4
    exoCalls.add(exceedsMaxInitCodeSize(wcp));

    // Set aborting condition
    setAbortingCondition(insufficientBalanceAbort || callStackDepthAbort || creatorNonceAbort);

    // Set failureCondition
    setFailureCondition(!isAbortingCondition() && (isHasCode() || nonzeroNonce));
  }

  protected abstract OobExoCall exceedsMaxInitCodeSize(Wcp wcp);

  @Override
  public Trace.Oob trace(Trace.Oob trace) {
    traceOobData10column(trace, codeSize);

    return trace
        .isCreate(true)
        .oobInst(OOB_INST_CREATE)
        .data1(value.hi())
        .data2(value.lo())
        .data3(balance)
        .data4(nonce)
        .data5(booleanToBytes(hasCode))
        .data6(callStackDepth)
        .data7(booleanToBytes(abortingCondition))
        .data8(booleanToBytes(failureCondition))
        .data9(creatorNonce);
  }

  protected abstract void traceOobData10column(Trace.Oob trace, long codeSize);

  @Override
  public Trace.Hub trace(Trace.Hub trace) {
    traceHubData10column(trace, codeSize);

    return trace
        .pMiscOobFlag(true)
        .pMiscOobInst(OOB_INST_CREATE)
        .pMiscOobData1(value.hi())
        .pMiscOobData2(value.lo())
        .pMiscOobData3(balance)
        .pMiscOobData4(nonce)
        .pMiscOobData5(booleanToBytes(hasCode))
        .pMiscOobData6(callStackDepth)
        .pMiscOobData7(booleanToBytes(abortingCondition))
        .pMiscOobData8(booleanToBytes(failureCondition))
        .pMiscOobData9(creatorNonce);
  }

  protected abstract void traceHubData10column(Trace.Hub trace, long codeSize);
}

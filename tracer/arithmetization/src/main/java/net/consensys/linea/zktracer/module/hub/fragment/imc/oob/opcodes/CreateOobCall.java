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

package net.consensys.linea.zktracer.module.hub.fragment.imc.oob.opcodes;

import static net.consensys.linea.zktracer.Trace.EIP2681_MAX_NONCE;
import static net.consensys.linea.zktracer.Trace.OOB_INST_CREATE;
import static net.consensys.linea.zktracer.module.hub.fragment.imc.oob.opcodes.CallOobCall.MAX_CALL_STACK_DEPTH_BYTES;
import static net.consensys.linea.zktracer.types.AddressUtils.getDeploymentAddress;
import static net.consensys.linea.zktracer.types.Conversions.*;
import static org.hyperledger.besu.evm.internal.Words.clampedToLong;

import java.math.BigInteger;
import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.Setter;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.OobCall;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.evm.account.Account;
import org.hyperledger.besu.evm.frame.MessageFrame;

@Getter
@Setter
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public final class CreateOobCall extends OobCall {

  private static final Bytes EIP2681_MAX_NONCE_BYTES = bigIntegerToBytes(EIP2681_MAX_NONCE);

  // Inputs
  @EqualsAndHashCode.Include EWord value;
  @EqualsAndHashCode.Include EWord balance;
  @EqualsAndHashCode.Include EWord nonce;
  @EqualsAndHashCode.Include boolean hasCode;
  @EqualsAndHashCode.Include EWord callStackDepth;
  @EqualsAndHashCode.Include EWord creatorNonce;
  @EqualsAndHashCode.Include long codeSize;

  // Outputs
  boolean abortingCondition;
  boolean failureCondition;

  public CreateOobCall() {
    super();
  }

  @Override
  public void setInputs(Hub hub, MessageFrame frame) {
    final OpCodeData opcode = hub.opCodeData();
    final Account creatorAccount = frame.getWorldUpdater().get(frame.getRecipientAddress());
    final Address deploymentAddress = getDeploymentAddress(frame, opcode);
    final Account deployedAccount = frame.getWorldUpdater().get(deploymentAddress);

    final boolean unaborted = hub.pch().abortingConditions().snapshot().none();
    final boolean unabortedCreateAndDeploymentAccountExists =
        ((deployedAccount != null) && unaborted);

    final long nonce = unabortedCreateAndDeploymentAccountExists ? deployedAccount.getNonce() : 0;
    final boolean hasCode = unabortedCreateAndDeploymentAccountExists && deployedAccount.hasCode();

    setValue(EWord.of(frame.getStackItem(0)));
    setBalance(EWord.of(creatorAccount.getBalance().toUnsignedBigInteger()));
    setNonce(EWord.of(nonce));
    setHasCode(hasCode);
    setCallStackDepth(EWord.of(BigInteger.valueOf(frame.getDepth())));
    setCreatorNonce(EWord.of(Bytes.minimalBytes(creatorAccount.getNonce())));
    setCodeSize(clampedToLong(frame.getStackItem(2)));
  }

  @Override
  public void setOutputs() {
    final boolean insufficientBalanceAbort = EWord.of(balance).compareTo(value) < 0;
    final boolean callStackDepthAbort =
        callStackDepth.compareTo(EWord.of(MAX_CALL_STACK_DEPTH_BYTES)) >= 0;
    final boolean nonzeroNonce = !nonce.isZero();
    final boolean creatorNonceAbort =
        creatorNonce.compareTo(EWord.of(EIP2681_MAX_NONCE_BYTES)) >= 0;

    setAbortingCondition(insufficientBalanceAbort || callStackDepthAbort || creatorNonceAbort);
    setFailureCondition(!isAbortingCondition() && (isHasCode() || nonzeroNonce));
  }

  @Override
  public Trace.Oob traceOob(Trace.Oob trace) {
    return trace
        .inst(OOB_INST_CREATE)
        .data1(value.hi())
        .data2(value.lo())
        .data3(balance)
        .data4(nonce)
        .data5(booleanToBytes(hasCode))
        .data6(callStackDepth)
        .data7(booleanToBytes(abortingCondition))
        .data8(booleanToBytes(failureCondition))
        .data9(creatorNonce)
        .data10(Bytes.ofUnsignedLong(codeSize))
        .validateRow();
  }

  @Override
  public Trace.Hub traceHub(Trace.Hub trace) {
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
        .pMiscOobData9(creatorNonce)
        .pMiscOobData10(Bytes.ofUnsignedLong(codeSize));
  }
}

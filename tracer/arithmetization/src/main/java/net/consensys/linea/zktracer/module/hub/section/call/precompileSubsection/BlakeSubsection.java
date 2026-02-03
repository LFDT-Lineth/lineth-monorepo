/*
 * Copyright ConsenSys Inc.
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

package net.consensys.linea.zktracer.module.hub.section.call.precompileSubsection;

import static com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.zktracer.module.blake2fmodexpdata.BlakeModexpDataOperation.BLAKE2f_R_SIZE;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileScenario.PRC_FAILURE_KNOWN_TO_HUB;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileScenario.PRC_FAILURE_KNOWN_TO_RAM;

import net.consensys.linea.zktracer.module.blake2fmodexpdata.BlakeComponents;
import net.consensys.linea.zktracer.module.blake2fmodexpdata.BlakeModexpDataOperation;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.ImcFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.Blake2fCallDataSizeOobCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.Blake2fParamsOobCall;
import net.consensys.linea.zktracer.module.hub.section.call.CallSection;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;
import org.apache.tuweni.bytes.Bytes;

public class BlakeSubsection extends PrecompileSubsection {
  // 3 = SCEN + MISC + CON (squash caller return data): if size != 213
  // 4 = SCEN + MISC + MISC + CON (squash caller return data): if insufficient gas / f not a bit
  // 6 = SCEN + MISC + (3 * MISC) + CON (provide caller with return data): if success
  // we don't optimize as BLAKE calls are rare
  public static final short NB_ROWS_HUB_PRC_BLAKE = 6;
  final Blake2fCallDataSizeOobCall blakeCdsOobCall;
  ImcFragment secondImcFragment;
  Blake2fParamsOobCall blake2fParamsOobCall;
  final boolean blakeSuccess;

  public BlakeSubsection(Hub hub, CallSection callSection) {
    super(hub, callSection);

    blakeCdsOobCall =
        (Blake2fCallDataSizeOobCall) firstImcFragment.callOob(new Blake2fCallDataSizeOobCall());

    if (!blakeCdsOobCall.isHubSuccess()) {
      this.setScenario(PRC_FAILURE_KNOWN_TO_HUB);
      blakeSuccess = false;
      return;
    }

    final Bytes callData = getCallDataRange().extract();
    final Bytes blakeR = callData.slice(0, 4);
    final Bytes blakeF = callData.slice(212, 1);

    final boolean wellFormedF = blakeF.get(0) == 0 || blakeF.get(0) == 1;
    final long rounds = blakeR.toLong();
    final long calleeGas = callSection.stpCall.effectiveChildContextGasAllowance();
    final boolean sufficientGas = calleeGas >= rounds;
    blakeSuccess = wellFormedF && sufficientGas;

    if (!blakeSuccess) {
      this.setScenario(PRC_FAILURE_KNOWN_TO_RAM);
    }

    final MmuCall blakeParameterExtractionMmuCall =
        MmuCall.parameterExtractionForBlake(hub, this, blakeSuccess, blakeR, blakeF);
    firstImcFragment.callMmu(blakeParameterExtractionMmuCall);

    secondImcFragment = ImcFragment.empty(hub);
    fragments.add(secondImcFragment);

    blake2fParamsOobCall =
        (Blake2fParamsOobCall) secondImcFragment.callOob(new Blake2fParamsOobCall(calleeGas));

    checkArgument(blake2fParamsOobCall.isRamSuccess() == blakeSuccess, "BLAKE2f success mismatch");
  }

  @Override
  public void resolveAtContextReEntry(Hub hub, CallFrame callFrame) {
    super.resolveAtContextReEntry(hub, callFrame);

    // sanity checks
    checkArgument(
        blakeCdsOobCall.isHubSuccess() == (callDataSize() == 213), "BLAKE2f hub success mismatch");
    checkArgument(callSuccess == blakeSuccess, "BLAKE2f call success mismatch");
    this.sanityCheck();

    if (!callSuccess) {
      return;
    }

    // finish 2nd MISC row
    final MmuCall callDataExtractionforBlake = MmuCall.callDataExtractionforBlake(hub, this);
    secondImcFragment.callMmu(callDataExtractionforBlake);

    // 3rd MISC row
    final ImcFragment thirdImcFragment = ImcFragment.empty(hub);
    fragments.add(thirdImcFragment);

    final MmuCall returnDataFullTransferForBlake =
        MmuCall.fullReturnDataTransferForBlake(hub, this);
    thirdImcFragment.callMmu(returnDataFullTransferForBlake);

    // 4th MISC row
    final ImcFragment fourthImcFragment = ImcFragment.empty(hub);
    fragments.add(fourthImcFragment);

    if (!this.getReturnAtRange().isEmpty()) {
      final MmuCall partialReturnDataCopyForBlake =
          MmuCall.partialCopyOfReturnDataforBlake(hub, this);
      fourthImcFragment.callMmu(partialReturnDataCopyForBlake);
    }

    final Bytes callData = getCallDataRange().extract();
    final BlakeComponents blake2f =
        new BlakeComponents(
            callData,
            callData.slice(0, BLAKE2f_R_SIZE),
            callData.slice(212, 1),
            extractReturnData());
    hub.blakeModexpData()
        .callBlake(
            new BlakeModexpDataOperation(blake2f, this.exoModuleOperationId()));
  }

  @Override
  protected short maxNumberOfLines() {
    return NB_ROWS_HUB_PRC_BLAKE;
  }
}

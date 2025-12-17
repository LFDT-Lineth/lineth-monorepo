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
import static com.google.common.base.Preconditions.checkState;
import static net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall.extractBbsForModexp;
import static net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall.extractEbsForModexp;
import static net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall.extractMbsForModexp;
import static net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall.extractModexpBase;
import static net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall.extractModexpExponent;
import static net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall.extractModexpModulus;
import static net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall.forModexpFullResultCopy;
import static net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall.forModexpLoadLead;
import static net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall.forModexpPartialResultCopy;
import static net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.modexp.ModexpXbsCase.MODEXP_XBS_CASE_BBS;
import static net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.modexp.ModexpXbsCase.MODEXP_XBS_CASE_EBS;
import static net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.modexp.ModexpXbsCase.MODEXP_XBS_CASE_MBS;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileScenario.PRC_FAILURE_KNOWN_TO_RAM;

import net.consensys.linea.zktracer.module.blake2fmodexpdata.BlakeModexpDataOperation;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.ImcFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.exp.ExpCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.exp.ModexpLogExpCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.modexp.*;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.modexp.ModexpPricingOobCall;
import net.consensys.linea.zktracer.module.hub.precompiles.ModexpMetadata;
import net.consensys.linea.zktracer.module.hub.section.call.CallSection;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;

public class ModexpSubsection extends PrecompileSubsection {

  // 13 = 1 + 12 (scenario row + up to 12 miscellaneous fragments)
  public static final short NB_ROWS_HUB_PRC_MODEXP = 13;

  public final ModexpMetadata modexpMetadata;
  private ModexpPricingOobCall sixthOobCall;
  private ImcFragment seventhImcFragment;
  public boolean transactionWillBePopped = false;

  public ModexpSubsection(
      final Hub hub, final CallSection callSection, ModexpMetadata modexpMetadata) {
    super(hub, callSection);

    this.modexpMetadata = modexpMetadata;

    /// call data size analysis
    firstImcFragment.callOob(new ModexpCallDataSizeOobCall(modexpMetadata));

    /// bbs extraction and analysis
    final ImcFragment secondImcFragment = ImcFragment.empty(hub);
    fragments().add(secondImcFragment);
    if (modexpMetadata.extractBbs()) {
      final MmuCall mmuCall = extractBbsForModexp(hub, this, modexpMetadata);
      secondImcFragment.callMmu(mmuCall);
    }
    secondImcFragment.callOob(new ModexpXbsOobCall(modexpMetadata, MODEXP_XBS_CASE_BBS));

    /// ebs extraction and analysis
    final ImcFragment thirdImcFragment = ImcFragment.empty(hub);
    fragments().add(thirdImcFragment);
    if (modexpMetadata.extractEbs()) {
      final MmuCall mmuCall = extractEbsForModexp(hub, this, modexpMetadata);
      thirdImcFragment.callMmu(mmuCall);
    }
    thirdImcFragment.callOob(new ModexpXbsOobCall(modexpMetadata, MODEXP_XBS_CASE_EBS));

    /// mbs extraction and analysis
    final ImcFragment fourthImcFragment = ImcFragment.empty(hub);
    fragments().add(fourthImcFragment);
    if (modexpMetadata.extractMbs()) {
      final MmuCall mmuCall = extractMbsForModexp(hub, this, modexpMetadata);
      fourthImcFragment.callMmu(mmuCall);
    }
    fourthImcFragment.callOob(new ModexpXbsOobCall(modexpMetadata, MODEXP_XBS_CASE_MBS));

    /// we add the last two IMC fragments of the common core of MODEXP
    final ImcFragment fifthImcFragment = ImcFragment.empty(hub);
    fragments().add(fifthImcFragment);
    final ImcFragment sixthImcFragment = ImcFragment.empty(hub);
    fragments().add(sixthImcFragment);

    /// the last two IMC fragments of the common core
    /// can remain empty in Osaka if some xbs are out of bounds:
    if (!allXbsesAreInBounds()) return;

    /// leading exponent word extraction,
    /// analysis and exponent log computation

    fifthImcFragment.callOob(new ModexpLeadOobCall(modexpMetadata));
    if (modexpMetadata.loadRawLeadingWord()) {
      final MmuCall mmuCall = forModexpLoadLead(hub, this, modexpMetadata);
      fifthImcFragment.callMmu(mmuCall);
      final ExpCall modexpLogCallToExp = new ModexpLogExpCall(modexpMetadata);
      fifthImcFragment.callExp(modexpLogCallToExp);
    }

    /// MODEXP pricing row

    // Note: we must compute the callee gas here as the eponymous PrecompileSubsection field gets
    // computed at traceContextEnter() which happens after the constructor invocation.
    final long calleeGas = callSection.stpCall.effectiveChildContextGasAllowance();
    sixthOobCall =
        (ModexpPricingOobCall)
            sixthImcFragment.callOob(new ModexpPricingOobCall(modexpMetadata, calleeGas));

    // We need to trigger the OOB before CALL's execution
    if (allXbsesAreInBounds() && sixthOobCall.isRamSuccess()) {
      seventhImcFragment = ImcFragment.empty(hub);
      seventhImcFragment.callOob(new ModexpExtractOobCall(modexpMetadata));
    }
  }

  @Override
  public void resolveAtContextReEntry(Hub hub, CallFrame callFrame) {
    super.resolveAtContextReEntry(hub, callFrame);

    // sanity check
    checkArgument(
        callSuccess == (allXbsesAreInBounds() && sixthOobCall.isRamSuccess()),
        "Inconsistent Modexp success status");

    if (!callSuccess) {
      precompileScenarioFragment.scenario(PRC_FAILURE_KNOWN_TO_RAM);
      return;
    }

    checkState(allXbsesAreInBounds(), "MODEXP' callSucess requires that all XBS' be in bounds");

    modexpMetadata.rawResult(extractReturnData());
    hub.blakeModexpData()
        .callModexp(new BlakeModexpDataOperation(modexpMetadata, exoModuleOperationId()));

    fragments().add(seventhImcFragment);
    if (modexpMetadata.extractModulus()) {
      final MmuCall mmuCall = extractModexpBase(hub, this, modexpMetadata);
      seventhImcFragment.callMmu(mmuCall);
    }

    final ImcFragment eighthImcFragment = ImcFragment.empty(hub);
    fragments().add(eighthImcFragment);
    if (modexpMetadata.extractModulus()) {
      final MmuCall mmuCall = extractModexpExponent(hub, this, modexpMetadata);
      eighthImcFragment.callMmu(mmuCall);
    }

    final ImcFragment ninthImcFragment = ImcFragment.empty(hub);
    fragments().add(ninthImcFragment);
    if (modexpMetadata.extractModulus()) {
      final MmuCall mmuCall = extractModexpModulus(hub, this, modexpMetadata);
      ninthImcFragment.callMmu(mmuCall);
    }

    final ImcFragment tenthImcFragment = ImcFragment.empty(hub);
    fragments().add(tenthImcFragment);
    if (modexpMetadata.extractModulus()) {
      final MmuCall mmuCall = forModexpFullResultCopy(hub, this, modexpMetadata);
      tenthImcFragment.callMmu(mmuCall);
    }

    final ImcFragment eleventhImcFragment = ImcFragment.empty(hub);
    fragments().add(eleventhImcFragment);
    if (modexpMetadata.mbsNonZero() && !getReturnAtRange().isEmpty()) {
      final MmuCall mmuCall = forModexpPartialResultCopy(hub, this, modexpMetadata);
      eleventhImcFragment.callMmu(mmuCall);
    }
  }

  protected boolean allXbsesAreInBounds() {
    return modexpMetadata.allXbsesAreInBounds();
  }
}

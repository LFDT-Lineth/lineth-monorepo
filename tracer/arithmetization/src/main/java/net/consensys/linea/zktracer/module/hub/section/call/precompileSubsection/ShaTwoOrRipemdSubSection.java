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

import static com.google.common.base.Preconditions.*;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_SHA2_256;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileScenario.PRC_FAILURE_KNOWN_TO_HUB;
import static net.consensys.linea.zktracer.module.shakiradata.HashFunction.RIPEMD;
import static net.consensys.linea.zktracer.module.shakiradata.HashFunction.SHA256;

import java.math.BigInteger;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.ImcFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.OobCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.CommonPrecompileOobCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.shaRipId.RipOobCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.shaRipId.Sha2OobCall;
import net.consensys.linea.zktracer.module.hub.section.call.CallSection;
import net.consensys.linea.zktracer.module.shakiradata.ShakiraDataOperation;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;

public class ShaTwoOrRipemdSubSection extends PrecompileSubsection {
  public static final short NB_ROWS_HUB_PRC_SHARIP = 4;

  final CommonPrecompileOobCall oobCall;

  public ShaTwoOrRipemdSubSection(Hub hub, CallSection callSection) {
    super(hub, callSection);

    final long calleeGas = callSection.stpCall.effectiveChildContextGasAllowance();
    final OobCall call =
        switch (flag()) {
          case PRC_SHA2_256 -> new Sha2OobCall(BigInteger.valueOf(calleeGas));
          case PRC_RIPEMD_160 -> new RipOobCall(BigInteger.valueOf(calleeGas));
          default ->
              throw new IllegalArgumentException(
                  String.format(
                      "Precompile address %s not supported by constructor",
                      this.flag().toString()));
        };
    oobCall = (CommonPrecompileOobCall) firstImcFragment.callOob(call);

    if (!oobCall.isHubSuccess()) {
      this.setScenario(PRC_FAILURE_KNOWN_TO_HUB);
    }
  }

  @Override
  public void resolveAtContextReEntry(Hub hub, CallFrame callFrame) {
    super.resolveAtContextReEntry(hub, callFrame);

    // sanity check
    checkArgument(
        callSuccess == oobCall.isHubSuccess(),
        "oob and hub disagree on precompile %s success",
        flag());

    if (!callSuccess) {
      return;
    }

    // NOTE: we trigger the SHAKIRA module for nonempty call data only
    if (!getCallDataRange().isEmpty()) {
      final ShakiraDataOperation shakiraCall =
          new ShakiraDataOperation(
              callSection.hubStamp(),
              this.flag() == PRC_SHA2_256 ? SHA256 : RIPEMD,
              extractCallData(),
              callFrame.frame().getReturnData());
      hub.shakiraData().call(shakiraCall);

      final MmuCall mmuCall = MmuCall.callDataExtractionForShaTwoAndRipemd(hub, this);
      firstImcFragment.callMmu(mmuCall);
    }

    // the full result transfer happens in all cases
    final ImcFragment secondImcFragment = ImcFragment.empty(hub);
    this.fragments().add(secondImcFragment);

    final MmuCall fullOutputDataTransfer = MmuCall.fullResultTransferForShaTwoAndRipemd(hub, this);
    secondImcFragment.callMmu(fullOutputDataTransfer);

    final ImcFragment thirdImcFragment = ImcFragment.empty(hub);
    this.fragments().add(thirdImcFragment);

    // the partial copy of return data happens only if the caller context
    // provided a nonempty return data target
    if (!getReturnAtRange().isEmpty()) {
      final MmuCall partialReturnDataCopy =
          MmuCall.partialCopyOfReturnDataForShaTwoAndRipemd(hub, this);
      thirdImcFragment.callMmu(partialReturnDataCopy);
    }
  }

  // 4 = 1 + 3 (scenario row + up to 3 miscellaneous fragments)
  @Override
  protected short maxNumberOfLines() {
    return NB_ROWS_HUB_PRC_SHARIP;
    // Note: we don't have the callSuccess available at the moment
    // and can't provide the "real" value (2 in case of FKTH.)
  }
}

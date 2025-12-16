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

package net.consensys.linea.zktracer.module.ecdata;

import static net.consensys.linea.zktracer.module.ModuleName.EC_DATA;

import java.util.List;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.CountingOnlyModule;
import net.consensys.linea.zktracer.container.module.IncrementingModule;
import net.consensys.linea.zktracer.container.module.OperationListModule;
import net.consensys.linea.zktracer.container.stacked.ModuleOperationStackedList;
import net.consensys.linea.zktracer.module.ModuleName;
import net.consensys.linea.zktracer.module.ext.Ext;
import net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import org.apache.tuweni.bytes.Bytes;

@RequiredArgsConstructor
@Getter
@Accessors(fluent = true)
public class EcData implements OperationListModule<EcDataOperation> {
  private final ModuleOperationStackedList<EcDataOperation> operations =
      new ModuleOperationStackedList<>();

  private final Wcp wcp;
  private final Ext ext;

  private final IncrementingModule ecAddEffectiveCall;
  private final IncrementingModule ecMulEffectiveCall;
  private final IncrementingModule ecRecoverEffectiveCall;

  private final CountingOnlyModule ecPairingG2MembershipCalls;
  private final CountingOnlyModule ecPairingMillerLoops;
  private final IncrementingModule ecPairingFinalExponentiations;

  private final IncrementingModule p256VerifyEffectiveCalls;

  @Getter private EcDataOperation ecDataOperation;

  @Override
  public ModuleName moduleKey() {
    return EC_DATA;
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.ecdata().headers(this.lineCount());
  }

  @Override
  public int spillage(Trace trace) {
    return trace.ecdata().spillage();
  }

  @Override
  public void commit(Trace trace) {
    int stamp = 0;
    long previousId = 0;
    for (EcDataOperation op : operations.getAll()) {
      op.trace(trace.ecdata(), ++stamp, previousId);
      previousId = op.id();
    }
  }

  public void callEcData(
      final int id,
      final PrecompileScenarioFragment.PrecompileFlag precompileFlag,
      final Bytes callData,
      final Bytes returnData) {
    ecDataOperation = EcDataOperation.of(wcp, ext, id, precompileFlag, callData, returnData);
    operations.add(ecDataOperation);

    switch (ecDataOperation.precompileFlag()) {
      case PRC_ECADD -> ecAddEffectiveCall.updateTally(ecDataOperation.internalChecksPassed());
      case PRC_ECMUL -> ecMulEffectiveCall.updateTally(ecDataOperation.internalChecksPassed());
      case PRC_ECRECOVER ->
          ecRecoverEffectiveCall.updateTally(ecDataOperation.internalChecksPassed());
      case PRC_ECPAIRING -> {
        // ecPairingG2MembershipCalls case
        // NOTE: the other precompile limits are managed below
        // NOTE: see EC_DATA specs Figure 3.5 for a graphical representation of this case analysis

        //  if (!ecDataOperation.internalChecksPassed()) {
        //    // The circuit is never invoked in the case of internal checks failing
        //    ecPairingG2MembershipCalls.updateTally(0);
        //  }

        // NOTE: the && of the conditions may seem not necessary since in the specs
        // !internalChecksPassed => !notOnG2AccMax
        // however, in EcDataOperation implementation the notOnG2AccMax takes into consideration
        // only large points G2 membership
        // , and it has to be && with internalChecksPassed to compute the actual
        // NOT_ON_G2_ACC_MAX to trace
        if (ecDataOperation.internalChecksPassed() && ecDataOperation.notOnG2AccMax()) {
          ecPairingG2MembershipCalls.updateTally(1);
          // The circuit is invoked only once if there is at least one point predicted to be not on
          // G2
        }
        //   if (ecDataOperation.internalChecksPassed()
        //       && !ecDataOperation.notOnG2AccMax()
        //       && ecDataOperation.isOverallTrivialPairing()) {
        //     // The circuit is never invoked in the case of a trivial pairing
        //     // ecPairingG2MembershipCalls.updateTally(0);
        //   }

        if (ecDataOperation.internalChecksPassed()
            && !ecDataOperation.notOnG2AccMax()
            && !ecDataOperation.isOverallTrivialPairing()) {
          ecPairingG2MembershipCalls.updateTally(
              ecDataOperation.circuitSelectorG2MembershipCounter());
          // The circuit is invoked as many times as there are points predicted to be on G2
        }

        // NOTE: a similar case analysis to the one above may be done for the other
        // precompile limits. However, circuitSelectorEcPairingCounter already takes
        // it into consideration and what follows is enough

        // ecPairingMillerLoops case
        // NOTE: the pairings that require Miller Loops are the valid ones where
        // the small point is on C_1, the large point is on G_2, and they are not
        // points at infinity (valid trivial pairings and valid pairings with the
        // small point at infinity are excluded from this counting)
        ecPairingMillerLoops.updateTally(ecDataOperation.circuitSelectorEcPairingCounter());

        // ecPairingFinalExponentiation case
        // NOTE: if at least one Miller Loop is computed, the final exponentiation is 1
        ecPairingFinalExponentiations.updateTally(
            ecDataOperation.circuitSelectorEcPairingCounter()
                > 0); // See https://eprint.iacr.org/2008/490.pdf
      }
      case PRC_P256_VERIFY -> {
        p256VerifyEffectiveCalls.updateTally(ecDataOperation.internalChecksPassed());
      }
      default -> throw new IllegalArgumentException("Operation not supported by EcData");
    }
  }
}

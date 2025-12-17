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

package net.consensys.linea.zktracer.module.mmu;

import static com.google.common.base.Preconditions.checkState;
import static net.consensys.linea.zktracer.module.ModuleName.MMU;

import java.util.List;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.OperationListModule;
import net.consensys.linea.zktracer.container.stacked.ModuleOperationStackedList;
import net.consensys.linea.zktracer.module.ModuleName;
import net.consensys.linea.zktracer.module.euc.Euc;
import net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall;
import net.consensys.linea.zktracer.module.mmu.values.HubToMmuValues;
import net.consensys.linea.zktracer.module.wcp.Wcp;

@RequiredArgsConstructor
@Accessors(fluent = true)
public class Mmu implements OperationListModule<MmuOperation> {
  @Getter
  private final ModuleOperationStackedList<MmuOperation> operations =
      new ModuleOperationStackedList<>();

  private final Euc euc;
  private final Wcp wcp;

  @Override
  public ModuleName moduleKey() {
    return MMU;
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.mmu().headers(this.lineCount());
  }

  @Override
  public int spillage(Trace trace) {
    return trace.mmu().spillage();
  }

  @Override
  public void commit(Trace trace) {
    int mmuStamp = 0;
    int mmioStamp = 0;

    for (MmuOperation mmuOperation : operations.getAll()) {
      checkState(mmuOperation.traceMe(), "Cannot commit MMU operation with false 'traceMe'");
      mmuOperation.getCFI();
      mmuOperation.fillLimb();

      mmioStamp = mmuOperation.trace(++mmuStamp, mmioStamp, trace.mmu());
    }
  }

  public void call(final MmuCall mmuCall) {
    checkState(mmuCall.traceMe(), "Shouldn't compute if traceMe is false");
    MmuData mmuData = new MmuData(mmuCall);
    mmuData.hubToMmuValues(
        HubToMmuValues.fromMmuCall(mmuCall, mmuData.exoLimbIsSource(), mmuData.exoLimbIsTarget()));

    final MmuInstructions mmuInstructions = new MmuInstructions(euc, wcp);
    mmuData = mmuInstructions.compute(mmuData);

    operations.add(new MmuOperation(mmuData));
  }
}

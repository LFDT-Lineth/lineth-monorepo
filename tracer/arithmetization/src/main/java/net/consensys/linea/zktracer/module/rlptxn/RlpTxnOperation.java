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

package net.consensys.linea.zktracer.module.rlptxn;

import static net.consensys.linea.zktracer.module.rlptxn.phaseSection.IntegerEntry.*;
import static org.hyperledger.besu.datatypes.TransactionType.EIP1559;
import static org.hyperledger.besu.datatypes.TransactionType.FRONTIER;

import java.util.ArrayList;
import java.util.List;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.ModuleOperation;
import net.consensys.linea.zktracer.module.rlpUtils.RlpUtils;
import net.consensys.linea.zktracer.module.rlptxn.phaseSection.*;
import net.consensys.linea.zktracer.module.trm.Trm;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;

public class RlpTxnOperation extends ModuleOperation {

  private final List<PhaseSection> phaseSectionList = new ArrayList<>();
  private final GenericTracedValue tracedValues;

  public RlpTxnOperation(RlpUtils rlpUtils, Trm trm, TransactionProcessingMetadata tx) {
    tracedValues = new GenericTracedValue(tx);

    // Phase RLP Prefix
    phaseSectionList.add(new GlobalPrefixPhaseSection(rlpUtils, tracedValues));

    // Phase Chain ID
    if (tx.getBesuTransaction().getType() != FRONTIER) {
      phaseSectionList.add(new IntegerPhaseSection(rlpUtils, CHAIN_ID, tx));
    }

    // Phase Nonce
    phaseSectionList.add(new IntegerPhaseSection(rlpUtils, NONCE, tx));

    // Phase Gas Price
    if (tx.getBesuTransaction().getType() != EIP1559) {
      phaseSectionList.add(new IntegerPhaseSection(rlpUtils, GAS_PRICE, tx));
    }

    // Phase Max Priority Fee Per Gas
    if (tx.getBesuTransaction().getType() == EIP1559) {
      phaseSectionList.add(new IntegerPhaseSection(rlpUtils, MAX_PRIORITY_FEE_PER_GAS, tx));
    }

    // Phase Max Fee Per Gas
    if (tx.getBesuTransaction().getType() == EIP1559) {
      phaseSectionList.add(new IntegerPhaseSection(rlpUtils, MAX_FEE_PER_GAS, tx));
    }

    // Phase Gas Limit
    phaseSectionList.add(new IntegerPhaseSection(rlpUtils, GAS_LIMIT, tx));

    // Phase To
    phaseSectionList.add(new ToPhaseSection(tx));

    // Phase Value
    phaseSectionList.add(new IntegerPhaseSection(rlpUtils, VALUE, tx));

    // Phase Data
    phaseSectionList.add(new DataPhaseSection(rlpUtils, tx));

    // Phase Access List
    if (tx.getBesuTransaction().getType() != FRONTIER) {
      phaseSectionList.add(new AccessListPhaseSection(rlpUtils, trm, tx));
    }

    // Phase Beta
    if (tx.getBesuTransaction().getType() == FRONTIER) {
      phaseSectionList.add(new BetaPhaseSection(rlpUtils, tx));
    }

    // Phase Y
    if (tx.getBesuTransaction().getType() != FRONTIER) {
      phaseSectionList.add(new IntegerPhaseSection(rlpUtils, Y, tx));
    }

    // Phase R
    phaseSectionList.add(new IntegerPhaseSection(rlpUtils, R, tx));

    // Phase S
    phaseSectionList.add(new IntegerPhaseSection(rlpUtils, S, tx));
  }

  protected void trace(Trace.Rlptxn trace, int userTransactionNumberMax) {
    tracedValues.userTxnNumberMax(userTransactionNumberMax);
    for (PhaseSection section : phaseSectionList) {
      section.trace(trace, tracedValues);
    }
  }

  @Override
  protected int computeLineCount() {
    return phaseSectionList.stream().mapToInt(PhaseSection::lineCount).sum();
  }
}

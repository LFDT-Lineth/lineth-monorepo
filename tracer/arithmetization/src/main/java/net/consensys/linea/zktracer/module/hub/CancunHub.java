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

package net.consensys.linea.zktracer.module.hub;

import static net.consensys.linea.zktracer.module.hub.HubProcessingPhase.TX_SKIP;
import static net.consensys.linea.zktracer.module.hub.TransactionProcessingType.SYSF;
import static net.consensys.linea.zktracer.module.hub.TransactionProcessingType.SYSI;

import java.util.Map;
import net.consensys.linea.zktracer.ChainConfig;
import net.consensys.linea.zktracer.module.blockdata.module.BlockData;
import net.consensys.linea.zktracer.module.blockdata.module.CancunBlockData;
import net.consensys.linea.zktracer.module.blsdata.BlsData;
import net.consensys.linea.zktracer.module.euc.Euc;
import net.consensys.linea.zktracer.module.hub.section.McopySection;
import net.consensys.linea.zktracer.module.hub.section.finalization.CancunFinalizationSection;
import net.consensys.linea.zktracer.module.hub.section.halt.selfdestruct.CancunSelfdestructSection;
import net.consensys.linea.zktracer.module.hub.section.skip.CancunTxSkipSection;
import net.consensys.linea.zktracer.module.hub.section.systemTransaction.EIP4788BeaconBlockRootSection;
import net.consensys.linea.zktracer.module.hub.section.systemTransaction.SysfNoopSection;
import net.consensys.linea.zktracer.module.hub.section.transients.TLoadSection;
import net.consensys.linea.zktracer.module.hub.section.transients.TStoreSection;
import net.consensys.linea.zktracer.module.hub.section.txInitializationSection.CancunInitializationSection;
import net.consensys.linea.zktracer.module.hub.transients.Transients;
import net.consensys.linea.zktracer.module.mxp.module.CancunMxp;
import net.consensys.linea.zktracer.module.mxp.module.Mxp;
import net.consensys.linea.zktracer.module.rlpUtils.RlpUtils;
import net.consensys.linea.zktracer.module.rlptxn.RlpTxn;
import net.consensys.linea.zktracer.module.rlptxn.cancun.CancunRlpTxn;
import net.consensys.linea.zktracer.module.tables.instructionDecoder.CancunInstructionDecoder;
import net.consensys.linea.zktracer.module.tables.instructionDecoder.InstructionDecoder;
import net.consensys.linea.zktracer.module.txndata.TxnData;
import net.consensys.linea.zktracer.module.txndata.cancun.CancunTxnData;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import net.consensys.linea.zktracer.types.PublicInputs;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.worldstate.WorldView;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;

public class CancunHub extends ShanghaiHub {
  public CancunHub(ChainConfig chain, PublicInputs publicInputs) {
    super(chain, publicInputs);
  }

  @Override
  protected BlsData setBlsData(Hub hub) {
    return new BlsData(
        hub.wcp(),
        pointEvaluationEffectiveCall,
        pointEvaluationFailureCall,
        blsG1AddEffectiveCall,
        blsG1MsmEffectiveCall,
        blsG2AddEffectiveCall,
        blsG2MsmEffectiveCall,
        blsPairingCheckMillerLoops,
        blsPairingCheckFinalExponentiations,
        blsG1MapFpToG1EffectiveCall,
        blsG1MapFp2ToG2EffectiveCall,
        blsC1MembershipCalls,
        blsC2MembershipCalls,
        blsG1MembershipCalls,
        blsG2MembershipCalls);
  }

  @Override
  protected Mxp setMxp() {
    return new CancunMxp();
  }

  @Override
  protected TxnData setTxnData() {
    return new CancunTxnData(this, wcp(), euc());
  }

  @Override
  protected BlockData setBlockData(
      Hub hub, Wcp wcp, Euc euc, ChainConfig chain, Map<Long, Bytes> blobBaseFees) {
    return new CancunBlockData(hub, wcp, euc, chain, blobBaseFees);
  }

  @Override
  protected RlpTxn setRlpTxn(Hub hub) {
    return new CancunRlpTxn((RlpUtils) hub.rlpUtils(), hub.trm());
  }

  @Override
  protected RlpUtils setRlpUtils() {
    return new RlpUtils();
  }

  @Override
  protected InstructionDecoder setInstructionDecoder() {
    return new CancunInstructionDecoder(this.opCodes());
  }

  @Override
  protected void setSkipSection(
      Hub hub,
      WorldView world,
      TransactionProcessingMetadata transactionProcessingMetadata,
      Transients transients) {
    new CancunTxSkipSection(hub, world, transactionProcessingMetadata, transients);
  }

  @Override
  protected void setInitializationSection(WorldView world) {
    new CancunInitializationSection(this, world);
  }

  @Override
  protected void setFinalizationSection(Hub hub) {
    new CancunFinalizationSection(hub);
  }

  @Override
  protected void setTransientSection(final Hub hub) {
    switch (hub.opCode()) {
      case TLOAD -> new TLoadSection(hub);
      case TSTORE -> new TStoreSection(hub);
      default ->
          throw new IllegalStateException("invalid operation in family TRANSIENT: " + hub.opCode());
    }
  }

  @Override
  protected void setMcopySection(Hub hub) {
    new McopySection(hub);
  }

  @Override
  protected void traceSysiTransactions(WorldView world, ProcessableBlockHeader blockHeader) {
    state.transactionProcessingType(SYSI);
    state.incrementSysiTransactionNumber();
    state.processingPhase(TX_SKIP);
    new EIP4788BeaconBlockRootSection(this, world, blockHeader);
  }

  @Override
  protected void traceSystemFinalTransaction() {
    state.transactionProcessingType(SYSF);
    // TODO: the two following should be done at the beginning of the none section, but requires
    // java > 21
    state.incrementSysfTransactionNumber();
    state.processingPhase(TX_SKIP);
    new SysfNoopSection(this);
  }

  @Override
  protected void setSelfdestructSection(final Hub hub, final MessageFrame frame) {
    new CancunSelfdestructSection(hub, frame);
  }
}

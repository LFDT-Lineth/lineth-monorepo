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

package net.consensys.linea.zktracer.module.rlptxn.phaseSection;

import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.Trace.RLP_TXN_NB_ROWS_PER_AUTHORIZATION;
import static net.consensys.linea.zktracer.module.rlptxn.phaseSection.ToPhaseSection.*;
import static net.consensys.linea.zktracer.types.AddressUtils.highPart;
import static net.consensys.linea.zktracer.types.AddressUtils.lowPart;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes32;

import java.util.ArrayList;
import java.util.List;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.rlpUtils.InstructionByteStringPrefix;
import net.consensys.linea.zktracer.module.rlpUtils.InstructionInteger;
import net.consensys.linea.zktracer.module.rlpUtils.RlpUtils;
import net.consensys.linea.zktracer.module.rlptxn.GenericTracedValue;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.CodeDelegation;

public class AuthorizationListSection extends PhaseSection {
  private final InstructionByteStringPrefix authorizationListRlpPrefix;
  private final List<AuthorizationListEntrySubSection> authorizationList;

  public AuthorizationListSection(RlpUtils rlpUtils, TransactionProcessingMetadata tx) {
    final List<CodeDelegation> authorizations =
        tx.getBesuTransaction().getCodeDelegationList().orElseThrow();
    final int nbAuthorizations = authorizations.size();
    authorizationList = new ArrayList<>(nbAuthorizations);
    for (int index = 1; index <= nbAuthorizations; index++) {
      authorizationList.add(
          new AuthorizationListEntrySubSection(rlpUtils, authorizations.get(index - 1), index));
    }

    final int innerAuthorizationListRlpSize =
        authorizationList.stream()
            .mapToInt(
                auth ->
                    auth.entryRlpPrefix.rlpPrefixByteSize()
                        + auth.entryRlpPrefix.byteStringLength())
            .sum();
    authorizationListRlpPrefix =
        (InstructionByteStringPrefix)
            rlpUtils.call(
                new InstructionByteStringPrefix(innerAuthorizationListRlpSize, (byte) 0, true));
  }

  @Override
  protected void traceComputationsRows(
      Trace.Rlptxn trace, TransactionProcessingMetadata tx, GenericTracedValue tracedValues) {

    tracedValues.setListRlpSize(authorizationListRlpPrefix.byteStringLength());

    // Phase RlpPrefix
    traceTransactionConstantValues(trace, tracedValues);
    authorizationListRlpPrefix.traceRlpTxn(trace, tracedValues, true, true, true, 0);
    trace.pCmpAux1(tracedValues.listRlpSize());
    tracePostValues(trace, tracedValues);

    // Trace each Authorization
    for (int index = 1; index <= authorizationList.size(); index++) {
      authorizationList.get(index - 1).trace(trace, tracedValues);
    }
  }

  @Override
  protected void traceIsPhaseX(Trace.Rlptxn trace) {
    trace.isAuthorizationList(true);
  }

  @Override
  public int lineCount() {
    return 1 + 1 + authorizationList.size() * RLP_TXN_NB_ROWS_PER_AUTHORIZATION;
  }

  private class AuthorizationListEntrySubSection {
    private final InstructionByteStringPrefix entryRlpPrefix;
    private final InstructionInteger chainId;
    private final Address address;
    private final InstructionInteger nonce;
    private final InstructionInteger y;
    private final InstructionInteger r;
    private final InstructionInteger s;
    private final int index;

    private AuthorizationListEntrySubSection(
        RlpUtils rlpUtils, CodeDelegation authorization, int index) {
      this.index = index;
      chainId =
          (InstructionInteger)
              rlpUtils.call(new InstructionInteger(bigIntegerToBytes32(authorization.chainId())));
      address = authorization.address();
      nonce =
          (InstructionInteger)
              rlpUtils.call(
                  new InstructionInteger(
                      Bytes32.leftPad(Bytes.ofUnsignedLong(authorization.nonce()))));
      y =
          (InstructionInteger)
              rlpUtils.call(new InstructionInteger(Bytes32.leftPad(Bytes.of(authorization.v()))));
      r =
          (InstructionInteger)
              rlpUtils.call(new InstructionInteger(bigIntegerToBytes32(authorization.r())));
      s =
          (InstructionInteger)
              rlpUtils.call(new InstructionInteger(bigIntegerToBytes32(authorization.s())));
      final int innerRlpSize =
          chainId.rlpSize()
              + (1 + Address.SIZE)
              + nonce.rlpSize()
              + y.rlpSize()
              + r.rlpSize()
              + s.rlpSize();
      entryRlpPrefix =
          (InstructionByteStringPrefix)
              rlpUtils.call(new InstructionByteStringPrefix(innerRlpSize, (byte) 0, true));
    }

    public void trace(Trace.Rlptxn trace, GenericTracedValue tracedValues) {
      tracedValues.setItemRlpSize(entryRlpPrefix.byteStringLength());

      // authorization rlp prefix
      traceTransactionConstantValues(trace, tracedValues);
      entryRlpPrefix.traceRlpTxn(trace, tracedValues, true, true, true, 0);
      tracedValues.decrementListRlpSizeBy(entryRlpPrefix.rlpPrefixByteSize());
      traceAuthorizationSpecificColumns(trace, tracedValues, (short) 0);
      tracePostValues(trace, tracedValues);

      traceIntInstruction(chainId, trace, tracedValues);

      // address
      // first Row: Address prefix
      traceTransactionConstantValues(trace, tracedValues);
      traceAddressPrefix(trace, address, tracedValues);
      traceAuthorizationSpecificColumns(trace, tracedValues, (short) 1);
      tracePostValues(trace, tracedValues);

      // second Row: Address Hi
      traceTransactionConstantValues(trace, tracedValues);
      traceAddressHi(trace, address, tracedValues);
      traceAuthorizationSpecificColumns(trace, tracedValues, (short) 4);
      tracePostValues(trace, tracedValues);

      // second Row: Address Lo
      traceTransactionConstantValues(trace, tracedValues);
      traceAddressLo(trace, address, tracedValues);
      traceAuthorizationSpecificColumns(trace, tracedValues, (short) LLARGE);
      tracePostValues(trace, tracedValues);

      traceIntInstruction(nonce, trace, tracedValues);
      traceIntInstruction(y, trace, tracedValues);
      traceIntInstruction(r, trace, tracedValues);
      traceIntInstruction(s, trace, tracedValues);
    }

    private void traceIntInstruction(
        InstructionInteger intInstruction, Trace.Rlptxn trace, GenericTracedValue tracedValues) {
      for (int ct = 0; ct <= 2; ct++) {
        traceTransactionConstantValues(trace, tracedValues);
        intInstruction.traceRlpTxn(trace, tracedValues, true, true, true, ct);
        traceAuthorizationSpecificColumns(trace, tracedValues, intInstruction.nBytes(ct));
        tracePostValues(trace, tracedValues);
      }
    }

    private void traceAuthorizationSpecificColumns(
        Trace.Rlptxn trace, GenericTracedValue tracedValues, short nBytes) {
      tracedValues.decrementAllCountersBy(nBytes);
      trace
          .pCmpAux1(tracedValues.listRlpSize())
          .pCmpAux2(tracedValues.itemRlpSize())
          .pCmpAuxCcc1(index)
          .pCmpAux8(chainId.integer().slice(0, LLARGE))
          .pCmpAux3(chainId.integer().slice(LLARGE, LLARGE))
          .pCmpAuxCcc4(highPart(address))
          .pCmpAuxCcc5(lowPart(address))
          .pCmpAuxCcc2(nonce.integer())
          .pCmpAuxCcc3(y.integer().toLong())
          .pCmpAux4(r.integer().slice(0, LLARGE))
          .pCmpAux5(r.integer().slice(LLARGE, LLARGE))
          .pCmpAux6(s.integer().slice(0, LLARGE))
          .pCmpAux7(s.integer().slice(LLARGE, LLARGE));
    }
  }
}

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

package net.consensys.linea.zktracer.module.rlptxn.london;

import static com.google.common.base.Preconditions.*;
import static net.consensys.linea.zktracer.Trace.GAS_CONST_G_TX_DATA_NONZERO;
import static net.consensys.linea.zktracer.Trace.GAS_CONST_G_TX_DATA_ZERO;
import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.Trace.RLP_PREFIX_INT_LONG;
import static net.consensys.linea.zktracer.Trace.RLP_PREFIX_INT_SHORT;
import static net.consensys.linea.zktracer.Trace.RLP_PREFIX_LIST_LONG;
import static net.consensys.linea.zktracer.Trace.RLP_PREFIX_LIST_SHORT;
import static net.consensys.linea.zktracer.Trace.RLP_TXN_PHASE_ACCESS_LIST;
import static net.consensys.linea.zktracer.Trace.RLP_TXN_PHASE_BETA;
import static net.consensys.linea.zktracer.Trace.RLP_TXN_PHASE_CHAIN_ID;
import static net.consensys.linea.zktracer.Trace.RLP_TXN_PHASE_DATA;
import static net.consensys.linea.zktracer.Trace.RLP_TXN_PHASE_GAS_LIMIT;
import static net.consensys.linea.zktracer.Trace.RLP_TXN_PHASE_GAS_PRICE;
import static net.consensys.linea.zktracer.Trace.RLP_TXN_PHASE_MAX_FEE_PER_GAS;
import static net.consensys.linea.zktracer.Trace.RLP_TXN_PHASE_MAX_PRIORITY_FEE_PER_GAS;
import static net.consensys.linea.zktracer.Trace.RLP_TXN_PHASE_NONCE;
import static net.consensys.linea.zktracer.Trace.RLP_TXN_PHASE_R;
import static net.consensys.linea.zktracer.Trace.RLP_TXN_PHASE_RLP_PREFIX;
import static net.consensys.linea.zktracer.Trace.RLP_TXN_PHASE_S;
import static net.consensys.linea.zktracer.Trace.RLP_TXN_PHASE_TO;
import static net.consensys.linea.zktracer.Trace.RLP_TXN_PHASE_VALUE;
import static net.consensys.linea.zktracer.Trace.RLP_TXN_PHASE_Y;
import static net.consensys.linea.zktracer.module.Util.getTxTypeAsInt;
import static net.consensys.linea.zktracer.module.rlpUtils.RlpUtils.BYTES_PREFIX_SHORT_INT;
import static net.consensys.linea.zktracer.module.rlpUtils.RlpUtils.BYTES_PREFIX_SHORT_LIST;
import static net.consensys.linea.zktracer.module.rlputilsOld.Pattern.byteCounting;
import static net.consensys.linea.zktracer.module.rlputilsOld.Pattern.innerRlpSize;
import static net.consensys.linea.zktracer.module.rlputilsOld.Pattern.outerRlpSize;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;
import static net.consensys.linea.zktracer.types.Conversions.longToUnsignedBigInteger;
import static net.consensys.linea.zktracer.types.TransactionUtils.getChainIdFromTransaction;
import static net.consensys.linea.zktracer.types.Utils.bitDecomposition;
import static net.consensys.linea.zktracer.types.Utils.leftPadTo;
import static net.consensys.linea.zktracer.types.Utils.rightPadTo;
import static org.hyperledger.besu.ethereum.core.encoding.EncodingContext.BLOCK_BODY;
import static org.hyperledger.besu.ethereum.core.encoding.TransactionEncoder.encodeOpaqueBytes;

import java.math.BigInteger;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.rlptxn.RlpTxn;
import net.consensys.linea.zktracer.module.rlptxn.RlpTxnOperation;
import net.consensys.linea.zktracer.module.rlputilsOld.ByteCountAndPowerOutput;
import net.consensys.linea.zktracer.module.romlex.ContractMetadata;
import net.consensys.linea.zktracer.module.romlex.RomLex;
import net.consensys.linea.zktracer.types.BitDecOutput;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import net.consensys.linea.zktracer.types.UnsignedByte;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.evm.account.AccountState;
import org.hyperledger.besu.evm.worldstate.WorldView;

@RequiredArgsConstructor
@Accessors(fluent = true)
public class LondonRlpTxn extends RlpTxn {
  private final RomLex romLex;

  // Used to check the reconstruction of RLPs
  Bytes reconstructedRlpLt;

  Bytes reconstructedRlpLx;

  @Override
  public void traceStartTx(WorldView worldView, TransactionProcessingMetadata txMetaData) {
    final Transaction tx = txMetaData.getBesuTransaction();
    // Contract Creation
    if (tx.getTo().isEmpty() && !tx.getInit().get().isEmpty()) {
      operations.add(new LondonRlpTxnOperation(tx, true));
    }

    // Call to a non-empty smart contract
    else if (tx.getTo().isPresent()
        && Optional.ofNullable(worldView.get(tx.getTo().orElseThrow()))
            .map(AccountState::hasCode)
            .orElse(false)) {
      operations.add(new LondonRlpTxnOperation(tx, true));
    } else {
      // Contract doesn't require EVM execution
      operations.add(new LondonRlpTxnOperation(tx, false));
    }
  }

  @Override
  public void traceOperation(RlpTxnOperation op, int absTxNum, Trace.Rlptxn trace) {
    final LondonRlpTxnOperation operation = (LondonRlpTxnOperation) op;
    // Create the local row storage and specify transaction constant columns
    final RlpTxnColumnsValue traceValue = new RlpTxnColumnsValue();
    traceValue.resetDataHiLo();
    traceValue.addrHi = bigIntegerToBytes(BigInteger.ZERO);
    traceValue.addrLo = bigIntegerToBytes(BigInteger.ZERO);
    traceValue.absTxNum = absTxNum;
    traceValue.requiresEvmExecution = operation.requireEvmExecution();
    traceValue.codeFragmentIndex =
        operation.tx().getTo().isEmpty() && operation.requireEvmExecution()
            ? this.romLex.getCodeFragmentIndexByMetadata(
                ContractMetadata.make(
                    Address.contractAddress(operation.tx().getSender(), operation.tx().getNonce()),
                    1,
                    true))
            : 0;
    traceValue.txType = getTxTypeAsInt(operation.tx().getType());

    // Initialise RLP_LT and RLP_LX byte size + verify that we construct the right RLP
    this.reconstructedRlpLt = Bytes.EMPTY;
    this.reconstructedRlpLx = Bytes.EMPTY;
    Bytes besuRlpLt =
        encodeOpaqueBytes(
            (org.hyperledger.besu.ethereum.core.Transaction) operation.tx(), BLOCK_BODY);
    // the encodeOpaqueBytes method already concatenate with the first byte "transaction  type"
    if (traceValue.txType == 0) {
      traceValue.rlpLtByteSize = innerRlpSize(besuRlpLt.size());
    } else {
      traceValue.rlpLtByteSize = innerRlpSize(besuRlpLt.size() - 1);
    }

    final Bytes besuRlpLx = operation.tx().encodedPreimage();
    switch (traceValue.txType) {
      case 0 -> traceValue.rlpLxByteSize = innerRlpSize(besuRlpLx.size());
      case 1, 2 ->
          // the innerRlp method already concatenate with the first byte "transaction  type"
          traceValue.rlpLxByteSize = innerRlpSize(besuRlpLx.size() - 1);
      default ->
          throw new IllegalStateException("Transaction Type not supported: " + traceValue.txType);
    }

    // Phase Global RLP prefix
    traceValue.dataLo = BigInteger.valueOf(traceValue.txType);
    handlePhaseGlobalRlpPrefix(traceValue, trace);

    // Phase ChainId
    if (traceValue.txType == 1 || traceValue.txType == 2) {
      checkArgument(
          bigIntegerToBytes(operation.tx().getChainId().orElseThrow()).size() <= 8,
          "ChainId is longer than 8 bytes");
      handlePhaseInteger(
          traceValue, RLP_TXN_PHASE_CHAIN_ID, operation.tx().getChainId().get(), 8, trace);
    }

    // Phase Nonce
    BigInteger nonce = longToUnsignedBigInteger(operation.tx().getNonce());
    traceValue.dataLo = nonce;
    handlePhaseInteger(traceValue, RLP_TXN_PHASE_NONCE, nonce, 8, trace);

    // Phase GasPrice
    if (traceValue.txType == 0 || traceValue.txType == 1) {
      BigInteger gasPrice = operation.tx().getGasPrice().orElseThrow().getAsBigInteger();
      checkArgument(bigIntegerToBytes(gasPrice).size() <= 8, "GasPrice is longer than 8 bytes");
      traceValue.dataLo = gasPrice;
      handlePhaseInteger(traceValue, RLP_TXN_PHASE_GAS_PRICE, gasPrice, 8, trace);
    }

    // Phase Max priority fee per gas (GasTipCap)
    if (traceValue.txType == 2) {
      BigInteger maxPriorityFeePerGas =
          operation.tx().getMaxPriorityFeePerGas().orElseThrow().getAsBigInteger();
      checkArgument(
          bigIntegerToBytes(maxPriorityFeePerGas).size() <= 8,
          "Max Priority Fee per Gas is longer than 8 bytes");
      handlePhaseInteger(
          traceValue, RLP_TXN_PHASE_MAX_PRIORITY_FEE_PER_GAS, maxPriorityFeePerGas, 8, trace);
    }

    // Phase Max fee per gas (GasFeeCap)
    if (traceValue.txType == 2) {
      traceValue.dataHi = operation.tx().getMaxPriorityFeePerGas().orElseThrow().getAsBigInteger();
      BigInteger maxFeePerGas = operation.tx().getMaxFeePerGas().orElseThrow().getAsBigInteger();
      checkArgument(
          bigIntegerToBytes(maxFeePerGas).size() <= 8, "Max Fee per Gas is longer than 8 bytes");
      traceValue.dataLo = maxFeePerGas;
      handlePhaseInteger(traceValue, RLP_TXN_PHASE_MAX_FEE_PER_GAS, maxFeePerGas, 8, trace);
    }

    // Phase GasLimit
    BigInteger gasLimit = BigInteger.valueOf(operation.tx().getGasLimit());
    traceValue.dataLo = gasLimit;
    handlePhaseInteger(traceValue, RLP_TXN_PHASE_GAS_LIMIT, gasLimit, 8, trace);

    // Phase To
    if (operation.tx().getTo().isPresent()) {
      traceValue.dataHi = operation.tx().getTo().orElseThrow().slice(0, 4).toUnsignedBigInteger();
      traceValue.dataLo = operation.tx().getTo().orElseThrow().slice(4, 16).toUnsignedBigInteger();
    } else {
      traceValue.dataHi = BigInteger.ZERO;
      traceValue.dataLo = BigInteger.ZERO;
    }
    handlePhaseTo(traceValue, operation.tx(), trace);

    // Phase Value
    BigInteger value = operation.tx().getValue().getAsBigInteger();
    traceValue.dataLo = value;
    if (operation.tx().getTo().isEmpty()) {
      traceValue.dataHi = BigInteger.ONE;
    } else {
      traceValue.dataHi = BigInteger.ZERO;
    }
    handlePhaseInteger(traceValue, RLP_TXN_PHASE_VALUE, value, LLARGE, trace);

    // Phase Data
    handlePhaseData(traceValue, operation.tx(), trace);

    // Phase AccessList
    if (traceValue.txType == 1 || traceValue.txType == 2) {
      handlePhaseAccessList(traceValue, operation.tx(), trace);
    }

    // Phase Beta / w
    if (traceValue.txType == 0) {
      handlePhaseBeta(traceValue, operation.tx(), trace);
    }

    // Phase y
    if (traceValue.txType == 1 || traceValue.txType == 2) {
      handlePhaseY(traceValue, operation.tx(), trace);
    }

    // Phase R
    handle32BytesInteger(traceValue, RLP_TXN_PHASE_R, operation.tx().getR(), trace);

    // Phase S
    handle32BytesInteger(traceValue, RLP_TXN_PHASE_S, operation.tx().getS(), trace);

    checkArgument(
        this.reconstructedRlpLt.equals(besuRlpLt), "Reconstructed RLP LT and Besu RLP LT differ");
    checkArgument(
        this.reconstructedRlpLx.equals(besuRlpLx), "Reconstructed RLP LX and Besu RLP LX differ");
  }

  // Define each phase's constraints
  private void handlePhaseGlobalRlpPrefix(RlpTxnColumnsValue traceValue, Trace.Rlptxn trace) {
    int phase = RLP_TXN_PHASE_RLP_PREFIX;
    // First, trace the Type prefix of the transaction
    traceValue.partialReset(phase, 1, true, true);
    if (traceValue.txType != 0) {
      traceValue.limbConstructed = true;
      traceValue.limb = bigIntegerToBytes(BigInteger.valueOf(traceValue.txType));
      traceValue.nBytes = 1;
      traceRow(traceValue, trace);
    } else {
      traceValue.lcCorrection = true;
      traceRow(traceValue, trace);
    }

    // RLP prefix of RLP(LT)
    rlpByteString(
        phase,
        traceValue.rlpLtByteSize,
        true,
        true,
        false,
        false,
        false,
        false,
        false,
        traceValue,
        trace);

    // RLP prefix of RLP(LT)
    rlpByteString(
        phase,
        traceValue.rlpLxByteSize,
        true,
        false,
        true,
        false,
        false,
        false,
        true,
        traceValue,
        trace);
  }

  private void handlePhaseInteger(
      RlpTxnColumnsValue traceValue, int phase, BigInteger input, int nbstep, Trace.Rlptxn trace) {
    if (input.equals(BigInteger.ZERO)) {
      traceZeroInt(traceValue, phase, true, true, false, true, trace);
    } else {
      rlpInt(phase, input, nbstep, true, true, false, true, false, traceValue, trace);
    }
  }

  private void handlePhaseTo(RlpTxnColumnsValue traceValue, Transaction tx, Trace.Rlptxn trace) {
    int phase = RLP_TXN_PHASE_TO;
    boolean lt = true;
    boolean lx = true;

    if (tx.getTo().isEmpty()) {
      traceZeroInt(traceValue, phase, lt, lx, false, true, trace);
    } else {
      handleAddress(traceValue, phase, tx.getTo().get(), trace);
    }
  }

  private void handlePhaseData(RlpTxnColumnsValue traceValue, Transaction tx, Trace.Rlptxn trace) {
    final int phase = RLP_TXN_PHASE_DATA;
    final boolean lt = true;
    final boolean lx = true;

    if (tx.getPayload().isEmpty()) {
      // Trivial case
      traceZeroInt(traceValue, phase, lt, lx, true, false, trace);

      // One row of padding
      traceValue.partialReset(phase, 1, lt, lx);
      traceValue.lcCorrection = true;
      traceValue.phaseEnd = true;
      traceRow(traceValue, trace);
    } else {
      // General case

      // Initialise DataSize and DataGasCost
      Bytes data = tx.getPayload();
      traceValue.phaseByteSize = data.size();
      for (int i = 0; i < traceValue.phaseByteSize; i++) {
        if (data.get(i) == 0) {
          traceValue.dataGasCost += GAS_CONST_G_TX_DATA_ZERO;
        } else {
          traceValue.dataGasCost += GAS_CONST_G_TX_DATA_NONZERO;
        }
      }
      traceValue.dataHi = BigInteger.valueOf(traceValue.dataGasCost);
      traceValue.dataLo = BigInteger.valueOf(traceValue.phaseByteSize);

      // Trace
      // RLP prefix
      if (traceValue.phaseByteSize == 1) {
        rlpInt(
            phase,
            tx.getPayload().toUnsignedBigInteger(),
            8,
            lt,
            lx,
            true,
            false,
            true,
            traceValue,
            trace);
      } else {
        // General case
        rlpByteString(
            phase,
            traceValue.phaseByteSize,
            false,
            lt,
            lx,
            true,
            false,
            false,
            false,
            traceValue,
            trace);
      }

      // Tracing the Data: several 16-rows ct-loop
      final int nbstep = LLARGE;
      final int nbloop = (traceValue.phaseByteSize - 1) / nbstep + 1;
      data = rightPadTo(data, nbstep * nbloop);
      for (int i = 0; i < nbloop; i++) {
        traceValue.partialReset(phase, nbstep, lt, lx);
        traceValue.input1 = data.slice(LLARGE * i, LLARGE);
        int accByteSize = 0;
        for (int ct = 0; ct < LLARGE; ct++) {
          traceValue.counter = ct;
          if (traceValue.phaseByteSize != 0) {
            accByteSize += 1;
          }
          traceValue.byte1 = traceValue.input1.get(ct);
          traceValue.acc1 = traceValue.input1.slice(0, ct + 1);
          traceValue.accByteSize = accByteSize;
          if (ct == nbstep - 1) {
            traceValue.limbConstructed = true;
            traceValue.limb = traceValue.input1;
            traceValue.nBytes = accByteSize;
          }
          traceRow(traceValue, trace);
        }
      }
      // Two rows of padding
      traceValue.partialReset(phase, 2, lt, lx);
      traceValue.lcCorrection = true;
      traceRow(traceValue, trace);

      traceValue.counter = 1;
      traceValue.phaseEnd = true;
      traceRow(traceValue, trace);
    }

    // Put INDEX_DATA to 0 at the end of the phase
    traceValue.indexData = 0;
  }

  private void handlePhaseAccessList(
      RlpTxnColumnsValue traceValue, Transaction tx, Trace.Rlptxn trace) {
    int phase = RLP_TXN_PHASE_ACCESS_LIST;
    boolean lt = true;
    boolean lx = true;

    // Trivial case
    if (tx.getAccessList().get().isEmpty()) {
      traceVoidList(traceValue, phase, lt, lx, true, false, false, true, trace);
    } else {
      // Initialise traceValue
      int nbAddr = 0;
      int nbSto = 0;
      List<Integer> nbStoPerAddrList = new ArrayList<>();
      List<Integer> accessTupleByteSizeList = new ArrayList<>();
      int phaseByteSize = 0;
      for (int i = 0; i < tx.getAccessList().orElseThrow().size(); i++) {
        nbAddr += 1;
        nbSto += tx.getAccessList().orElseThrow().get(i).storageKeys().size();
        nbStoPerAddrList.add(tx.getAccessList().orElseThrow().get(i).storageKeys().size());
        accessTupleByteSizeList.add(
            21 + outerRlpSize(33 * tx.getAccessList().orElseThrow().get(i).storageKeys().size()));
        phaseByteSize += outerRlpSize(accessTupleByteSizeList.get(i));
      }

      traceValue.partialReset(phase, 0, lt, lx);
      traceValue.nbAddr = nbAddr;
      traceValue.dataLo = BigInteger.valueOf(nbAddr);
      traceValue.nbSto = nbSto;
      traceValue.dataHi = BigInteger.valueOf(nbSto);
      traceValue.phaseByteSize = phaseByteSize;

      // Trace RLP(Phase Byte Size)
      rlpByteString(
          phase,
          traceValue.phaseByteSize,
          true,
          lt,
          lx,
          true,
          false,
          false,
          false,
          traceValue,
          trace);

      // Loop Over AccessTuple
      for (int i = 0; i < nbAddr; i++) {

        // Update columns at the beginning of an AccessTuple entry
        traceValue.nbAddr -= 1;
        traceValue.nbStoPerAddr = nbStoPerAddrList.get(i);
        traceValue.addrHi = tx.getAccessList().orElseThrow().get(i).address().slice(0, 4);
        traceValue.addrLo = tx.getAccessList().orElseThrow().get(i).address().slice(4, LLARGE);
        traceValue.accessTupleByteSize = accessTupleByteSizeList.get(i);

        // Rlp(AccessTupleByteSize)
        rlpByteString(
            phase,
            traceValue.accessTupleByteSize,
            true,
            lt,
            lx,
            true,
            true,
            false,
            false,
            traceValue,
            trace);

        // RLP (address)
        handleAddress(traceValue, phase, tx.getAccessList().get().get(i).address(), trace);

        // Rlp prefix of the list of storage key
        if (nbStoPerAddrList.get(i) == 0) {
          traceVoidList(
              traceValue,
              phase,
              lt,
              lx,
              true,
              true,
              true,
              ((traceValue.nbSto == 0) && (traceValue.nbAddr == 0)),
              trace);
        } else {
          rlpByteString(
              phase,
              33L * traceValue.nbStoPerAddr,
              true,
              lt,
              lx,
              true,
              true,
              true,
              false,
              traceValue,
              trace);

          // Loop over StorageKey
          for (int j = 0; j < nbStoPerAddrList.get(i); j++) {
            traceValue.nbSto -= 1;
            traceValue.nbStoPerAddr -= 1;
            handleStorageKey(
                traceValue,
                ((traceValue.nbSto == 0) && (traceValue.nbAddr == 0)),
                tx.getAccessList().get().get(i).storageKeys().get(j),
                trace);
          }
        }
        traceValue.addrHi = bigIntegerToBytes(BigInteger.ZERO);
        traceValue.addrLo = bigIntegerToBytes(BigInteger.ZERO);
      }
    }
  }

  private void handlePhaseBeta(RlpTxnColumnsValue traceValue, Transaction tx, Trace.Rlptxn trace) {
    final int phase = RLP_TXN_PHASE_BETA;
    final BigInteger V = tx.getV();
    checkArgument(bigIntegerToBytes(V).size() <= 8, "V is longer than 8 bytes");
    final boolean betaIsZero =
        V.equals(BigInteger.valueOf(27))
            || V.equals(BigInteger.valueOf(28)); // beta = 0 iff (V == 27 or V == 28)

    // Rlp(w)
    rlpInt(
        phase,
        V,
        8,
        true,
        false,
        false,
        betaIsZero,
        false,
        traceValue,
        trace); // end of the phase if beta == 0

    // if beta != 0, then RLP(beta) and then one row with RLP().RLP()
    if (!betaIsZero) {
      final BigInteger beta = BigInteger.valueOf(getChainIdFromTransaction(tx));

      rlpInt(phase, beta, 8, false, true, true, false, false, traceValue, trace);

      traceValue.partialReset(phase, 1, false, true);
      traceValue.limbConstructed = true;
      traceValue.limb = Bytes.concatenate(BYTES_PREFIX_SHORT_INT, BYTES_PREFIX_SHORT_INT);
      traceValue.nBytes = 2;
      traceValue.phaseEnd = true;
      traceRow(traceValue, trace);
    }
  }

  private void handlePhaseY(RlpTxnColumnsValue traceValue, Transaction tx, Trace.Rlptxn trace) {
    traceValue.partialReset(RLP_TXN_PHASE_Y, 1, true, false);
    traceValue.input1 = bigIntegerToBytes(tx.getYParity());
    traceValue.limbConstructed = true;
    if (tx.getYParity().equals(BigInteger.ZERO)) {
      traceValue.limb = BYTES_PREFIX_SHORT_INT;
    } else {
      traceValue.limb = bigIntegerToBytes(BigInteger.ONE);
    }
    traceValue.nBytes = 1;
    traceValue.phaseEnd = true;
    traceRow(traceValue, trace);
  }

  private void rlpByteString(
      int phase,
      long length,
      boolean isList,
      boolean lt,
      boolean lx,
      boolean isPrefix,
      boolean depth1,
      boolean depth2,
      boolean endPhase,
      RlpTxnColumnsValue traceValue,
      Trace.Rlptxn trace) {
    int lengthSize = bigIntegerToBytes(BigInteger.valueOf(length)).size();

    ByteCountAndPowerOutput byteCountingOutput = byteCounting(lengthSize, 8);

    traceValue.partialReset(phase, 8, lt, lx);
    traceValue.input1 = bigIntegerToBytes(BigInteger.valueOf(length));
    traceValue.isPrefix = isPrefix;
    traceValue.depth1 = depth1;
    traceValue.depth2 = depth2;

    Bytes input1RightShift = leftPadTo(traceValue.input1, 8);

    long acc2LastRow;
    if (length >= 56) {
      acc2LastRow = length - 56;
    } else {
      acc2LastRow = 55 - length;
    }
    Bytes acc2LastRowShift = leftPadTo(bigIntegerToBytes(BigInteger.valueOf(acc2LastRow)), 8);

    for (int ct = 0; ct < 8; ct++) {
      traceValue.counter = ct;
      traceValue.accByteSize = byteCountingOutput.accByteSizeList().get(ct);
      traceValue.power = byteCountingOutput.powerList().get(ct);
      traceValue.byte1 = input1RightShift.get(ct);
      traceValue.acc1 = input1RightShift.slice(0, ct + 1);
      traceValue.byte2 = acc2LastRowShift.get(ct);
      traceValue.acc2 = acc2LastRowShift.slice(0, ct + 1);

      if (length >= 56) {
        if (ct == 6) {
          traceValue.limbConstructed = true;
          traceValue.nBytes = 1;
          BigInteger tmp;
          if (isList) {
            tmp = BigInteger.valueOf(RLP_PREFIX_LIST_LONG + lengthSize);
          } else {
            tmp = BigInteger.valueOf(RLP_PREFIX_INT_LONG + lengthSize);
          }
          traceValue.limb = bigIntegerToBytes(tmp);
        }

        if (ct == 7) {
          traceValue.limb = traceValue.input1;
          traceValue.nBytes = lengthSize;
          traceValue.bit = true;
          traceValue.bitAcc = 1;
          traceValue.phaseEnd = endPhase;
        }
      } else {
        if (ct == 7) {
          traceValue.limbConstructed = true;
          Bytes tmp;
          if (isList) {
            tmp = bigIntegerToBytes(BigInteger.valueOf(RLP_PREFIX_LIST_SHORT + length));
          } else {
            tmp = bigIntegerToBytes(BigInteger.valueOf(RLP_PREFIX_INT_SHORT + length));
          }
          traceValue.limb = tmp;
          traceValue.nBytes = 1;
          traceValue.phaseEnd = endPhase;
        }
      }
      traceRow(traceValue, trace);
    }
  }

  private void rlpInt(
      int phase,
      BigInteger input,
      int nStep,
      boolean lt,
      boolean lx,
      boolean isPrefix,
      boolean endPhase,
      boolean onlyPrefix,
      RlpTxnColumnsValue traceValue,
      Trace.Rlptxn trace) {

    traceValue.partialReset(phase, nStep, lt, lx);
    traceValue.isPrefix = isPrefix;

    Bytes inputByte = bigIntegerToBytes(input);
    int inputSize = inputByte.size();
    ByteCountAndPowerOutput byteCountingOutput = byteCounting(inputSize, nStep);

    Bytes inputBytePadded = leftPadTo(inputByte, nStep);
    BitDecOutput bitDecOutput =
        bitDecomposition(0xff & inputBytePadded.get(inputBytePadded.size() - 1), nStep);

    traceValue.input1 = inputByte;

    for (int ct = 0; ct < nStep; ct++) {
      traceValue.counter = ct;
      traceValue.byte1 = inputBytePadded.get(ct);
      traceValue.acc1 = inputBytePadded.slice(0, ct + 1);
      traceValue.power = byteCountingOutput.powerList().get(ct);
      traceValue.accByteSize = byteCountingOutput.accByteSizeList().get(ct);
      traceValue.bit = bitDecOutput.bitDecList().get(ct);
      traceValue.bitAcc = bitDecOutput.bitAccList().get(ct);

      if (input.compareTo(BigInteger.valueOf(128)) >= 0 && ct == nStep - 2) {
        traceValue.limbConstructed = true;
        traceValue.limb = bigIntegerToBytes(BigInteger.valueOf(RLP_PREFIX_INT_SHORT + inputSize));
        traceValue.nBytes = 1;
      }

      if (ct == nStep - 1) {
        if (onlyPrefix) {
          traceValue.lcCorrection = true;
          traceValue.limbConstructed = false;
          traceValue.limb = Bytes.ofUnsignedShort(0);
          traceValue.nBytes = 0;
        } else {
          traceValue.limbConstructed = true;
          traceValue.limb = inputByte;
          traceValue.nBytes = inputSize;
          traceValue.phaseEnd = endPhase;
        }
      }
      traceRow(traceValue, trace);
    }
  }

  private void handle32BytesInteger(
      RlpTxnColumnsValue traceValue, int phase, BigInteger input, Trace.Rlptxn trace) {
    traceValue.partialReset(phase, LLARGE, true, false);
    if (input.equals(BigInteger.ZERO)) {
      // Trivial case
      traceZeroInt(traceValue, phase, true, false, false, true, trace);
    } else {
      // General case
      Bytes inputByte = bigIntegerToBytes(input);
      int inputLen = inputByte.size();
      Bytes inputByte32 = leftPadTo(inputByte, 32);
      traceValue.input1 = inputByte32.slice(0, LLARGE);
      traceValue.input2 = inputByte32.slice(LLARGE, LLARGE);

      ByteCountAndPowerOutput byteCounting;
      if (inputLen <= traceValue.nStep) {
        ByteCountAndPowerOutput byteCountingOutput = byteCounting(inputLen, traceValue.nStep);
        BitDecOutput bitDecOutput =
            bitDecomposition((0xFF & inputByte.get(inputByte.size() - 1)), traceValue.nStep);

        for (int ct = 0; ct < traceValue.nStep; ct++) {
          traceValue.counter = ct;
          traceValue.byte2 = traceValue.input2.get(ct);
          traceValue.acc2 = traceValue.input2.slice(0, ct + 1);
          traceValue.accByteSize = byteCountingOutput.accByteSizeList().get(ct);
          traceValue.power = byteCountingOutput.powerList().get(ct);
          traceValue.bit = bitDecOutput.bitDecList().get(ct);
          traceValue.bitAcc = bitDecOutput.bitAccList().get(ct);

          // if input >= 128, there is a RLP prefix, nothing if 0 < input < 128
          if (ct == traceValue.nStep - 2 && input.compareTo(BigInteger.valueOf(128)) >= 0) {
            traceValue.limbConstructed = true;
            traceValue.limb =
                bigIntegerToBytes(BigInteger.valueOf(RLP_PREFIX_INT_SHORT + inputLen));
            traceValue.nBytes = 1;
          }
          if (ct == traceValue.nStep - 1) {
            traceValue.limbConstructed = true;
            traceValue.limb = traceValue.input2.slice(LLARGE - inputLen, inputLen);
            traceValue.nBytes = inputLen;
            traceValue.phaseEnd = true;
          }
          traceRow(traceValue, trace);
        }
      } else {
        inputLen -= traceValue.nStep;
        byteCounting = byteCounting(inputLen, traceValue.nStep);

        for (int ct = 0; ct < traceValue.nStep; ct++) {
          traceValue.counter = ct;
          traceValue.byte1 = traceValue.input1.get(ct);
          traceValue.acc1 = traceValue.input1.slice(0, ct + 1);
          traceValue.byte2 = traceValue.input2.get(ct);
          traceValue.acc2 = traceValue.input2.slice(0, ct + 1);
          traceValue.accByteSize = byteCounting.accByteSizeList().get(ct);
          traceValue.power = byteCounting.powerList().get(ct);

          if (ct == traceValue.nStep - 3) {
            traceValue.limbConstructed = true;
            traceValue.limb =
                bigIntegerToBytes(BigInteger.valueOf(RLP_PREFIX_INT_SHORT + LLARGE + inputLen));
            traceValue.nBytes = 1;
          }
          if (ct == traceValue.nStep - 2) {
            traceValue.limb = traceValue.input1.slice(LLARGE - inputLen, inputLen);
            traceValue.nBytes = inputLen;
          }
          if (ct == traceValue.nStep - 1) {
            traceValue.limb = traceValue.input2;
            traceValue.nBytes = LLARGE;
            traceValue.phaseEnd = true;
          }
          traceRow(traceValue, trace);
        }
      }
    }
  }

  private void handleAddress(
      RlpTxnColumnsValue traceValue, int phase, Address address, Trace.Rlptxn trace) {
    boolean lt = true;
    boolean lx = true;
    traceValue.partialReset(phase, LLARGE, lt, lx);
    traceValue.input1 = leftPadTo(address.slice(0, 4), LLARGE);
    traceValue.input2 = address.slice(4, LLARGE);

    if (phase == RLP_TXN_PHASE_ACCESS_LIST) {
      traceValue.depth1 = true;
    }

    for (int ct = 0; ct < traceValue.nStep; ct++) {
      traceValue.counter = ct;
      traceValue.byte1 = traceValue.input1.get(ct);
      traceValue.acc1 = traceValue.input1.slice(0, ct + 1);
      traceValue.byte2 = traceValue.input2.get(ct);
      traceValue.acc2 = traceValue.input2.slice(0, ct + 1);

      if (ct == traceValue.nStep - 3) {
        traceValue.limbConstructed = true;
        traceValue.limb = bigIntegerToBytes(BigInteger.valueOf(RLP_PREFIX_INT_SHORT + 20));
        traceValue.nBytes = 1;
      }

      if (ct == traceValue.nStep - 2) {
        traceValue.limb = address.slice(0, 4);
        traceValue.nBytes = 4;
      }
      if (ct == traceValue.nStep - 1) {
        traceValue.limb = traceValue.input2;
        traceValue.nBytes = LLARGE;

        if (phase == RLP_TXN_PHASE_TO) {
          traceValue.phaseEnd = true;
        }
      }
      traceRow(traceValue, trace);
    }
  }

  private void handleStorageKey(
      RlpTxnColumnsValue traceValue, boolean end_phase, Bytes32 storage_key, Trace.Rlptxn trace) {
    traceValue.partialReset(RLP_TXN_PHASE_ACCESS_LIST, LLARGE, true, true);
    traceValue.depth1 = true;
    traceValue.depth2 = true;
    traceValue.input1 = storage_key.slice(0, LLARGE);
    traceValue.input2 = storage_key.slice(LLARGE, LLARGE);

    for (int ct = 0; ct < traceValue.nStep; ct++) {
      traceValue.counter = ct;
      traceValue.byte1 = traceValue.input1.get(ct);
      traceValue.acc1 = traceValue.input1.slice(0, ct + 1);
      traceValue.byte2 = traceValue.input2.get(ct);
      traceValue.acc2 = traceValue.input2.slice(0, ct + 1);

      if (ct == traceValue.nStep - 3) {
        traceValue.limbConstructed = true;
        traceValue.limb = bigIntegerToBytes(BigInteger.valueOf(RLP_PREFIX_INT_SHORT + 32));
        traceValue.nBytes = 1;
      }

      if (ct == traceValue.nStep - 2) {
        traceValue.limb = traceValue.input1;
        traceValue.nBytes = LLARGE;
      }

      if (ct == traceValue.nStep - 1) {
        traceValue.limb = traceValue.input2;
        traceValue.phaseEnd = end_phase;
      }

      traceRow(traceValue, trace);
    }
  }

  private void traceZeroInt(
      RlpTxnColumnsValue traceValue,
      int phase,
      boolean lt,
      boolean lx,
      boolean isPrefix,
      boolean phaseEnd,
      Trace.Rlptxn trace) {
    traceValue.partialReset(phase, 1, lt, lx);
    traceValue.limbConstructed = true;
    traceValue.limb = BYTES_PREFIX_SHORT_INT;
    traceValue.nBytes = 1;
    traceValue.isPrefix = true;
    traceValue.phaseEnd = phaseEnd;
    traceRow(traceValue, trace);
  }

  private void traceVoidList(
      RlpTxnColumnsValue traceValue,
      int phase,
      boolean lt,
      boolean lx,
      boolean isPrefix,
      boolean depth1,
      boolean depth2,
      boolean phaseEnd,
      Trace.Rlptxn trace) {
    traceValue.partialReset(phase, 1, lt, lx);
    traceValue.limbConstructed = true;
    traceValue.limb = BYTES_PREFIX_SHORT_LIST;
    traceValue.nBytes = 1;
    traceValue.isPrefix = isPrefix;
    traceValue.depth1 = depth1;
    traceValue.depth2 = depth2;
    traceValue.phaseEnd = phaseEnd;
    traceRow(traceValue, trace);
  }

  // Define the Tracer
  private void traceRow(RlpTxnColumnsValue traceValue, Trace.Rlptxn builder) {
    // Decrements RLP_BYTESIZE
    if (traceValue.phase != RLP_TXN_PHASE_RLP_PREFIX) {
      if (traceValue.limbConstructed && traceValue.lt) {
        traceValue.rlpLtByteSize -= traceValue.nBytes;
      }
      if (traceValue.limbConstructed && traceValue.lx) {
        traceValue.rlpLxByteSize -= traceValue.nBytes;
      }
    }

    // Decrement phaseByteSize and accessTupleByteSize for Phase AccessList
    if (traceValue.phase == RLP_TXN_PHASE_ACCESS_LIST) {
      // Decreases PhaseByteSize
      if (traceValue.depth1 && traceValue.limbConstructed) {
        traceValue.phaseByteSize -= traceValue.nBytes;
      }
      // Decreases AccessTupleSize
      if (traceValue.depth1
          && !(traceValue.isPrefix && !traceValue.depth2)
          && traceValue.limbConstructed) {
        traceValue.accessTupleByteSize -= traceValue.nBytes;
      }
    }

    builder
        .absTxNum(traceValue.absTxNum)
        .absTxNumInfiny(operations.size())
        .acc1(traceValue.acc1)
        .acc2(traceValue.acc2)
        .accBytesize((short) traceValue.accByteSize)
        .accessTupleBytesize(traceValue.accessTupleByteSize)
        .addrHi(traceValue.addrHi.toLong())
        .addrLo(traceValue.addrLo)
        .bit(traceValue.bit)
        .bitAcc(UnsignedByte.of(traceValue.bitAcc))
        .byte1(UnsignedByte.of(traceValue.byte1))
        .byte2(UnsignedByte.of(traceValue.byte2))
        .codeFragmentIndex(traceValue.codeFragmentIndex)
        .counter((short) traceValue.counter)
        .dataHi(bigIntegerToBytes(traceValue.dataHi))
        .dataLo(bigIntegerToBytes(traceValue.dataLo))
        .dataGasCost(traceValue.dataGasCost)
        .depth1(traceValue.depth1)
        .depth2(traceValue.depth2)
        .done(traceValue.counter == traceValue.nStep - 1)
        .phaseEnd(traceValue.phaseEnd)
        .indexData(traceValue.indexData)
        .indexLt(traceValue.indexLt)
        .indexLx(traceValue.indexLx)
        .input1(traceValue.input1)
        .input2(traceValue.input2)
        .lcCorrection(traceValue.lcCorrection)
        .isPrefix(traceValue.isPrefix)
        .limb(rightPadTo(traceValue.limb, LLARGE))
        .limbConstructed(traceValue.limbConstructed)
        .lt(traceValue.lt)
        .lx(traceValue.lx)
        .toHashByProver(traceValue.limbConstructed && traceValue.lx)
        .nBytes((short) traceValue.nBytes)
        .nAddr(traceValue.nbAddr)
        .nKeys(traceValue.nbSto)
        .nKeysPerAddr(traceValue.nbStoPerAddr)
        .nStep((short) traceValue.nStep)
        .phase((short) traceValue.phase)
        .isPhaseRlpPrefix(traceValue.phase == RLP_TXN_PHASE_RLP_PREFIX)
        .isPhaseChainId(traceValue.phase == RLP_TXN_PHASE_CHAIN_ID)
        .isPhaseNonce(traceValue.phase == RLP_TXN_PHASE_NONCE)
        .isPhaseGasPrice(traceValue.phase == RLP_TXN_PHASE_GAS_PRICE)
        .isPhaseMaxPriorityFeePerGas(traceValue.phase == RLP_TXN_PHASE_MAX_PRIORITY_FEE_PER_GAS)
        .isPhaseMaxFeePerGas(traceValue.phase == RLP_TXN_PHASE_MAX_FEE_PER_GAS)
        .isPhaseGasLimit(traceValue.phase == RLP_TXN_PHASE_GAS_LIMIT)
        .isPhaseTo(traceValue.phase == RLP_TXN_PHASE_TO)
        .isPhaseValue(traceValue.phase == RLP_TXN_PHASE_VALUE)
        .isPhaseData(traceValue.phase == RLP_TXN_PHASE_DATA)
        .isPhaseAccessList(traceValue.phase == RLP_TXN_PHASE_ACCESS_LIST)
        .isPhaseBeta(traceValue.phase == RLP_TXN_PHASE_BETA)
        .isPhaseY(traceValue.phase == RLP_TXN_PHASE_Y)
        .isPhaseR(traceValue.phase == RLP_TXN_PHASE_R)
        .isPhaseS(traceValue.phase == RLP_TXN_PHASE_S)
        .phaseSize(traceValue.phaseByteSize)
        .power(bigIntegerToBytes(traceValue.power))
        .requiresEvmExecution(traceValue.requiresEvmExecution)
        .rlpLtBytesize(traceValue.rlpLtByteSize)
        .rlpLxBytesize(traceValue.rlpLxByteSize)
        .type((short) traceValue.txType);

    // Increments Index
    if (traceValue.limbConstructed && traceValue.lt) {
      traceValue.indexLt += 1;
    }
    if (traceValue.limbConstructed && traceValue.lx) {
      traceValue.indexLx += 1;
    }

    // Increments IndexData Phase Data
    if (traceValue.phase == RLP_TXN_PHASE_DATA
        && !traceValue.isPrefix
        && (traceValue.limbConstructed || traceValue.lcCorrection)) {
      traceValue.indexData += 1;
    }

    // Decrements PhaseByteSize and DataGasCost in Data phase
    if (traceValue.phase == RLP_TXN_PHASE_DATA) {
      if (traceValue.phaseByteSize != 0 && !traceValue.isPrefix) {
        traceValue.phaseByteSize -= 1;
        if (traceValue.byte1 == 0) {
          traceValue.dataGasCost -= GAS_CONST_G_TX_DATA_ZERO;
        } else {
          traceValue.dataGasCost -= GAS_CONST_G_TX_DATA_NONZERO;
        }
      }
    }

    // Reset Data Hi and Data Lo if end of the phase
    if (traceValue.phaseEnd) {
      traceValue.resetDataHiLo();
    }
    builder.validateRow();

    // reconstruct RLPs
    if (traceValue.limbConstructed && traceValue.lt) {
      this.reconstructedRlpLt =
          Bytes.concatenate(this.reconstructedRlpLt, traceValue.limb.slice(0, traceValue.nBytes));
    }
    if (traceValue.limbConstructed && traceValue.lx) {
      this.reconstructedRlpLx =
          Bytes.concatenate(this.reconstructedRlpLx, traceValue.limb.slice(0, traceValue.nBytes));
    }
  }
}

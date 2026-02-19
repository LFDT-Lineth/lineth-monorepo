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

package net.consensys.linea.zktracer.module.romlex;

import static com.google.common.base.Preconditions.*;
import static net.consensys.linea.zktracer.Trace.*;
import static net.consensys.linea.zktracer.module.ModuleName.ROM_LEX;
import static net.consensys.linea.zktracer.types.AddressUtils.getDeploymentAddress;
import static net.consensys.linea.zktracer.types.AddressUtils.highPart;
import static net.consensys.linea.zktracer.types.AddressUtils.lowPart;
import static net.consensys.linea.zktracer.types.Conversions.bytesToLong;

import com.google.common.base.Preconditions;
import java.util.*;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.OperationSetModule;
import net.consensys.linea.zktracer.container.stacked.ModuleOperationStackedSet;
import net.consensys.linea.zktracer.module.ModuleName;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.defer.ContextEntryDefer;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.evm.account.AccountState;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.internal.Words;
import org.hyperledger.besu.evm.worldstate.WorldView;

@Accessors(fluent = true)
@RequiredArgsConstructor
public class RomLex implements OperationSetModule<RomOperation>, ContextEntryDefer {

  private final Hub hub;

  @Getter
  private final ModuleOperationStackedSet<RomOperation> operations =
      new ModuleOperationStackedSet<>();

  @Getter private List<RomOperation> sortedOperations;
  Map<ContractMetadata, Integer> cfiMetadataCorrespondance = new HashMap<>();
  @Getter private Bytes byteCode = Bytes.EMPTY;
  private Address address = Address.ZERO;

  @Getter private final DeferRegistry createDefers = new DeferRegistry();

  @Override
  public ModuleName moduleKey() {
    return ROM_LEX;
  }

  public int getCodeFragmentIndexByMetadata(final ContractMetadata metadata) {
    if (sortedOperations.isEmpty()) {
      throw new RuntimeException("Chunks have not been sorted yet");
    }

    final Integer romOps = cfiMetadataCorrespondance.get(metadata);
    if (romOps == null) {
      throw new RuntimeException(
          "RomChunk with:"
              + String.format("\n\t\taddress = %s", metadata.address())
              + String.format("\n\t\tdeployment number = %s", metadata.deploymentNumber())
              + String.format("\n\t\tdeployment status = %s", metadata.underDeployment())
              + "\n\tnot found");
    }
    return romOps;
  }

  public Optional<RomOperation> getChunkByMetadata(final ContractMetadata metadata) {
    // First search in the chunk added in the current transaction
    for (RomOperation c : operations.operationsInTransactionBundle().keySet()) {
      if (c.metadata().equals(metadata)) {
        return Optional.of(c);
      }
    }

    // If not found, search in the chunk added since the beginning of the conflation
    for (RomOperation c : operations.operationsCommitedToTheConflation().keySet()) {
      if (c.metadata().equals(metadata)) {
        return Optional.of(c);
      }
    }
    throw new RuntimeException(
        "RomChunk with:"
            + String.format("\n\t\taddress = %s", metadata.address())
            + String.format("\n\t\tdeployment number = %s", metadata.deploymentNumber())
            + String.format("\n\t\tdeployment status = %s", metadata.underDeployment())
            + "\n\tnot found");
  }

  public Bytes getCodeByMetadata(final ContractMetadata metadata) {
    return getChunkByMetadata(metadata).map(RomOperation::byteCode).orElseThrow();
  }

  @Override
  public void traceStartTx(WorldView worldView, TransactionProcessingMetadata txMetaData) {
    final Transaction tx = txMetaData.getBesuTransaction();
    // Contract creation with InitCode
    if (tx.getInit().isPresent() && !tx.getInit().get().isEmpty()) {
      final Address deploymentAddress = Address.contractAddress(tx.getSender(), tx.getNonce());
      final RomOperation operation =
          new RomOperation(
              ContractMetadata.canonical(hub, deploymentAddress),
              tx.getInit().get(),
              hub.opCodes());

      operations.add(operation);
    }

    // Call to an account with bytecode
    tx.getTo()
        .map(worldView::get)
        .map(AccountState::getCode)
        .ifPresent(
            code -> {
              if (!code.isEmpty()) {

                final Address calledAddress = tx.getTo().get();
                final RomOperation operation =
                    new RomOperation(
                        ContractMetadata.canonical(hub, calledAddress), code, hub.opCodes());

                operations.add(operation);
              }
            });
  }

  public void callRomLex(final MessageFrame frame) {
    OpCodeData opCode = hub.opCodeData(frame);

    switch (opCode.mnemonic()) {
      case CREATE, CREATE2 -> {
        final long offset = Words.clampedToLong(frame.getStackItem(1));
        final long length = Words.clampedToLong(frame.getStackItem(2));

        checkArgument(length > 0, "callRomLex expects positive size for CREATE(2)");

        hub.defers().scheduleForContextEntry(this);
        byteCode = frame.shadowReadMemory(offset, length);
        address = getDeploymentAddress(frame, opCode);
      }

      case RETURN -> {
        final int currentDeploymentNumber = hub.deploymentNumberOfBytecodeAddress();
        final boolean currentDeploymentStatus = hub.deploymentStatusOfBytecodeAddress();

        final long offset = Words.clampedToLong(frame.getStackItem(0));
        final long length = Words.clampedToLong(frame.getStackItem(1));

        checkArgument(
            frame.getType() == MessageFrame.Type.CONTRACT_CREATION
                && currentDeploymentNumber > 0
                && currentDeploymentStatus,
            "callRomLex for RETURN expects the byte code address to be under deployment, yet:"
                + String.format("\n\t\tframe.getType() = %s", frame.getType())
                + String.format(
                    "\n\t\tMessageFrame contract address = %s", frame.getContractAddress())
                + String.format(
                    "\n\t\tCallFrame    bytecode address = %s",
                    hub.currentFrame().byteCodeAddress())
                + String.format("\n\t\tdeployment number = %s", currentDeploymentNumber)
                + String.format("\n\t\tdeployment status = %s", currentDeploymentStatus));
        checkArgument(length > 0, "callRomLex for RETURN expects positive size");

        byteCode = frame.shadowReadMemory(offset, length);
        final Address deploymentAddress = hub.currentFrame().byteCodeAddress();
        final ContractMetadata contractMetadata =
            ContractMetadata.make(
                deploymentAddress,
                hub.deploymentNumberOf(deploymentAddress),
                false,
                hub.delegationNumberOf(deploymentAddress));
        final RomOperation chunk = new RomOperation(contractMetadata, byteCode, hub.opCodes());
        operations.add(chunk);
      }

      case CALL, CALLCODE, DELEGATECALL, STATICCALL -> {
        final Address calleeAddress = Words.toAddress(frame.getStackItem(1));

        Optional.ofNullable(frame.getWorldUpdater().get(calleeAddress))
            .map(AccountState::getCode)
            .ifPresent(
                byteCode -> {
                  if (!byteCode.isEmpty()) {
                    final RomOperation operation =
                        new RomOperation(
                            ContractMetadata.canonical(hub, calleeAddress),
                            byteCode,
                            hub.opCodes());
                    operations.add(operation);
                  }
                });
      }

      case EXTCODECOPY -> {
        final Address foreignCodeAddress = Words.toAddress(frame.getStackItem(0));
        final long length = Words.clampedToLong(frame.getStackItem(3));

        checkArgument(
            length > 0,
            "EXTCODECOPY should only trigger a ROM_LEX chunk if nonzero size parameter");
        checkArgument(
            !hub.deploymentStatusOf(foreignCodeAddress),
            "EXTCODECOPY should only trigger a ROM_LEX chunk if its target isn't currently deploying");
        checkArgument(
            !frame.getWorldUpdater().get(foreignCodeAddress).isEmpty()
                && frame.getWorldUpdater().get(foreignCodeAddress).hasCode());

        Optional.ofNullable(frame.getWorldUpdater().get(foreignCodeAddress))
            .map(AccountState::getCode)
            .ifPresent(
                byteCode -> {
                  if (!byteCode.isEmpty()) {
                    final RomOperation operation =
                        new RomOperation(
                            ContractMetadata.canonical(hub, foreignCodeAddress),
                            byteCode,
                            hub.opCodes());

                    operations.add(operation);
                  }
                });
      }

      default ->
          throw new RuntimeException(
              String.format("%s does not trigger the creation of ROM_LEX", opCode.mnemonic()));
    }
  }

  @Override
  public void resolveUponContextEntry(Hub hub, MessageFrame frame) {
    checkArgument(frame.getType() == MessageFrame.Type.CONTRACT_CREATION);
    checkArgument(
        hub.deploymentStatusOf(address), "After a CREATE the deployment status should be true");

    final ContractMetadata contractMetadata = ContractMetadata.canonical(hub, address);

    final RomOperation operation = new RomOperation(contractMetadata, byteCode, hub.opCodes());
    operations.add(operation);
    createDefers.trigger(contractMetadata);
  }

  // This is the tracing for ROMLEX module
  private void traceOperation(
      final RomOperation operation,
      final int cfi,
      final int codeFragmentIndexInfinity,
      Trace.Romlex trace) {
    final Hash codeHash =
        operation.metadata().underDeployment() ? Hash.EMPTY : Hash.hash(operation.byteCode());
    final boolean couldBeDelegationCode =
        operation.byteCode().size() == EIP_7702_DELEGATED_ACCOUNT_CODE_SIZE
            && !operation.metadata().underDeployment();
    final long leadingThreeBytes =
        couldBeDelegationCode ? bytesToLong(operation.byteCode().slice(0, 3)) : 0;
    final boolean actuallyDelegationCode = leadingThreeBytes == EIP_7702_DELEGATION_INDICATOR;
    final long potentiallyAddressHi =
        couldBeDelegationCode ? bytesToLong(operation.byteCode().slice(3, 4)) : 0;
    final Bytes potentiallyAddressLo =
        couldBeDelegationCode ? operation.byteCode().slice(7, LLARGE) : Bytes.EMPTY;
    trace
        .codeFragmentIndex(cfi)
        .codeFragmentIndexInfty(codeFragmentIndexInfinity)
        .codeSize(operation.byteCode().size())
        .addressHi(highPart(operation.metadata().address()))
        .addressLo(lowPart(operation.metadata().address()))
        .deploymentNumber(operation.metadata().deploymentNumber())
        .deploymentStatus(operation.metadata().underDeployment())
        .delegationNumber(operation.metadata().delegationNumber())
        .couldBeDelegationCode(couldBeDelegationCode)
        .actuallyDelegationCode(actuallyDelegationCode)
        .leadingThreeBytes(leadingThreeBytes)
        .leadDelegationBytes(potentiallyAddressHi)
        .tailDelegationBytes(potentiallyAddressLo)
        .delegationAddressHi(actuallyDelegationCode ? potentiallyAddressHi : 0)
        .delegationAddressLo(actuallyDelegationCode ? potentiallyAddressLo : Bytes.EMPTY)
        .codeHashHi(codeHash.slice(0, LLARGE))
        .codeHashLo(codeHash.slice(LLARGE, LLARGE))
        .validateRow();
  }

  public void determineCodeFragmentIndex() {
    operations.finishConflation();
    sortedOperations = new ArrayList<>(operations.getAll());
    final RomOperationComparator ROM_CHUNK_COMPARATOR = new RomOperationComparator();
    sortedOperations.sort(ROM_CHUNK_COMPARATOR);
    for (int i = 0; i < sortedOperations.size(); i++) {
      final RomOperation romOperation = sortedOperations.get(i);
      cfiMetadataCorrespondance.put(romOperation.metadata(), i + 1);
    }
  }

  @Override
  public int lineCount() {
    // WARN: the line count for the RomLex is the *number of code fragments*, not their actual line
    // count – that's for the ROM.
    return operations.size();
  }

  @Override
  public int spillage(Trace trace) {
    return trace.romlex().spillage();
  }

  @Override
  public void traceEndConflation(final WorldView state) {
    Preconditions.checkArgument(
        operations.conflationFinished(), "Conflation is done before traceEndConflation for RomLex");
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.romlex().headers(this.lineCount());
  }

  @Override
  public void commit(Trace trace) {
    final int codeFragmentIndexInfinity = operations.size();

    int cfi = 0;
    for (RomOperation operation : sortedOperations) {
      traceOperation(operation, ++cfi, codeFragmentIndexInfinity, trace.romlex());
    }
  }
}

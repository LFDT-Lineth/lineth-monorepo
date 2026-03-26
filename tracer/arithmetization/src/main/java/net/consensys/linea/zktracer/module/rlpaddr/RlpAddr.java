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

package net.consensys.linea.zktracer.module.rlpaddr;

import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.Trace.RLP_ADDR_RECIPE_1;
import static net.consensys.linea.zktracer.Trace.RLP_ADDR_RECIPE_2;
import static net.consensys.linea.zktracer.Trace.RLP_PREFIX_LIST_SHORT;
import static net.consensys.linea.zktracer.Trace.Rlpaddr.MAX_CT_CREATE;
import static net.consensys.linea.zktracer.Trace.Rlpaddr.MAX_CT_CREATE2;
import static net.consensys.linea.zktracer.module.ModuleName.RLP_ADDR;
import static net.consensys.linea.zktracer.module.rlpUtils.RlpUtils.BYTES_PREFIX_SHORT_INT;
import static net.consensys.linea.zktracer.module.rlputilsOld.Pattern.byteCounting;
import static net.consensys.linea.zktracer.types.AddressUtils.getCreate2RawAddress;
import static net.consensys.linea.zktracer.types.AddressUtils.getCreateRawAddress;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;
import static net.consensys.linea.zktracer.types.Conversions.longToUnsignedBigInteger;
import static net.consensys.linea.zktracer.types.Utils.bitDecomposition;
import static net.consensys.linea.zktracer.types.Utils.leftPadTo;
import static net.consensys.linea.zktracer.types.Utils.rightPadTo;

import java.math.BigInteger;
import java.util.List;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.OperationSetModule;
import net.consensys.linea.zktracer.container.stacked.ModuleOperationStackedSet;
import net.consensys.linea.zktracer.module.ModuleName;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.limits.Keccak;
import net.consensys.linea.zktracer.module.rlputilsOld.ByteCountAndPowerOutput;
import net.consensys.linea.zktracer.module.trm.Trm;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.types.BitDecOutput;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import net.consensys.linea.zktracer.types.UnsignedByte;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.worldstate.WorldView;

@RequiredArgsConstructor
@Accessors(fluent = true)
public class RlpAddr implements OperationSetModule<RlpAddrOperation> {
  @Getter
  private final ModuleOperationStackedSet<RlpAddrOperation> operations =
      new ModuleOperationStackedSet<>();

  public static final Bytes CREATE2_SHIFT = Bytes.minimalBytes(Trace.CREATE2_SHIFT);
  private static final UnsignedByte BYTES_LLARGE = UnsignedByte.of(LLARGE);
  final int recipe1NbRows = MAX_CT_CREATE + 1;

  private final Hub hub;
  private final Trm trm;
  private final Keccak keccak;

  @Override
  public ModuleName moduleKey() {
    return RLP_ADDR;
  }

  @Override
  public void traceStartTx(WorldView world, TransactionProcessingMetadata txMetaData) {
    final Transaction tx = txMetaData.getBesuTransaction();
    if (tx.getTo().isEmpty()) {
      final Address senderAddress = tx.getSender();
      final long nonce = tx.getNonce();
      final Bytes32 rawTo = getCreateRawAddress(senderAddress, nonce);
      final RlpAddrOperation operation =
          new RlpAddrOperation(
              keccak, rawTo, OpCode.CREATE, longToUnsignedBigInteger(nonce), senderAddress);
      operations.add(operation);
      trm.callTrimming(rawTo);
    }
  }

  public void callRlpAddrCreate() {
    final MessageFrame frame = hub.currentFrame().frame();
    final Address currentAddress = frame.getRecipientAddress();
    final Bytes32 rawCreateAddress = getCreateRawAddress(frame);
    final RlpAddrOperation operation =
        new RlpAddrOperation(
            keccak,
            rawCreateAddress,
            OpCode.CREATE,
            longToUnsignedBigInteger(frame.getWorldUpdater().get(currentAddress).getNonce()),
            currentAddress);
    operations.add(operation);
    hub.trm().callTrimming(rawCreateAddress);
  }

  public void callRlpAddrCreate2(MessageFrame frame, Bytes32 salt, Bytes32 hash) {
    final Address currentAddress = frame.getRecipientAddress();
    final Bytes32 rawCreate2Address = getCreate2RawAddress(currentAddress, salt, hash);
    final RlpAddrOperation operation =
        new RlpAddrOperation(
            keccak,
            rawCreate2Address,
            OpCode.CREATE2,
            currentAddress,
            salt,
            hash,
            longToUnsignedBigInteger(frame.getWorldUpdater().get(currentAddress).getNonce()));
    operations.add(operation);
    hub.trm().callTrimming(rawCreate2Address);
  }

  private void traceCreate2(int stamp, RlpAddrOperation chunk, Trace.Rlpaddr trace) {
    final Bytes rawAddressHi = chunk.rawHash().slice(0, LLARGE);
    final long depAddressHi = rawAddressHi.slice(12, 4).toLong();
    final Bytes depAddressLo = chunk.rawHash().slice(LLARGE, LLARGE);
    final BigInteger nonce = chunk.nonce();
    final Bytes nonceBytes = bigIntegerToBytes(nonce);

    for (int ct = 0; ct <= MAX_CT_CREATE2; ct++) {
      trace
          .stamp(stamp)
          .recipe(UnsignedByte.of(RLP_ADDR_RECIPE_2))
          .recipe1(false)
          .recipe2(true)
          .rawAddrHi(rawAddressHi)
          .depAddrHi(depAddressHi)
          .depAddrLo(depAddressLo)
          .addrHi(chunk.address().getBytes().slice(0, 4).toLong())
          .addrLo(chunk.address().getBytes().slice(4, LLARGE))
          .saltHi(chunk.salt().slice(0, LLARGE))
          .saltLo(chunk.salt().slice(LLARGE, LLARGE))
          .kecHi(chunk.keccak().slice(0, LLARGE))
          .kecLo(chunk.keccak().slice(LLARGE, LLARGE))
          .nonce(nonceBytes)
          .lc(true)
          .index(UnsignedByte.of(ct))
          .counter(UnsignedByte.of(ct));

      switch (ct) {
        case 0 -> {
          trace.limb(
              rightPadTo(
                  Bytes.concatenate(CREATE2_SHIFT, chunk.address().getBytes().slice(0, 4)),
                  LLARGE));
          trace.nBytes(UnsignedByte.of(5)).selectorKeccakRes(true);
        }
        case 1 ->
            trace
                .limb(chunk.address().getBytes().slice(4, LLARGE))
                .nBytes(BYTES_LLARGE)
                .selectorKeccakRes(false);
        case 2 ->
            trace.limb(chunk.salt().slice(0, LLARGE)).nBytes(BYTES_LLARGE).selectorKeccakRes(false);
        case 3 ->
            trace
                .limb(chunk.salt().slice(LLARGE, LLARGE))
                .nBytes(BYTES_LLARGE)
                .selectorKeccakRes(false);
        case 4 ->
            trace
                .limb(chunk.keccak().slice(0, LLARGE))
                .nBytes(BYTES_LLARGE)
                .selectorKeccakRes(false);
        case 5 ->
            trace
                .limb(chunk.keccak().slice(LLARGE, LLARGE))
                .nBytes(BYTES_LLARGE)
                .selectorKeccakRes(false);
      }

      // Columns unused for Recipe2
      trace.fillAndValidateRow();
    }
  }

  private void traceCreate(int stamp, RlpAddrOperation chunk, Trace.Rlpaddr trace) {
    final BigInteger nonce = chunk.nonce();

    final Bytes nonceShifted = leftPadTo(bigIntegerToBytes(nonce), recipe1NbRows);
    boolean tinyNonZeroNonce = true;
    if (nonce.compareTo(BigInteger.ZERO) == 0 || nonce.compareTo(BigInteger.valueOf(128)) >= 0) {
      tinyNonZeroNonce = false;
    }
    // Compute the BYTESIZE and POWER columns
    int nonceByteSize = bigIntegerToBytes(nonce).size();
    if (nonce.equals(BigInteger.ZERO)) {
      nonceByteSize = 0;
    }
    ByteCountAndPowerOutput byteCounting = byteCounting(nonceByteSize, recipe1NbRows);

    // Compute the bit decomposition of the last input's byte
    final byte lastByte = nonceShifted.get(recipe1NbRows - 1);
    BitDecOutput bitDecomposition = bitDecomposition(0xff & lastByte, recipe1NbRows);

    int size_rlp_nonce = nonceByteSize;
    if (!tinyNonZeroNonce) {
      size_rlp_nonce += 1;
    }

    // Bytes RLP(nonce)
    Bytes rlpNonce;
    if (nonce.compareTo(BigInteger.ZERO) == 0) {
      rlpNonce = BYTES_PREFIX_SHORT_INT;
    } else {
      if (tinyNonZeroNonce) {
        rlpNonce = bigIntegerToBytes(nonce);
      } else {
        rlpNonce =
            Bytes.concatenate(
                bigIntegerToBytes(
                    BigInteger.valueOf(
                        128 + byteCounting.accByteSizeList().get(recipe1NbRows - 1))),
                bigIntegerToBytes(nonce));
      }
    }

    final Bytes rawAddressHi = chunk.rawHash().slice(0, LLARGE);
    final long depAddressHi = rawAddressHi.slice(12, 4).toLong();
    final Bytes depAddressLo = chunk.rawHash().slice(LLARGE, LLARGE);
    final Bytes nonceBytes = bigIntegerToBytes(nonce);

    for (int ct = 0; ct < recipe1NbRows; ct++) {
      trace
          .stamp(stamp)
          .recipe(UnsignedByte.of(RLP_ADDR_RECIPE_1))
          .recipe1(true)
          .recipe2(false)
          .addrHi(chunk.address().getBytes().slice(0, 4).toLong())
          .addrLo(chunk.address().getBytes().slice(4, LLARGE))
          .rawAddrHi(rawAddressHi)
          .depAddrHi(depAddressHi)
          .depAddrLo(depAddressLo)
          .nonce(nonceBytes)
          .counter(UnsignedByte.of(ct))
          .byte1(UnsignedByte.of(nonceShifted.get(ct)))
          .acc(nonceShifted.slice(0, ct + 1))
          .accBytesize(UnsignedByte.of(byteCounting.accByteSizeList().get(ct)))
          .power(
              bigIntegerToBytes(byteCounting.powerList().get(ct).divide(BigInteger.valueOf(256))))
          .bit1(bitDecomposition.bitDecList().get(ct))
          .bitAcc(UnsignedByte.of(bitDecomposition.bitAccList().get(ct)))
          .tinyNonZeroNonce(tinyNonZeroNonce);

      switch (ct) {
        case 0, 1, 2, 3 ->
            trace
                .lc(false)
                .limb(Bytes.EMPTY)
                .nBytes(UnsignedByte.ZERO)
                .index(UnsignedByte.ZERO)
                .selectorKeccakRes(ct == 0);
        case 4 ->
            trace
                .lc(true)
                .limb(
                    rightPadTo(
                        bigIntegerToBytes(
                            BigInteger.valueOf(RLP_PREFIX_LIST_SHORT)
                                .add(BigInteger.valueOf(21))
                                .add(BigInteger.valueOf(size_rlp_nonce))),
                        LLARGE))
                .nBytes(UnsignedByte.of(1))
                .index(UnsignedByte.ZERO)
                .selectorKeccakRes(false);
        case 5 ->
            trace
                .lc(true)
                .limb(
                    rightPadTo(
                        Bytes.concatenate(
                            bigIntegerToBytes(BigInteger.valueOf(148)),
                            chunk.address().getBytes().slice(0, 4)),
                        LLARGE))
                .nBytes(UnsignedByte.of(5))
                .index(UnsignedByte.of(1))
                .selectorKeccakRes(false);
        case 6 ->
            trace
                .lc(true)
                .limb(chunk.address().getBytes().slice(4, LLARGE))
                .nBytes(UnsignedByte.of(LLARGE))
                .index(UnsignedByte.of(2))
                .selectorKeccakRes(false);
        case 7 ->
            trace
                .lc(true)
                .limb(rightPadTo(rlpNonce, LLARGE))
                .nBytes(UnsignedByte.of(size_rlp_nonce))
                .index(UnsignedByte.of(3))
                .selectorKeccakRes(false);
      }

      // Column not used fo recipe 1:
      trace.fillAndValidateRow();
    }
  }

  private void traceOperation(RlpAddrOperation operation, int stamp, Trace.Rlpaddr trace) {
    if (operation.opCode().equals(OpCode.CREATE)) {
      traceCreate(stamp, operation, trace);
    } else {
      traceCreate2(stamp, operation, trace);
    }
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.rlpaddr().headers(this.lineCount());
  }

  @Override
  public int spillage(Trace trace) {
    return trace.rlpaddr().spillage();
  }

  @Override
  public void commit(Trace trace) {
    int stamp = 0;
    for (RlpAddrOperation op : operations.sortOperations(new RlpAddrOperationComparator())) {
      traceOperation(op, ++stamp, trace.rlpaddr());
    }
  }
}

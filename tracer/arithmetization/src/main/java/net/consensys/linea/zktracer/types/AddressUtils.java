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

package net.consensys.linea.zktracer.types;

import static com.google.common.base.Preconditions.*;
import static net.consensys.linea.zktracer.Trace.CREATE2_SHIFT;
import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.types.Utils.leftPadTo;
import static org.hyperledger.besu.crypto.Hash.keccak256;
import static org.hyperledger.besu.datatypes.Address.*;
import static org.hyperledger.besu.evm.internal.Words.clampedToLong;

import java.util.List;
import java.util.stream.Stream;
import net.consensys.linea.zktracer.Fork;
import net.consensys.linea.zktracer.module.hub.transients.OperationAncillaries;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.crypto.Hash;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.ethereum.rlp.RLP;
import org.hyperledger.besu.evm.frame.MessageFrame;

public class AddressUtils {

  private static final Bytes CREATE2_PREFIX = Bytes.minimalBytes(CREATE2_SHIFT);

  public static final List<Address> precompileAddressLondon =
      List.of(
          ECREC,
          SHA256,
          RIPEMD160,
          Address.ID,
          Address.MODEXP,
          Address.ALTBN128_ADD,
          Address.ALTBN128_MUL,
          Address.ALTBN128_PAIRING,
          Address.BLAKE2B_F_COMPRESSION);

  public static final List<Address> precompileAddressCancun =
      Stream.concat(precompileAddressLondon.stream(), Stream.of(Address.KZG_POINT_EVAL)).toList();

  public static final List<Address> BLS_PRECOMPILES =
      List.of(
          Address.BLS12_G1ADD,
          Address.BLS12_G1MULTIEXP,
          Address.BLS12_G2ADD,
          Address.BLS12_G2MULTIEXP,
          Address.BLS12_PAIRING,
          Address.BLS12_MAP_FP_TO_G1,
          Address.BLS12_MAP_FP2_TO_G2);

  public static final List<Address> precompileAddressPrague =
      Stream.concat(precompileAddressCancun.stream(), BLS_PRECOMPILES.stream()).toList();

  public static final List<Address> precompileAddressOsaka =
      Stream.concat(precompileAddressPrague.stream(), Stream.of(P256_VERIFY)).toList();

  /**
   * Check if the address is one of the BLS precompiles added in Prague (so excluding
   * KZG_POINT_EVAL).
   */
  public static boolean isBlsPrecompile(Address address) {
    return BLS_PRECOMPILES.contains(address);
  }

  public static boolean isPrecompile(Fork fork, Address to) {
    return switch (fork) {
      case OSAKA -> precompileAddressOsaka.contains(to);
      case LONDON, PARIS, SHANGHAI, CANCUN, PRAGUE ->
          throw new IllegalArgumentException("Fork no more supported by the tracer: " + fork);
      default -> throw new IllegalArgumentException("Unknown fork: " + fork);
    };
  }

  /**
   * Compute the effective address of a transaction target, i.e. the specified target if explicitly
   * set, or the to-be-deployed address otherwise.
   *
   * @return the effective target address of tx
   */
  public static Address effectiveToAddress(Transaction tx) {
    return tx.getTo()
        .map(x -> (Address) x)
        .orElse(Address.contractAddress(tx.getSender(), tx.getNonce()));
  }

  /* Warning: this method uses the nonce as currently found in the state
  however, CREATE raises the nonce and so this method should only be called
  pre OpCode and pre transaction for deployment */
  public static Address getCreateAddress(final MessageFrame frame) {
    return Address.extract(getCreateRawAddress(frame));
  }

  public static Bytes32 getCreateRawAddress(final MessageFrame frame) {
    final Address address = frame.getRecipientAddress();
    final long nonce = frame.getWorldUpdater().get(address).getNonce();
    return getCreateRawAddress(address, nonce);
  }

  public static Bytes32 getCreateRawAddress(final Address senderAddress, final long nonce) {
    return org.hyperledger.besu.crypto.Hash.keccak256(
        RLP.encode(
            (out) -> {
              out.startList();
              out.writeBytes(senderAddress);
              out.writeLongScalar(nonce);
              out.endList();
            }));
  }

  public static Bytes32 gerCreate2RawAddress(final MessageFrame frame) {
    final Address sender = frame.getRecipientAddress();

    final Bytes32 salt = Bytes32.leftPad(frame.getStackItem(3));

    final long offset = clampedToLong(frame.getStackItem(1));
    final long length = clampedToLong(frame.getStackItem(2));
    final Bytes initCode = frame.shadowReadMemory(offset, length);
    final Bytes32 hash = keccak256(initCode);

    return getCreate2RawAddress(sender, salt, hash);
  }

  public static Bytes32 getCreate2RawAddress(
      final Address sender, final Bytes32 salt, final Bytes32 hash) {
    return Hash.keccak256(Bytes.concatenate(CREATE2_PREFIX, sender, salt, hash));
  }

  public static Address getCreate2Address(final MessageFrame frame, OpCodeData opCode) {
    final Address sender = frame.getRecipientAddress();
    final Bytes32 salt = Bytes32.leftPad(frame.getStackItem(3));
    final Bytes initCode = OperationAncillaries.initCode(frame, opCode);
    final Bytes32 hash = Hash.keccak256(initCode);
    return Address.extract(getCreate2RawAddress(sender, salt, hash));
  }

  public static Address getDeploymentAddress(final MessageFrame frame, final OpCodeData opCode) {
    checkArgument(opCode.isCreate(), "Must be called only for CREATE/CREATE2 opcode");
    return opCode.mnemonic() == OpCode.CREATE
        ? getCreateAddress(frame)
        : getCreate2Address(frame, opCode);
  }

  public static Address addressFromBytes(final Bytes input) {
    return input.size() == Address.SIZE
        ? Address.wrap(input)
        : Address.wrap(leftPadTo(input.trimLeadingZeros(), Address.SIZE));
  }

  public static long hiPart(Address address) {
    return address.slice(0, 4).toLong();
  }

  public static Bytes loPart(Address address) {
    return address.slice(4, LLARGE);
  }

  public static boolean isAddressWarm(
      final Fork fork, final MessageFrame messageFrame, final Address address) {
    return messageFrame.isAddressWarm(address) || isPrecompile(fork, address);
  }
}

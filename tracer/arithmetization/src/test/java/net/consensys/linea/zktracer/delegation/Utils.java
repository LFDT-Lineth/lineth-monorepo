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
package net.consensys.linea.zktracer.delegation;

import static net.consensys.linea.zktracer.Trace.EIP_7702_DELEGATION_INDICATOR;
import static net.consensys.linea.zktracer.opcode.OpCode.*;

import java.math.BigInteger;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyTransaction;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.TransactionType;
import org.hyperledger.besu.datatypes.Wei;

public class Utils extends TracerTestBase {

  // sender account
  public static final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
  public static final Address senderAddress =
      Address.extract(Hash.hash(senderKeyPair.getPublicKey().getEncodedBytes()));
  public static int senderNonce = 0x77;

  public static final ToyAccount senderAccount() {
    return ToyAccount.builder()
        .balance(Wei.fromEth(56))
        .nonce(senderNonce)
        .address(senderAddress)
        .build();
  }

  // SMC
  public static final Address smcAddress =
      Address.fromHexString("0x1122334455667788990011223344556677889900");

  public static final ToyAccount smcAccount() {
    return ToyAccount.builder().balance(Wei.fromEth(22)).nonce(3).address(smcAddress).build();
  }

  public static final ToyTransaction.ToyTransactionBuilder tx() {
    final ToyAccount smcAccount = smcAccount();

    return ToyTransaction.builder()
        .sender(senderAccount())
        .to(smcAccount)
        .keyPair(senderKeyPair)
        .gasLimit(300_000L)
        .transactionType(TransactionType.DELEGATE_CODE)
        .value(Wei.of(1000));
  }

  // authority
  public static final KeyPair authorityKeyPair = new SECP256K1().generateKeyPair();
  public static final Address authorityAddress =
      Address.extract(Hash.hash(authorityKeyPair.getPublicKey().getEncodedBytes()));
  public static final long authNonce = 0xfee1beefL;

  public static final ToyAccount authorityAccount() {

    return ToyAccount.builder()
        .balance(Wei.fromEth(2))
        .nonce(authNonce)
        .address(authorityAddress)
        .build();
  }

  // delegation address
  public static final Address delegationAddress = Address.fromHexString("0x0de1e9");

  enum ChainIdValidity {
    DELEGATION_CHAIN_ID_IS_ZERO,
    DELEGATION_CHAIN_ID_IS_NETWORK_CHAIN_ID,
    DELEGATION_CHAIN_ID_IS_INVALID,
    ;

    public final BigInteger tupleChainId() {
      return switch (this) {
        case DELEGATION_CHAIN_ID_IS_ZERO -> BigInteger.ZERO;
        case DELEGATION_CHAIN_ID_IS_NETWORK_CHAIN_ID -> chainConfig.id;
        case DELEGATION_CHAIN_ID_IS_INVALID ->
            Bytes.fromHexString("0x17891789178917891789178917891789178917891789178900000000")
                .toUnsignedBigInteger();
      };
    }
  }

  /**
   * Used to flip the <b>s</b> of a valid signature to its complement with respect to the curve
   * order, and thus to produce another valid signature with the same <b>r</b> and a different
   * <b>s</b> which breaks the <b>non-malleability bound</b> property.
   */
  public enum SFlipping {
    NOT_FLIPPED,
    FLIPPED,
  }

  /**
   * Used to modify the <b>s</b> of a valid signature by a small amount. This tampering must
   * preserve the <b>non-malleability bound</b> property of the initial <b>s</b>.
   */
  public enum STampering {
    NOT_TAMPERED_WITH,
    TAMPERED_WITH
  }

  enum AuthorityExistence {
    AUTHORITY_DOES_NOT_EXIST,
    AUTHORITY_EXISTS;

    public final long tupleNonce() {
      return switch (this) {
        case AUTHORITY_DOES_NOT_EXIST -> 0L;
        case AUTHORITY_EXISTS -> authNonce;
      };
    }
  }

  enum RequiresEvmExecution {
    REQUIRES_EVM_EXECUTION,
    DOES_NOT_REQUIRE_EVM_EXECUTION
  }

  enum TransactionReverts {
    TRANSACTION_REVERTS,
    TRANSACTION_DOES_NOT_REVERT
  }

  enum OtherRefunds {
    OTHER_REFUNDS,
    NO_OTHER_REFUNDS
  }

  enum TouchAuthority {
    EXECUTION_DOES_NOT_TOUCH_AUTHORITY,
    EXECUTION_TOUCHES_AUTHORITY
  }

  enum TouchMethod {
    EXTCODESIZE,
    EXTCODEHASH,
    BALANCE
  }

  enum AuthorityInAccessList {
    AUTHORITY_NOT_IN_ACCESS_LIST,
    AUTHORITY_IN_ACCESS_LIST
  }

  public enum DelegationSuccess {
    DELEGATION_SUCCESS,
    DELEGATION_FAILURE
  }

  enum AuthorityDelegationStatus {
    AUTHORITY_DOES_NOT_EXIST,
    AUTHORITY_EXISTS_NOT_DELEGATED,
    AUTHORITY_DELEGATED_TO_EMPTY_CODE_EOA,
    AUTHORITY_DELEGATED_TO_SMC,
    AUTHORITY_DELEGATED_TO_SMC_ALT,
    AUTHORITY_DELEGATED_TO_DELEGATED,
    AUTHORITY_DELEGATED_TO_PRC,
    ;

    public boolean authorityMustExist() {
      return this == AUTHORITY_EXISTS_NOT_DELEGATED
          || this == AUTHORITY_DELEGATED_TO_EMPTY_CODE_EOA
          || this == AUTHORITY_DELEGATED_TO_SMC
          || this == AUTHORITY_DELEGATED_TO_SMC_ALT
          || this == AUTHORITY_DELEGATED_TO_DELEGATED
          || this == AUTHORITY_DELEGATED_TO_PRC;
    }

    public boolean requiresEvmExecution() {
      return switch (this) {
        case AUTHORITY_DELEGATED_TO_SMC,
            AUTHORITY_DELEGATED_TO_SMC_ALT,
            AUTHORITY_DELEGATED_TO_DELEGATED ->
            true;
        case AUTHORITY_DELEGATED_TO_EMPTY_CODE_EOA,
            AUTHORITY_DELEGATED_TO_PRC,
            AUTHORITY_DOES_NOT_EXIST,
            AUTHORITY_EXISTS_NOT_DELEGATED ->
            false;
      };
    }
  }

  static BytecodeCompiler codeThatMayTouchAuthority(
      RequiresEvmExecution requiresEvmExecution,
      TouchAuthority touchAuthority,
      TouchMethod touchMethod) {
    final BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    if (requiresEvmExecution == Utils.RequiresEvmExecution.DOES_NOT_REQUIRE_EVM_EXECUTION) {
      return program;
    }

    if (touchAuthority == TouchAuthority.EXECUTION_DOES_NOT_TOUCH_AUTHORITY) {
      return program.op(JUMPDEST);
    }

    if (touchAuthority == TouchAuthority.EXECUTION_TOUCHES_AUTHORITY) {
      switch (touchMethod) {
        case EXTCODESIZE -> program.push(authorityAddress).op(OpCode.EXTCODESIZE).op(OpCode.POP);
        case EXTCODEHASH -> program.push(authorityAddress).op(OpCode.EXTCODEHASH).op(OpCode.POP);
        case BALANCE -> program.push(authorityAddress).op(OpCode.BALANCE).op(OpCode.POP);
      }
    }

    return program;
  }

  static BytecodeCompiler codeThatMayAccrueRefundsAndMayRevert(
      Utils.RequiresEvmExecution requiresEvmExecution,
      Utils.OtherRefunds executionAccruesOtherRefunds,
      Utils.TransactionReverts transactionReverts) {
    final BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    if (requiresEvmExecution == Utils.RequiresEvmExecution.DOES_NOT_REQUIRE_EVM_EXECUTION) {
      return program;
    }

    switch (executionAccruesOtherRefunds) {
      case NO_OTHER_REFUNDS -> program.push(1).push(2).push(3).op(OpCode.ADDMOD).op(OpCode.POP);
      case OTHER_REFUNDS ->
          program
              .push(0xc0ffee)
              .push(0x5107) // 0x 5107 <> slot
              .op(OpCode.SSTORE) // write nontrivial value
              .push(0)
              .push(0x5107)
              .op(OpCode.SSTORE); // incur refund (reset to zero)
    }

    if (transactionReverts == Utils.TransactionReverts.TRANSACTION_REVERTS) {
      program.push(0).push(0).op(OpCode.REVERT);
    }

    return program;
  }

  public static Bytes pseudoDelegationCode(String hexString) {
    return Bytes.fromHexString(Integer.toHexString(EIP_7702_DELEGATION_INDICATOR) + hexString);
  }

  public static Bytes delegationCodeFromAddress(Address address) {
    return Bytes.fromHexString(
        Integer.toHexString(EIP_7702_DELEGATION_INDICATOR) + address.toHexString().substring(2));
  }
}

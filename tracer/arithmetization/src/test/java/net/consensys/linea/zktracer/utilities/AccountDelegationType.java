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

package net.consensys.linea.zktracer.utilities;

import static net.consensys.linea.zktracer.utilities.Utils.addDelegationPrefixToAccount;
import static net.consensys.linea.zktracer.utilities.Utils.addDelegationPrefixToAddress;

import net.consensys.linea.testing.ToyAccount;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.Wei;

public enum AccountDelegationType {
  NO_DELEGATION,
  DELEGATED_TO_SMC,
  DELEGATED_TO_EOA,
  DELEGATED_TO_ITSELF,
  DELEGATED_TO_EOA_DELEGATED_TO_SMC,
  DELEGATED_TO_PRC;

  static final String smcBytecode = "0x30473833345a";
  static final ToyAccount smcAccount =
      ToyAccount.builder()
          .address(Address.fromHexString("1234"))
          .nonce(90)
          .code(Bytes.concatenate(Bytes.fromHexString(smcBytecode)))
          .build();
  static final String delegationCodeToSmc = addDelegationPrefixToAccount(smcAccount);

  public static ToyAccount getAccountForDelegationType(final AccountDelegationType delegationType) {
    KeyPair keyPair = new SECP256K1().generateKeyPair();
    return getAccountForDelegationTypeWithKeyPair(keyPair, delegationType);
  }

  public static ToyAccount getAccountForDelegationTypeWithKeyPair(
      KeyPair keyPair, final AccountDelegationType delegationType) {
    Address accountAddress = Address.extract(Hash.hash(keyPair.getPublicKey().getEncodedBytes()));
    ToyAccount.ToyAccountBuilder eoaBuilder =
        ToyAccount.builder().address(accountAddress).balance(Wei.fromEth(123)).nonce(12);
    switch (delegationType) {
      case NO_DELEGATION:
        return eoaBuilder.build();
      case DELEGATED_TO_SMC:
        return eoaBuilder.code(Bytes.concatenate(Bytes.fromHexString(delegationCodeToSmc))).build();
      case DELEGATED_TO_EOA:
        ToyAccount eoa2 =
            ToyAccount.builder()
                .address(Address.fromHexString("ca11ee2")) // identity caller
                .nonce(80)
                .build();
        String delegationCodeToEoa2 = addDelegationPrefixToAccount(eoa2);
        return eoaBuilder
            .code(Bytes.concatenate(Bytes.fromHexString(delegationCodeToEoa2)))
            .build();
      case DELEGATED_TO_ITSELF:
        String delegationCodeToEoa = addDelegationPrefixToAddress(eoaBuilder.build().getAddress());
        return eoaBuilder.code(Bytes.concatenate(Bytes.fromHexString(delegationCodeToEoa))).build();
      case DELEGATED_TO_EOA_DELEGATED_TO_SMC:
        ToyAccount eoa2DelegatedToSmc =
            ToyAccount.builder()
                .address(Address.fromHexString("ca11ee2")) // identity caller
                .nonce(80)
                .code(Bytes.concatenate(Bytes.fromHexString(delegationCodeToSmc)))
                .build();
        String delegationCodeToEoa2DelegatedToSmc =
            addDelegationPrefixToAccount(eoa2DelegatedToSmc);
        return eoaBuilder
            .code(Bytes.concatenate(Bytes.fromHexString(delegationCodeToEoa2DelegatedToSmc)))
            .build();
      case DELEGATED_TO_PRC:
        String delegationCodeToP256Verify = addDelegationPrefixToAddress(Address.P256_VERIFY);
        return eoaBuilder
            .code(Bytes.concatenate(Bytes.fromHexString(delegationCodeToP256Verify)))
            .build();
      default:
        throw new IllegalArgumentException("Unknown delegation type: " + delegationType);
    }
  }
}

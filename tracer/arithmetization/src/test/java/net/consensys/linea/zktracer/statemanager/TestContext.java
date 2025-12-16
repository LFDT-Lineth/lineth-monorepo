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
package net.consensys.linea.zktracer.statemanager;

import java.math.BigInteger;
import java.util.Arrays;
import java.util.Collections;
import java.util.List;
import lombok.Getter;
import lombok.NoArgsConstructor;
import net.consensys.linea.testing.*;
import net.consensys.linea.testing.generated.FrameworkEntrypoint;
import net.consensys.linea.zktracer.types.AddressUtils;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.web3j.abi.FunctionEncoder;
import org.web3j.abi.datatypes.DynamicArray;
import org.web3j.abi.datatypes.Function;
import org.web3j.abi.datatypes.generated.Uint256;

@NoArgsConstructor
public class TestContext {
  Long txNonce = null;
  static final Long gasLimit = 5000000L;
  static final Wei defaultBalance = Wei.of(4500L);
  static final int numberOfAccounts = 10;
  static final int numberOfNewAccounts = 10;
  static final int numberOfEOA = 1;
  static final Bytes snippetsCode =
      SmartContractUtils.getYulContractRuntimeByteCode("StateManagerSnippets.yul");
  static final Bytes snippetsCodeForCreate2 =
      SmartContractUtils.getYulContractCompiledByteCode("StateManagerSnippets.yul");
  @Getter ToyAccount frameworkEntryPointAccount;
  Address frameworkEntryPointAddress;
  ToyAccount[] initialAccounts;
  ToyAccount[] externallyOwnedAccounts;
  KeyPair[] keyPairs;
  Address[] addresses;
  Address[] newAddresses;
  String[] salts = new String[numberOfNewAccounts];

  public void initializeTestContext() {
    // initialize vectors
    initialAccounts = new ToyAccount[numberOfAccounts];
    addresses = new Address[numberOfAccounts];
    externallyOwnedAccounts = new ToyAccount[numberOfEOA];
    keyPairs = new KeyPair[numberOfEOA];
    newAddresses = new Address[numberOfNewAccounts];
    // generate externally owned accounts
    for (int i = 0; i < numberOfEOA; i++) {
      KeyPair keyPair = new SECP256K1().generateKeyPair();
      Address senderAddress = Address.extract(Hash.hash(keyPair.getPublicKey().getEncodedBytes()));
      externallyOwnedAccounts[i] =
          ToyAccount.builder().balance(Wei.fromEth(1)).nonce(5).address(senderAddress).build();
      keyPairs[i] = keyPair;
    }
    // initialize the testing framework entry point account
    frameworkEntryPointAccount =
        ToyAccount.builder()
            .address(Address.fromHexString("0x123456"))
            .balance(defaultBalance)
            .nonce(5)
            .code(SmartContractUtils.getSolidityContractRuntimeByteCode(FrameworkEntrypoint.class))
            .build();
    frameworkEntryPointAddress = frameworkEntryPointAccount.getAddress();
    // add to arrays
    // initialize the .yul snippets account
    // load the .yul bytecode
    for (int i = 0; i < numberOfAccounts; i++) {

      initialAccounts[i] =
          ToyAccount.builder()
              .address(Address.fromHexString("0x" + i + i + i + i + i))
              .balance(defaultBalance)
              .nonce(6)
              .code(TestContext.snippetsCode)
              .build();
      addresses[i] = initialAccounts[i].getAddress();
    }

    // initialize the new addresses vector
    for (int i = 0; i < numberOfAccounts; i++) {
      salts[i] = "0x000000000000000000000000000000000000000000000000000000000000000" + i;
      newAddresses[i] = getCreate2AddressForSnippet(salts[i]);
    }
  }

  // destination must be our .yul smart contract
  Transaction newTxFromCalls(
      ToyAccount sender, KeyPair senderKeyPair, FrameworkEntrypoint.ContractCall[] contractCalls) {
    Function frameworkEntryPointFunction =
        new Function(
            FrameworkEntrypoint.FUNC_EXECUTECALLS,
            List.of(new DynamicArray<>(FrameworkEntrypoint.ContractCall.class, contractCalls)),
            Collections.emptyList());
    Bytes txPayload =
        Bytes.fromHexStringLenient(FunctionEncoder.encode(frameworkEntryPointFunction));

    ToyTransaction.ToyTransactionBuilder tempTx =
        ToyTransaction.builder()
            .sender(sender)
            .to(this.frameworkEntryPointAccount)
            .payload(txPayload)
            .keyPair(senderKeyPair)
            .gasLimit(TestContext.gasLimit);

    if (this.txNonce != null) {
      tempTx = tempTx.nonce(++this.txNonce);
    }
    Transaction tx = tempTx.build();
    if (this.txNonce == null) {
      this.txNonce = tx.getNonce();
    }
    return tx;
  }

  // destination must be our .yul smart contract
  FrameworkEntrypoint.ContractCall writeToStorageCall(
      Address destination, Long key, Long value, boolean revertFlag, BigInteger callType) {
    Function yulFunction =
        new Function(
            "writeToStorage",
            Arrays.asList(
                new Uint256(BigInteger.valueOf(key)),
                new Uint256(BigInteger.valueOf(value)),
                new org.web3j.abi.datatypes.Bool(revertFlag)),
            Collections.emptyList());

    var encoding = FunctionEncoder.encode(yulFunction);
    FrameworkEntrypoint.ContractCall snippetContractCall =
        new FrameworkEntrypoint.ContractCall(
            /*Address*/ destination.toHexString(),
            /*calldata*/ Bytes.fromHexStringLenient(encoding).toArray(),
            /*gasLimit*/ BigInteger.ZERO,
            /*value*/ BigInteger.ZERO,
            /*callType*/ callType);
    return snippetContractCall;
  }

  // destination must be our .yul smart contract
  Transaction writeToStorage(
      ToyAccount sender,
      KeyPair senderKeyPair,
      Address destination,
      Long key,
      Long value,
      boolean revertFlag,
      BigInteger callType) {
    FrameworkEntrypoint.ContractCall snippetContractCall =
        writeToStorageCall(destination, key, value, revertFlag, callType);
    return newTxFromCalls(
        sender, senderKeyPair, new FrameworkEntrypoint.ContractCall[] {snippetContractCall});
  }

  FrameworkEntrypoint.ContractCall readFromStorageCall(
      ToyAccount sender,
      KeyPair senderKeyPair,
      Address destination,
      Long key,
      boolean revertFlag,
      BigInteger callType) {
    Function yulFunction =
        new Function(
            "readFromStorage",
            Arrays.asList(
                new Uint256(BigInteger.valueOf(key)), new org.web3j.abi.datatypes.Bool(revertFlag)),
            Collections.emptyList());

    var encoding = FunctionEncoder.encode(yulFunction);
    FrameworkEntrypoint.ContractCall snippetContractCall =
        new FrameworkEntrypoint.ContractCall(
            /*Address*/ destination.toHexString(),
            /*calldata*/ Bytes.fromHexStringLenient(encoding).toArray(),
            /*gasLimit*/ BigInteger.ZERO,
            /*value*/ BigInteger.ZERO,
            /*callType*/ callType);

    return snippetContractCall;
  }

  // destination must be our .yul smart contract
  Transaction readFromStorage(
      ToyAccount sender,
      KeyPair senderKeyPair,
      Address destination,
      Long key,
      boolean revertFlag,
      BigInteger callType) {
    FrameworkEntrypoint.ContractCall snippetContractCall =
        readFromStorageCall(sender, senderKeyPair, destination, key, revertFlag, callType);
    return newTxFromCalls(
        sender, senderKeyPair, new FrameworkEntrypoint.ContractCall[] {snippetContractCall});
  }

  // destination must be our .yul smart contract
  Transaction selfDestruct(
      ToyAccount sender,
      KeyPair senderKeyPair,
      Address destination,
      Address recipient,
      boolean revertFlag,
      BigInteger callType) {
    FrameworkEntrypoint.ContractCall snippetContractCall =
        selfDestructCall(destination, recipient, revertFlag, callType);
    return newTxFromCalls(
        sender, senderKeyPair, new FrameworkEntrypoint.ContractCall[] {snippetContractCall});
  }

  // destination must be our .yul smart contract
  FrameworkEntrypoint.ContractCall selfDestructCall(
      Address destination, Address recipient, boolean revertFlag, BigInteger callType) {
    String recipientAddressString = recipient.toHexString();
    Function yulFunction =
        new Function(
            "selfDestruct",
            Arrays.asList(
                new org.web3j.abi.datatypes.Address(recipientAddressString),
                new org.web3j.abi.datatypes.Bool(revertFlag)),
            Collections.emptyList());

    var encoding = FunctionEncoder.encode(yulFunction);
    FrameworkEntrypoint.ContractCall snippetContractCall =
        new FrameworkEntrypoint.ContractCall(
            /*Address*/ destination.toHexString(),
            /*calldata*/ Bytes.fromHexStringLenient(encoding).toArray(),
            /*gasLimit*/ BigInteger.ZERO,
            /*value*/ BigInteger.ZERO,
            /*callType*/ callType); // Normal call, not a delegate call as would be the default

    return snippetContractCall;
  }

  // destination must be our .yul smart contract
  Transaction transferTo(
      ToyAccount sender,
      KeyPair senderKeyPair,
      Address destination,
      Address recipient,
      long amount,
      boolean revertFlag,
      BigInteger callType) {
    FrameworkEntrypoint.ContractCall snippetContractCall =
        transferToCall(destination, recipient, amount, revertFlag, callType);
    return newTxFromCalls(
        sender, senderKeyPair, new FrameworkEntrypoint.ContractCall[] {snippetContractCall});
  }

  FrameworkEntrypoint.ContractCall transferToCall(
      Address destination,
      Address recipient,
      long amount,
      boolean revertFlag,
      BigInteger callType) {
    String recipientAddressString = recipient.toHexString();
    Function yulFunction =
        new Function(
            "transferTo",
            Arrays.asList(
                new org.web3j.abi.datatypes.Address(recipientAddressString),
                new Uint256(amount),
                new org.web3j.abi.datatypes.Bool(revertFlag)),
            Collections.emptyList());

    var encoding = FunctionEncoder.encode(yulFunction);
    FrameworkEntrypoint.ContractCall snippetContractCall =
        new FrameworkEntrypoint.ContractCall(
            /*Address*/ destination.toHexString(),
            /*calldata*/ Bytes.fromHexStringLenient(encoding).toArray(),
            /*gasLimit*/ BigInteger.ZERO,
            /*value*/ BigInteger.ZERO,
            /*callType*/ callType); // Normal call, not a delegate call as would be the default
    return snippetContractCall;
  }

  // destination must be our .yul smart contract
  Transaction deployWithCreate2_withRevertTrigger(
      ToyAccount sender,
      KeyPair senderKeyPair,
      Address destination,
      String saltString,
      Bytes contractBytes,
      boolean endWithRevert) {
    FrameworkEntrypoint.ContractCall snippetContractCall =
        deployWithCreate2_withRevertTriggerCall(
            destination, saltString, contractBytes, endWithRevert);
    return newTxFromCalls(
        sender, senderKeyPair, new FrameworkEntrypoint.ContractCall[] {snippetContractCall});
  }

  FrameworkEntrypoint.ContractCall deployWithCreate2_withRevertTriggerCall(
      Address destination, String saltString, Bytes contractBytes, boolean revertFlag) {
    Bytes salt = Bytes.fromHexStringLenient(saltString);
    // the following is the bytecode of the .yul contract
    // Bytes yulContractBytes =
    // Bytes.fromHexStringLenient("61037d61001060003961037d6000f3fe6100076102e1565b63a770741d8114610064576397deb47b81146100715763acf07154811461007d57632d97bf1081146100b45763eba7ff7f81146100e757632b261e94811461012157633ecfd51e811461015b5763ffffffff811461017057600080fd5b61006c610177565b610171565b60005460005260206000f35b6004356024356044356100918183856102c7565b61009b828461019d565b600181036100ac576100ab61034d565b5b505050610171565b6004356024356100c481836102cf565b6100ce81846101da565b600182036100df576100de61034d565b5b505050610171565b6004356024356100f682610253565b600081036101095761010881836102db565b5b6001810361011a5761011961034d565b5b5050610171565b6004356024356044353061013682848661030a565b610141838583610217565b600182036101525761015161034d565b5b50505050610171565b61016361028b565b61016b61037a565b610171565b5b5061037c565b7f0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef600055565b7f577269746528616464726573732c75696e743235362c75696e74323536290000604051818152601e8120308585828460606020a4505050505050565b7f5265616428616464726573732c75696e743235362c75696e7432353629202020604051818152601d8120308585828460606020a4505050505050565b7f50617945544828616464726573732c616464726573732c75696e743235362900604051818152601f81208585858360606020a4505050505050565b7f436f6e747261637444657374726f796564286164647265737329000000000000604051818152601a8120838160606020a250505050565b7f52656345544828616464726573732c75696e743235362900000000000000000060405181815260178120303480828460606020a35050505050565b818155505050565b60008154905092915050565b80ff5050565b60007c010000000000000000000000000000000000000000000000000000000060003504905090565b6040517f3ecfd51e0000000000000000000000000000000000000000000000000000000080825260008060208487875af18061034557600080fd5b505050505050565b7f526576657274696e67000000000000000000000000000000000000000000000060206040518281528181fd5b565b");
    // prepare the Create2 function
    Function create2Function =
        new Function(
            FrameworkEntrypoint.FUNC_DEPLOYWITHCREATE2_WITHREVERTTRIGGER,
            Arrays.asList(
                new org.web3j.abi.datatypes.generated.Bytes32(salt.toArray()),
                new org.web3j.abi.datatypes.DynamicBytes(contractBytes.toArray()),
                new org.web3j.abi.datatypes.Bool(revertFlag)),
            Collections.emptyList());

    String encoding = FunctionEncoder.encode(create2Function);

    FrameworkEntrypoint.ContractCall snippetContractCall =
        new FrameworkEntrypoint.ContractCall(
            /*Address*/ destination.toHexString(),
            /*calldata*/ Bytes.fromHexStringLenient(encoding).toArray(),
            /*gasLimit*/ BigInteger.ZERO,
            /*value*/ BigInteger.ZERO,
            /*callType*/ BigInteger.ONE); // Normal call, not a delegate call as it is the default

    return snippetContractCall;
  }

  public Address getCreate2AddressForSnippet(String salt) {
    org.apache.tuweni.bytes.Bytes32 initCodeHash =
        org.hyperledger.besu.crypto.Hash.keccak256(Bytes.wrap(TestContext.snippetsCodeForCreate2));
    org.apache.tuweni.bytes.Bytes32 targetAddress =
        AddressUtils.getCreate2RawAddress(
            frameworkEntryPointAccount.getAddress(),
            org.apache.tuweni.bytes.Bytes32.wrap(Bytes.fromHexStringLenient(salt).toArray()),
            initCodeHash);
    return Address.extract(targetAddress);
  }
}

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
package net.consensys.linea.zktracer.instructionprocessing.createTests.advanced;

import static com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.MonoOpCodeSmcs.keyPair;
import static org.junit.jupiter.api.Assertions.assertEquals;

import java.util.ArrayList;
import java.util.List;
import net.consensys.linea.testing.*;
import net.consensys.linea.testing.generated.ContractC;
import net.consensys.linea.testing.generated.CustomCreate2;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.Hash;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.web3j.abi.EventEncoder;

public class ScenarioUtils {

  /*
  Deployment params
   */

  // Deployment code from testing/src/main/solidity/ContractC.sol
  // Generated for the London EVM in testing/build/resources/main/solidity/ContractC.json
  // Value =
  // 6080604052600034905060003390506001820361006c578060008084815260200190815260200160002060006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550610196565b60028203610161578073ffffffffffffffffffffffffffffffffffffffff16630161e6d76040518163ffffffff1660e01b8152600401600060405180830381600087803b1580156100bc57600080fd5b505af19250505080156100cd575060015b61015c576100d96101cd565b806308c379a0036100fa57506100ed61026a565b806100f857506100fc565b005b505b3d8060008114610128576040519150601f19603f3d011682016040523d82523d6000602084013e61012d565b606091505b507f95a42732c1514e697b8c2992b6ee317a9bc3c4568e341cdbd58ff87fc4c467cd60405160405180910390a1005b610195565b6003820361017c5761017761019d60201b60201c565b610194565b60048203610193576101926101bb60201b60201c565b5b5b5b5b50506102fa565b60003090508073ffffffffffffffffffffffffffffffffffffffff16ff5b600080fd5b60008160e01c9050919050565b600060033d11156101ec5760046000803e6101e96000516101c0565b90505b90565b6000604051905090565b6000601f19601f8301169050919050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052604160045260246000fd5b610242826101f9565b810181811067ffffffffffffffff821117156102615761026061020a565b5b80604052505050565b600060443d106102f75761027c6101ef565b60043d036004823e80513d602482011167ffffffffffffffff821117156102a45750506102f7565b808201805167ffffffffffffffff8111156102c257505050506102f7565b80602083010160043d0385018111156102df5750505050506102f7565b6102ee82602001850186610239565b82955050505050505b90565b610379806103096000396000f3fe608060405234801561001057600080fd5b50600436106100575760003560e01c80632199ecd71461005c578063545b079a146100665780635e666e4a146100825780636c0c412e146100b2578063983d1ce2146100ce575b600080fd5b6100646100d8565b005b610080600480360381019061007b9190610249565b6100f6565b005b61009c600480360381019061009791906102ac565b610159565b6040516100a991906102e8565b60405180910390f35b6100cc60048036038101906100c79190610303565b61018c565b005b6100d66101e1565b005b60003090508073ffffffffffffffffffffffffffffffffffffffff16ff5b8073ffffffffffffffffffffffffffffffffffffffff16630161e6d76040518163ffffffff1660e01b8152600401600060405180830381600087803b15801561013e57600080fd5b505af1158015610152573d6000803e3d6000fd5b5050505050565b60006020528060005260406000206000915054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b8060008084815260200190815260200160002060006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055505050565b600080fd5b600080fd5b600073ffffffffffffffffffffffffffffffffffffffff82169050919050565b6000610216826101eb565b9050919050565b6102268161020b565b811461023157600080fd5b50565b6000813590506102438161021d565b92915050565b60006020828403121561025f5761025e6101e6565b5b600061026d84828501610234565b91505092915050565b6000819050919050565b61028981610276565b811461029457600080fd5b50565b6000813590506102a681610280565b92915050565b6000602082840312156102c2576102c16101e6565b5b60006102d084828501610297565b91505092915050565b6102e28161020b565b82525050565b60006020820190506102fd60008301846102d9565b92915050565b6000806040838503121561031a576103196101e6565b5b600061032885828601610297565b925050602061033985828601610234565b915050925092905056fea26469706673582212203989edc877fe659634a18fccfea5c11d5bb1f6f616f6cacad1172e8af1c1e83064736f6c634300081a0033
  static final String initCodeC =
      SmartContractUtils.getSolidityContractCompiledByteCode(ContractC.class).toString();
  static final String salt = "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef";
  static final String buildCustomCreate2Address = "789101";

  /*
  Transaction params
   */
  static final Wei defaultBalance = Wei.of(4500L);
  static ToyAccount customCreate2Account =
      ToyAccount.builder()
          .address(Address.fromHexString("0x" + buildCustomCreate2Address))
          .balance(defaultBalance)
          .nonce(1)
          .code(SmartContractUtils.getSolidityContractRuntimeByteCode(CustomCreate2.class))
          .build();

  static final Long gasLimit = 10000000L;

  /*
  Utils for logs validation
  */

  // Compute expected address for ContractC with Create2
  // address = keccak256(0xff + sender_address + salt + keccak256(initialisation_code))[12:]
  public static final String senderAddNoOx =
      customCreate2Account.getAddress().toHexString().substring(2);
  public static final String saltNoOx = salt.substring(2);
  public static final String hashInitCodeCNo0x =
      Hash.keccak256(Bytes.fromHexString(initCodeC)).toHexString().substring(2);
  public static final Bytes expectedContractCAddress =
      Hash.keccak256(Bytes.fromHexString("0xff" + senderAddNoOx + saltNoOx + hashInitCodeCNo0x))
          .slice(12);
  // padding left to fit the 32 bytes log data format
  public static final Bytes expectedContractCAddressLogData =
      Bytes.fromHexString(
          "0x000000000000000000000000" + expectedContractCAddress.toString().substring(2));

  /*
  Payload preparations
   */

  public static final Bytes storeInitCodeC = CustomCreate2Payload.storeInitCodeC(initCodeC);
  public static final Bytes storeSalt = CustomCreate2Payload.storeSalt(salt);

  // For Scenario 1 Unit tests
  public static final Bytes create2FourTimes_noRevert =
      CustomCreate2Payload.create2FourTimes_withRevertTrigger(false);
  public static final Bytes create2FourTimes_withRevert =
      CustomCreate2Payload.create2FourTimes_withRevertTrigger(true);
  public static final Bytes callMyselfWithCreate2FourTimes_withRevert =
      CustomCreate2Payload.callMyself(
          CustomCreate2Payload.create2FourTimes_withRevertTrigger(true), false, 1000000);

  // For Scenario 2 Unit tests
  public static final Bytes create2WithStaticCall =
      CustomCreate2Payload.create2WithStaticCall(false);
  public static final Bytes callMyselfWithCreate2WithStaticCall_nested =
      CustomCreate2Payload.callMyself(
          CustomCreate2Payload.create2WithStaticCall(true), false, 3000000);

  // For Scenario 3 Unit tests
  public static final Bytes create2CallC_noRevert =
      CustomCreate2Payload.create2CallC_withRevertTrigger(false, false);
  public static final Bytes create2CallC_withRevert =
      CustomCreate2Payload.create2CallC_withRevertTrigger(true, false);
  public static final Bytes callMyselfWithCreate2CallC_withRevertAndNested =
      CustomCreate2Payload.callMyself(
          CustomCreate2Payload.create2CallC_withRevertTrigger(true, true), false, 5000000);

  // For Scenario 4 Unit tests
  public static final Bytes create2WithCallCtoCallback_noValue =
      CustomCreate2Payload.create2WithCallCtoCallback_noValue(false);
  public static final Bytes callMyselfWithCreate2WithCallCtoCallback_noValueAndNested =
      CustomCreate2Payload.callMyself(
          CustomCreate2Payload.create2WithCallCtoCallback_noValue(true), false, 7000000);

  // For Scenario 5 Unit tests
  public static final Bytes create2WithInitCodeC_withValue =
      CustomCreate2Payload.create2WithInitCodeC_withValueAndRevert();
  public static final Bytes callCToModifyStorageAndSelfdestruct =
      CustomCreate2Payload.callCToModifyStorageAndSelfdestruct();

  /*
  Logs for transaction validator
  */
  public static final String contractCreatedEvent =
      EventEncoder.encode(CustomCreate2.CONTRACTCREATED_EVENT);
  public static final String staticCallMyselfFailEvent =
      EventEncoder.encode(CustomCreate2.STATICCALLMYSELFFAIL_EVENT);
  public static final String callCreate2WithInitCodeC_withValue_Event =
      EventEncoder.encode(CustomCreate2.CALLCREATE2WITHINITCODEC_WITHVALUE_EVENT);
  public static final String callCreate2WithInitCodeC_noValue_Event =
      EventEncoder.encode(CustomCreate2.CALLCREATE2WITHINITCODEC_NOVALUE_EVENT);
  public static final String callMyselfFailEvent =
      EventEncoder.encode(CustomCreate2.CALLMYSELFFAIL_EVENT);
  public static final String storeInMapEvent = EventEncoder.encode(ContractC.STOREINMAP_EVENT);
  public static final String immediateRedeploymentFailEvent =
      EventEncoder.encode(ContractC.IMMEDIATEREDEPLOYMENTFAIL_EVENT);
  public static final String selfDestructEvent = EventEncoder.encode(ContractC.SELFDESTRUCT_EVENT);

  /// Common helpers

  /*
  Create transactions with payloads/values for the same user and the same to account
  */
  public static List<Transaction> getTransactions(
      ToyAccount to,
      ToyAccount userAccount,
      List<Bytes> payloads,
      List<AdvancedCreate2ScenarioValue> values) {

    checkArgument(payloads.size() == values.size());
    final List<ToyTransaction.ToyTransactionBuilder> builders = new ArrayList<>();

    for (int i = 0; i < payloads.size(); i++) {
      ToyTransaction.ToyTransactionBuilder builder =
          ToyTransaction.builder()
              .to(to)
              .payload(payloads.get(i))
              .keyPair(keyPair)
              .gasLimit(gasLimit)
              .value(Wei.of(values.get(i).getAdvancedCreate2ScenarioValue()));
      builders.add(builder);
    }
    return ToyMultiTransaction.builder().build(builders, userAccount);
  }

  public static void assertDeploymentNumberContractC(
      ToyExecutionEnvironmentV2 toyExecutionEnvironmentV2, int expectedDeploymentNumber) {
    int deploymentNumber =
        toyExecutionEnvironmentV2
            .getHub()
            .transients()
            .conflation()
            .deploymentInfo()
            .deploymentNumber(Address.fromHexString(expectedContractCAddress.toString()));
    assertEquals(
        expectedDeploymentNumber, deploymentNumber, "Unexpected deployment number for ContractC");
  }
}

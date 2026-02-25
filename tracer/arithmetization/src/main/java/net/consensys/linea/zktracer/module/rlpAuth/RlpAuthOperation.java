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

package net.consensys.linea.zktracer.module.rlpAuth;

import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;
import static org.hyperledger.besu.crypto.Hash.keccak256;
import static org.hyperledger.besu.ethereum.core.CodeDelegation.MAGIC;

import java.math.BigInteger;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.ModuleOperation;
import net.consensys.linea.zktracer.module.ecdata.EcData;
import net.consensys.linea.zktracer.module.hub.fragment.AuthorizationFragment;
import net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment;
import net.consensys.linea.zktracer.module.shakiradata.ShakiraData;
import net.consensys.linea.zktracer.module.shakiradata.ShakiraDataOperation;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import net.consensys.linea.zktracer.types.UnsignedByte;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.CodeDelegation;
import org.hyperledger.besu.ethereum.rlp.BytesValueRLPOutput;

@Accessors(fluent = true)
@Getter
public class RlpAuthOperation extends ModuleOperation {
  final AuthorizationFragment authorizationFragment;
  final CodeDelegation delegation;
  final TransactionProcessingMetadata txMetadata;
  final Bytes msg;

  RlpAuthOperation(
      AuthorizationFragment authorizationFragment, EcData ecData, ShakiraData shakiraData) {
    this.authorizationFragment = authorizationFragment;
    this.delegation = authorizationFragment.delegation();
    this.txMetadata = authorizationFragment.txMetadata();

    final Bytes magicConcatToRlpOfChainIdAddressNonceList =
        getMagicConcatToRlpOfChainIdAddressNonceList(
            delegation.chainId(), delegation.address(), delegation.nonce());
    this.msg = getMsg(magicConcatToRlpOfChainIdAddressNonceList);

    // Note:
    // msg = keccak(MAGIC || rlp([chain_id, address, nonce]))
    // authority = ecrecover(msg, y_parity, r, s)

    shakiraData.call(
        new ShakiraDataOperation(
            authorizationFragment.hubStamp(), magicConcatToRlpOfChainIdAddressNonceList));
    ecData.callEcData(
        authorizationFragment.hubStamp() + 1,
        PrecompileScenarioFragment.PrecompileFlag.PRC_ECRECOVER,
        Bytes.concatenate(
            Bytes32.leftPad(msg),
            Bytes32.leftPad(Bytes.ofUnsignedLong(UnsignedByte.of(delegation.v()).toInteger() + 27)),
            Bytes32.leftPad(bigIntegerToBytes(delegation.r())),
            Bytes32.leftPad(bigIntegerToBytes(delegation.s()))),
        Bytes32.leftPad(delegation.authorizer().orElse(Address.ZERO)));
  }

  protected void trace(Trace.Rlpauth trace) {
    trace
        // system data
        .blkNumber(txMetadata.getRelativeBlockNumber())
        .networkChainId(bigIntegerToBytes(authorizationFragment.networkChainId()))
        .userTxnNumber(txMetadata.getUserTransactionNumber())
        .txnFromAddress(txMetadata.getSender())
        .hubStamp(authorizationFragment.hubStamp())
        // tuple data
        .chainId(bigIntegerToBytes(delegation.chainId()))
        .delegationAddress(delegation.address())
        .nonce(Bytes.ofUnsignedLong((delegation.nonce())))
        .yParity(UnsignedByte.of(delegation.v()).toInteger())
        .r(bigIntegerToBytes(delegation.r()))
        .s(bigIntegerToBytes(delegation.s()))
        .msg(msg) // predicted output from keccak256
        // lookup to hub.auth/ data
        .tupleIndex(authorizationFragment.tupleIndex())
        .authorityEcrecoverSuccess(authorizationFragment.tracedEcRecoverSuccess())
        // senderIsAuthority is computed
        .senderIsAuthorityAcc(authorizationFragment.validSenderIsAuthorityAcc())
        .authorityAddress(
            authorizationFragment.tracedAuthorityAddress()) // predicted output from ecRecover
        .authorityNonce(Bytes.ofUnsignedLong(authorizationFragment.tracedAuthorityNonce()))
        .authorityHasEmptyCodeOrIsDelegated(
            authorizationFragment.tracedAuthorityHasEmptyCodeOrIsDelegated())
        .authorizationTupleIsValid(authorizationFragment.authorizationTupleIsValid())
        .validateRow();
  }

  @Override
  protected int computeLineCount() {
    return 1;
  }

  Bytes getMagicConcatToRlpOfChainIdAddressNonceList(
      BigInteger chainId, Address address, long nonce) {
    final BytesValueRLPOutput listRlp = new BytesValueRLPOutput();
    listRlp.startList();
    listRlp.writeBigIntegerScalar(chainId);
    listRlp.writeBytes(address);
    listRlp.writeLongScalar(nonce);
    listRlp.endList();
    final Bytes rlpOfListWithChainIdAddressNonce = listRlp.encoded();
    return Bytes.concatenate(MAGIC, rlpOfListWithChainIdAddressNonce);
  }

  Bytes getMsg(BigInteger chainId, Address address, long nonce) {
    return keccak256(getMagicConcatToRlpOfChainIdAddressNonceList(chainId, address, nonce));
  }

  Bytes getMsg(Bytes magicConcatToRlpOfChainIdAddressNonceList) {
    return keccak256(magicConcatToRlpOfChainIdAddressNonceList);
  }

  /* Alternative method
  Bytes getEcRecoverInput() {
    BytesValueRLPOutput rlpOutput = new BytesValueRLPOutput();
    CodeDelegationTransactionEncoder.encodeSingleCodeDelegationWithoutSignature(codeDelegation, rlpOutput);
    return Hash.hash(Bytes.concatenate(new Bytes[] {MAGIC, rlpOutput.encoded()}));
    msg = keccak(MAGIC || rlp([chain_id, address, nonce]))
  }
   */
}

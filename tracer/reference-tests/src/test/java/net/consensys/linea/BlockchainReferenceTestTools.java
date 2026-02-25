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

package net.consensys.linea;

import static net.consensys.linea.BlockchainReferenceTestJson.readBlockchainReferenceTestsOutput;
import static net.consensys.linea.ReferenceTestOutcomeRecorderTool.JSON_INPUT_FILENAME;
import static net.consensys.linea.reporting.TracerTestBase.getForkOrDefault;
import static net.consensys.linea.testing.ToyExecutionTools.addSystemAccountsIfRequired;
import static net.consensys.linea.zktracer.Fork.*;
import static org.assertj.core.api.Assertions.assertThat;

import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.HashSet;
import java.util.List;
import java.util.Optional;
import java.util.Set;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ConcurrentMap;
import java.util.concurrent.ConcurrentSkipListSet;
import java.util.concurrent.ExecutionException;
import java.util.stream.Collectors;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.corset.CorsetValidator;
import net.consensys.linea.reporting.TestOutcome;
import net.consensys.linea.reporting.TestOutcomeWriterTool;
import net.consensys.linea.testing.ExecutionEnvironment;
import net.consensys.linea.zktracer.ChainConfig;
import net.consensys.linea.zktracer.Fork;
import net.consensys.linea.zktracer.ZkTracer;
import org.hyperledger.besu.ethereum.BlockValidator;
import org.hyperledger.besu.ethereum.MainnetBlockValidatorBuilder;
import org.hyperledger.besu.ethereum.ProtocolContext;
import org.hyperledger.besu.ethereum.chain.MutableBlockchain;
import org.hyperledger.besu.ethereum.core.Block;
import org.hyperledger.besu.ethereum.core.BlockHeader;
import org.hyperledger.besu.ethereum.core.MutableWorldState;
import org.hyperledger.besu.ethereum.mainnet.BlockImportResult;
import org.hyperledger.besu.ethereum.mainnet.HeaderValidationMode;
import org.hyperledger.besu.ethereum.mainnet.MainnetBlockImporter;
import org.hyperledger.besu.ethereum.mainnet.ProtocolSchedule;
import org.hyperledger.besu.ethereum.mainnet.ProtocolSpec;
import org.hyperledger.besu.ethereum.referencetests.BlockchainReferenceTestCaseSpec;
import org.hyperledger.besu.ethereum.referencetests.ReferenceTestProtocolSchedules;
import org.hyperledger.besu.ethereum.rlp.RLPException;
import org.hyperledger.besu.ethereum.trie.pathbased.common.provider.WorldStateQueryParams;
import org.hyperledger.besu.testutil.JsonTestParameters;
import org.junit.jupiter.api.Assumptions;

@Slf4j
public class BlockchainReferenceTestTools {

  // Keep the forkName and the zkevm_fork in github worklow in PascalCase
  private static final Fork fork = getForkOrDefault(OSAKA);
  private static final ReferenceTestProtocolSchedules REFERENCE_TEST_PROTOCOL_SCHEDULES =
      ReferenceTestProtocolSchedules.create();
  private static final List<String> NETWORKS_TO_RUN = List.of(toPascalCase(fork));

  public static final JsonTestParameters<?, ?> PARAMS =
      JsonTestParameters.create(BlockchainReferenceTestCaseSpec.class)
          .generator(
              (testName, fullPath, spec, collector) -> {
                final String eip = spec.getNetwork();
                collector.add(
                    testName + "[" + eip + "]", fullPath, spec, NETWORKS_TO_RUN.contains(eip));
              });

  static {
    if (NETWORKS_TO_RUN.isEmpty()) {
      PARAMS.ignoreAll();
    }

    // ignore for v5.0 Osaka v1 release : type 3 and 4 transactions
    // PARAMS.ignore("/prague/eip6110_deposits/");
    PARAMS.ignore("/prague/eip7685_general_purpose_el_requests/");
    PARAMS.ignore(
        "/prague/eip7623_increase_calldata_cost/test_transaction_validity.py::test_transaction_validity_type_(3|4)");
    PARAMS.ignore(
        "/cancun/eip4788_beacon_root/test_beacon_root_contract.py::test_tx_to_beacon_root_contract\\[fork_(Prague|Osaka)-tx_type_(3|4)-blockchain_test-call_beacon_root_contract_True-auto_access_list_\\w+");
    PARAMS.ignore(
        "/prague/eip7623_increase_calldata_cost/test_execution_gas.py::TestGasConsumption::test_full_gas_consumption\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-exact_gas-type_(3|4)");
    PARAMS.ignore(
        "/prague/eip7623_increase_calldata_cost/test_execution_gas.py::TestGasConsumption::test_full_gas_consumption\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-extra_gas-type_(3|4)");
    PARAMS.ignore(
        "/prague/eip7623_increase_calldata_cost/test_execution_gas.py::TestGasConsumptionBelowDataFloor::test_gas_consumption_below_data_floor\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-exact_gas-type_(3|4)");
    PARAMS.ignore(
        "/prague/eip7623_increase_calldata_cost/test_execution_gas.py::TestGasConsumptionBelowDataFloor::test_gas_consumption_below_data_floor\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-exact_gas-type_4");
    PARAMS.ignore(
        "/prague/eip7623_increase_calldata_cost/test_refunds.py::test_gas_refunds_from_data_floor\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-refund_type_RefundType.AUTHORIZATION_EXISTING_AUTHORITY-refund_test_type_RefundTestType.EXECUTION_GAS_MINUS_REFUND*");
    PARAMS.ignore(
        "/prague/eip7623_increase_calldata_cost/test_refunds.py::test_gas_refunds_from_data_floor\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-refund_type_RefundType.AUTHORIZATION_EXISTING_AUTHORITY-refund_test_type_RefundTestType.EXECUTION_GAS_MINUS_REFUND*");
    PARAMS.ignore(
        "/prague/eip7623_increase_calldata_cost/test_refunds.py::test_gas_refunds_from_data_floor\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-refund_type_RefundType.STORAGE_CLEAR|AUTHORIZATION_EXISTING_AUTHORITY-refund_test_type_RefundTestType.EXECUTION_GAS_MINUS_REFUND*");
    // note : called none0 and none1 but are txs of type 4 and 3 respectively
    PARAMS.ignore(
        "/osaka/eip7825_transaction_gas_limit_cap/test_tx_gas_limit.py::test_transaction_gas_limit_cap\\[fork_(Prague|Osaka)-tx_gas_limit_cap_none0-blockchain_test_from_state_test\\]");
    PARAMS.ignore(
        "/osaka/eip7825_transaction_gas_limit_cap/test_tx_gas_limit.py::test_transaction_gas_limit_cap\\[fork_(Prague|Osaka)-tx_gas_limit_cap_none1-blockchain_test_from_state_test\\]");
    PARAMS.ignore(
        "/istanbul/eip1344_chainid/test_chainid.py::test_chainid\\[fork_(Prague|Osaka)-typed_transaction_(3|4)*");
    PARAMS.ignore(
        "/osaka/eip7934_block_rlp_limit/test_max_block_rlp_size.py::test_block_at_rlp_limit_with_logs*");
    PARAMS.ignore(
        "/osaka/eip7934_block_rlp_limit/test_max_block_rlp_size.py::test_block_rlp_size_at_limit_with_all_typed_transactions*");
    PARAMS.ignore(
        "prague/eip7623_increase_calldata_cost/test_eip_mainnet.py::test_eip_7623\\[fork_Prague-blockchain_test_from_state_test----type_3\\]\\[Prague\\]");
    // PARAMS.ignore("prague/eip7623_increase_calldata_cost/test_eip_mainnet.py::test_eip_7623\\[fork_Prague-blockchain_test_from_state_test----type_4\\]\\[Prague\\]");

    // type 3 (BLOB):
    PARAMS.ignore("/cancun/eip4844_blobs/");
    PARAMS.ignore("/Cancun/stEIP4844_blobtransactions");
    PARAMS.ignore(
        "/osaka/eip7918_blob_reserve_price/test_blob_base_fee.py::test_reserve_price_various_base_fee_scenarios*");
    PARAMS.ignore(
        "/osaka/eip7918_blob_reserve_price/test_blob_base_fee.py::test_reserve_price_boundary*");
    PARAMS.ignore("/osaka/eip7594*");
    PARAMS.ignore(
        "eip7825_transaction_gas_limit_cap/test_tx_gas_limit.py::test_transaction_gas_limit_cap\\[fork_Osaka-tx_gas_limit_cap_over1-blockchain_test_from_state_test\\]");
    PARAMS.ignore("fork_Osaka-tx_type_3-evm_code_type_LEGACY");

    // // type 4 (7702):
    // PARAMS.ignore("tests/osaka/eip7939_count_leading_zeros/test_count_leading_zeros.py::test_clz_from_set_code");
    // PARAMS.ignore("tests/osaka/eip7825_transaction_gas_limit_cap/test_tx_gas_limit.py::test_transaction_gas_limit_cap\\[fork_Osaka-tx_gas_limit_cap_over(0|1)-blockchain_test_from_state_test*");
    // PARAMS.ignore("tests/osaka/eip7825_transaction_gas_limit_cap/test_tx_gas_limit.py::test_tx_gas_limit_cap_authorized_tx\\[fork_Osaka-blockchain_test_from_state_test-exceed_tx_gas_limit_False-correct_intrinsic_cost_in_transaction_gas_limit_True\\]*");

    // tests that timeout and pass locally
    // Log when launching locally
    // Test Name:
    // tests/frontier/scenarios/test_scenarios.py::test_scenarios[fork_Prague-blockchain_test-test_program_program_BLOCKHASH-debug][Prague]
    // PASSED (27m 52s)
    PARAMS.ignore(
        "frontier/scenarios/test_scenarios.py::test_scenarios\\[fork_(Prague|Osaka)-blockchain_test-test_program_program_BLOCKHASH-debug\\]");

    // tests that timeout even locally
    // Log when launching locally
    // Test Name:
    // tests/cancun/eip1153_tstore/test_tstorage.py::test_run_until_out_of_gas[fork_Prague-tx_gas_limit_0x055d4a80-blockchain_test_from_state_test-tstore_wide_address_space][Prague] FAILED (1h 3m)
    // java.util.concurrent.TimeoutException: execution(java.lang.String,
    // org.hyperledger.besu.ethereum.referencetests.BlockchainReferenceTestCaseSpec, boolean)
    // timed
    // out after 60 minutes
    PARAMS.ignore("/cancun/eip1153_tstore/test_tstorage.py::test_run_until_out_of_gas");

    // withdrawals are not supported by Linea
    PARAMS.ignore("/prague/eip7002_el_triggerable_withdrawals/");
    PARAMS.ignore("/prague/eip7002_el_triggerable_withdrawals_and_transfers/");
    PARAMS.ignore(
        "cancun/eip4788_beacon_root/test_beacon_root_contract.py::test_multi_block_beacon_root_timestamp_calls");
    PARAMS.ignore("shanghai/eip4895_withdrawals/test_withdrawals.py::test_balance_within_block");
    PARAMS.ignore("shanghai/eip4895_withdrawals/test_withdrawals.py::test_use_value_in_contract");

    // ignore tests that are failing in Besu too
    PARAMS.ignore(
        "RevertInCreateInInitCreate2Paris\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-\\]");
    PARAMS.ignore(
        "create2collisionStorageParis\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d0\\]");
    PARAMS.ignore(
        "create2collisionStorageParis\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d1\\]");
    PARAMS.ignore(
        "create2collisionStorageParis\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d2\\]");
    PARAMS.ignore(
        "dynamicAccountOverwriteEmpty_Paris\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-\\]");

    // Arithmetization restriction: recipient address is a precompile.
    PARAMS.ignore(
        "osaka/eip7883_modexp_gas_increase/test_modexp_thresholds.py::test_modexp_used_in_transaction_entry_points");
    PARAMS.ignore(
        "modexpRandomInput\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d0-g0\\]");
    PARAMS.ignore(
        "modexpRandomInput\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d0-g1\\]");
    PARAMS.ignore(
        "modexpRandomInput\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d1-g0\\]");
    PARAMS.ignore(
        "modexpRandomInput\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d1-g1\\]");
    PARAMS.ignore(
        "modexpRandomInput\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d2-g0\\]");
    PARAMS.ignore(
        "modexpRandomInput\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d2-g1\\]");
    PARAMS.ignore("randomStatetest642\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-\\]");
    PARAMS.ignore("randomStatetest644\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-\\]");
    PARAMS.ignore(
        "randomStatetest645\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--v0\\]");
    PARAMS.ignore(
        "randomStatetest645\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--v1\\]");
    PARAMS.ignore(
        "test_p256verify.py::test_precompile_as_tx_entry_point\\[fork_Osaka-blockchain_test_from_state_test-valid_entry_point\\]\\[Osaka\\]");
    PARAMS.ignore(
        "/prague/eip7702_set_code_tx/test_set_code_txs_2.py::test_gas_diff_pointer_vs_direct_call\\[fork_Osaka-blockchain_test-access_list_to_AccessListTo");

    // Consumes a huge amount of memory.
    PARAMS.ignore(
        "stStaticCall/static_Return50000_2Filler.json::static_Return50000_2\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-\\]");

    // Balance is more than 128 bits (arithmetization limitation).
    PARAMS.ignore(
        "stMemoryStressTest/CALLCODE_BoundsFiller.json::CALLCODE_Bounds\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stMemoryStressTest/CALLCODE_BoundsFiller.json::CALLCODE_Bounds\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore(
        "stMemoryStressTest/CALLCODE_Bounds2Filler.json::CALLCODE_Bounds2\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stMemoryStressTest/CALLCODE_Bounds2Filler.json::CALLCODE_Bounds2\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore(
        "stMemoryStressTest/CALLCODE_Bounds3Filler.json::CALLCODE_Bounds3\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stMemoryStressTest/CALLCODE_Bounds3Filler.json::CALLCODE_Bounds3\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore(
        "stMemoryStressTest/CALLCODE_Bounds4Filler.json::CALLCODE_Bounds4\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stMemoryStressTest/CALLCODE_Bounds4Filler.json::CALLCODE_Bounds4\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore(
        "stMemoryStressTest/CALLCODE_Bounds4Filler.json::CALLCODE_Bounds4\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g2\\]");
    PARAMS.ignore(
        "stMemoryStressTest/static_CALL_BoundsFiller.json::static_CALL_Bounds\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stMemoryStressTest/static_CALL_BoundsFiller.json::static_CALL_Bounds\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore(
        "stMemoryStressTest/static_CALL_Bounds2Filler.json::static_CALL_Bounds2\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stMemoryStressTest/static_CALL_Bounds2Filler.json::static_CALL_Bounds2\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore(
        "stMemoryStressTest/static_CALL_Bounds2aFiller.json::static_CALL_Bounds2a\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stMemoryStressTest/static_CALL_Bounds2aFiller.json::static_CALL_Bounds2a\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore(
        "stMemoryStressTest/static_CALL_Bounds3Filler.json::static_CALL_Bounds3\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stMemoryStressTest/static_CALL_Bounds3Filler.json::static_CALL_Bounds3\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore(
        "stMemoryStressTest/CALL_BoundsFiller.json::CALL_Bounds\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stMemoryStressTest/CALL_BoundsFiller.json::CALL_Bounds\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore(
        "stMemoryStressTest/CALL_Bounds2Filler.json::CALL_Bounds2\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stMemoryStressTest/CALL_Bounds2Filler.json::CALL_Bounds2\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore(
        "stMemoryStressTest/CALL_Bounds2aFiller.json::CALL_Bounds2a\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stMemoryStressTest/CALL_Bounds2aFiller.json::CALL_Bounds2a\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore(
        "stMemoryStressTest/CALL_Bounds3Filler.json::CALL_Bounds3\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stMemoryStressTest/CALL_Bounds3Filler.json::CALL_Bounds3\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore(
        "stMemoryStressTest/CALL_Bounds3Filler.json::CALL_Bounds3\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g2\\]");
    PARAMS.ignore(
        "stMemoryStressTest/CREATE_BoundsFiller.json::CREATE_Bounds\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stMemoryStressTest/CREATE_BoundsFiller.json::CREATE_Bounds\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore(
        "stMemoryStressTest/CREATE_Bounds2Filler.json::CREATE_Bounds2\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stMemoryStressTest/CREATE_Bounds2Filler.json::CREATE_Bounds2\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore(
        "stMemoryStressTest/CREATE_Bounds3Filler.json::CREATE_Bounds3\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stMemoryStressTest/CREATE_Bounds3Filler.json::CREATE_Bounds3\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore(
        "stMemoryStressTest/CREATE_Bounds3Filler.json::CREATE_Bounds3\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g2\\]");
    PARAMS.ignore(
        "stCreate2/CREATE2_BoundsFiller.json::CREATE2_Bounds\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stCreate2/CREATE2_BoundsFiller.json::CREATE2_Bounds\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore(
        "stCreate2/CREATE2_Bounds2Filler.json::CREATE2_Bounds2\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stCreate2/CREATE2_Bounds2Filler.json::CREATE2_Bounds2\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore(
        "stCreate2/CREATE2_Bounds3Filler.json::CREATE2_Bounds3\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stCreate2/CREATE2_Bounds3Filler.json::CREATE2_Bounds3\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore(
        "stCreate2/CREATE2_Bounds3Filler.json::CREATE2_Bounds3\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g2\\]");
    PARAMS.ignore(
        "stMemoryStressTest/DELEGATECALL_BoundsFiller.json::DELEGATECALL_Bounds\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stMemoryStressTest/DELEGATECALL_BoundsFiller.json::DELEGATECALL_Bounds\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore(
        "stMemoryStressTest/DELEGATECALL_Bounds2Filler.json::DELEGATECALL_Bounds2\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stMemoryStressTest/DELEGATECALL_Bounds2Filler.json::DELEGATECALL_Bounds2\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore(
        "stMemoryStressTest/DELEGATECALL_Bounds3Filler.json::DELEGATECALL_Bounds3\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stMemoryStressTest/DELEGATECALL_Bounds3Filler.json::DELEGATECALL_Bounds3\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore(
        "stMemoryStressTest/DELEGATECALL_Bounds3Filler.json::DELEGATECALL_Bounds3\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g2\\]");
    PARAMS.ignore(
        "stMemoryStressTest/MSTORE_BoundsFiller.json::MSTORE_Bounds\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stMemoryStressTest/MSTORE_BoundsFiller.json::MSTORE_Bounds\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore(
        "stMemoryStressTest/MSTORE_Bounds2Filler.json::MSTORE_Bounds2\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stMemoryStressTest/MSTORE_Bounds2Filler.json::MSTORE_Bounds2\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore(
        "stMemoryStressTest/MSTORE_Bounds2aFiller.json::MSTORE_Bounds2a\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stMemoryStressTest/MSTORE_Bounds2aFiller.json::MSTORE_Bounds2a\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore("HighGasLimit\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-\\]");
    PARAMS.ignore(
        "stInitCodeTest/OutOfGasContractCreationFiller.json::OutOfGasContractCreation\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d0-g0\\]");
    PARAMS.ignore(
        "stInitCodeTest/OutOfGasContractCreationFiller.json::OutOfGasContractCreation\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d0-g1\\]");
    PARAMS.ignore(
        "stInitCodeTest/OutOfGasContractCreationFiller.json::OutOfGasContractCreation\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d1-g0\\]");
    PARAMS.ignore(
        "stInitCodeTest/OutOfGasContractCreationFiller.json::OutOfGasContractCreation\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d1-g1\\]");
    PARAMS.ignore(
        "stMemoryStressTest/RETURN_BoundsFiller.json::RETURN_Bounds\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stMemoryStressTest/RETURN_BoundsFiller.json::RETURN_Bounds\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore(
        "stMemoryStressTest/RETURN_BoundsFiller.json::RETURN_Bounds\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g2\\]");
    PARAMS.ignore(
        "stCallCreateCallCodeTest/Call1024PreCallsFiller.json::Call1024PreCalls\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stCallCreateCallCodeTest/Call1024PreCallsFiller.json::Call1024PreCalls\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore(
        "stStaticCall/static_Call1024PreCallsFiller.json::static_Call1024PreCalls\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d0\\]");
    PARAMS.ignore(
        "stStaticCall/static_Call1024PreCallsFiller.json::static_Call1024PreCalls\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d1\\]");
    PARAMS.ignore(
        "stStaticCall/static_Call1024PreCalls2Filler.json::static_Call1024PreCalls2\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d0\\]");
    PARAMS.ignore(
        "stStaticCall/static_Call1024PreCalls2Filler.json::static_Call1024PreCalls2\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d1\\]");
    PARAMS.ignore(
        "stStaticCall/static_Call1024PreCalls3Filler.json::static_Call1024PreCalls3\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d0\\]");
    PARAMS.ignore(
        "stStaticCall/static_Call1024PreCalls3Filler.json::static_Call1024PreCalls3\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d1\\]");
    PARAMS.ignore("static_RETURN_Bounds\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-\\]");
    PARAMS.ignore(
        "stStaticCall/static_RETURN_BoundsOOGFiller.json::static_RETURN_BoundsOOG\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d0\\]");
    PARAMS.ignore(
        "stStaticCall/static_RETURN_BoundsOOGFiller.json::static_RETURN_BoundsOOG\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d1\\]");
    PARAMS.ignore(
        "stDelegatecallTestHomestead/Call1024PreCallsFiller.json::Call1024PreCalls\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stDelegatecallTestHomestead/Call1024PreCallsFiller.json::Call1024PreCalls\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore(
        "stDelegatecallTestHomestead/Call1024PreCallsFiller.json::Call1024PreCalls\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g2\\]");
    PARAMS.ignore("Create2OnDepth1023\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-\\]");
    PARAMS.ignore(
        "stCreate2/Create2RecursiveFiller.json::Create2Recursive\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0\\]");
    PARAMS.ignore(
        "stCreate2/Create2RecursiveFiller.json::Create2Recursive\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1\\]");
    PARAMS.ignore(
        "stCreate2/Create2RecursiveFiller.json::Create2Recursive\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g2\\]");
    PARAMS.ignore("Create2OnDepth1024\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-\\]");
    PARAMS.ignore(
        "stTransactionTest/OverflowGasRequire2Filler.json::OverflowGasRequire2\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-\\]");
    PARAMS.ignore(
        "stStaticCall/static_Call50000_ecrecFiller.json::static_Call50000_ecrec\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d0\\]");
    PARAMS.ignore(
        "stStaticCall/static_Call50000_ecrecFiller.json::static_Call50000_ecrec\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d1\\]");

    // Deployment transaction to an account with nonce / code
    PARAMS.ignore(
        "TransactionCollisionToEmptyButCode\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0-v0\\]");
    PARAMS.ignore(
        "TransactionCollisionToEmptyButCode\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0-v1\\]");
    PARAMS.ignore(
        "TransactionCollisionToEmptyButCode\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1-v0\\]");
    PARAMS.ignore(
        "TransactionCollisionToEmptyButCode\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1-v1\\]");
    PARAMS.ignore(
        "TransactionCollisionToEmptyButNonce\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0-v0\\]");
    PARAMS.ignore(
        "TransactionCollisionToEmptyButNonce\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g0-v1\\]");
    PARAMS.ignore(
        "TransactionCollisionToEmptyButNonce\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1-v0\\]");
    PARAMS.ignore(
        "TransactionCollisionToEmptyButNonce\\[fork_(Prague|Osaka)-blockchain_test_from_state_test--g1-v1\\]");
    PARAMS.ignore(
        "initCollidingWithNonEmptyAccount\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d0\\]");
    PARAMS.ignore(
        "initCollidingWithNonEmptyAccount\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d1\\]");
    PARAMS.ignore(
        "initCollidingWithNonEmptyAccount\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d2\\]");
    PARAMS.ignore(
        "initCollidingWithNonEmptyAccount\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d3\\]");
    PARAMS.ignore(
        "initCollidingWithNonEmptyAccount\\[fork_(Prague|Osaka)-blockchain_test_from_state_test-d4\\]");

    // EIP-7251 not supported by Linea: "World State Root does not match expected value"
    PARAMS.ignore("/prague/eip7251_consolidations/");

    // Prior to Osaka MODEXP had unsupported arguments in the arithmetization
    if (forkPredatesOsaka(fork)) {
      // massive mbs
      PARAMS.ignore(
          "modexpFiller.json::modexp\\[fork_.*-blockchain_test_from_state_test-d2-g[0-3]\\]");
      PARAMS.ignore(
          "test_modexp.py::test_modexp\\[fork_.*-blockchain_test_from_state_test-EIP-198-case3-raw-input-out-of-gas\\]");
      PARAMS.ignore(
          "modexp_modsize0_returndatasizeFiller\\.json::modexp_modsize0_returndatasize\\[fork_.*-blockchain_test_from_state_test-d4\\]");
      PARAMS.ignore(
          "test_modexp.py::test_modexp\\[fork_Prague-blockchain_test_from_state_test-large-modulus-length-0x20000020-out-of-gas\\]\\[Prague\\]");
      PARAMS.ignore(
          "test_modexp.py::test_modexp\\[fork_Prague-blockchain_test_from_state_test-large-modulus-length-0x40000020-out-of-gas\\]\\[Prague\\]");
      PARAMS.ignore(
          "test_modexp.py::test_modexp\\[fork_Prague-blockchain_test_from_state_test-large-modulus-length-0x80000020-out-of-gas\\]\\[Prague\\]");
      PARAMS.ignore(
          "test_modexp.py::test_modexp\\[fork_Prague-blockchain_test_from_state_test-large-modulus-length-0xffffffff-out-of-gas\\]\\[Prague\\]");

      // massive bbs
      PARAMS.ignore(
          "modexpFiller.json::modexp\\[fork_.*-blockchain_test_from_state_test-d28-g[0-3]\\]");
      PARAMS.ignore(
          "test_precompiles.py::test_precompiles\\[fork_.*-address_0x0000000000000000000000000000000000000005-precompile_exists_True-blockchain_test_from_state_test\\]");

      // massive ebs
      PARAMS.ignore(
          "modexpFiller.json::modexp\\[fork_.*-blockchain_test_from_state_test-d29-g[0-3]\\]");
      PARAMS.ignore(
          "modexpFiller.json::modexp\\[fork_.*-blockchain_test_from_state_test-d30-g[0-3]\\]");
      PARAMS.ignore(
          "modexpFiller.json::modexp\\[fork_.*-blockchain_test_from_state_test-d36-g[0-3]\\]");
      PARAMS.ignore(
          "modexpFiller.json::modexp\\[fork_.*-blockchain_test_from_state_test-d37-g[0-3]\\]");
      PARAMS.ignore(
          "randomStatetest650Filler.json::randomStatetest650\\[fork_.*-blockchain_test_from_state_test-\\]");
      PARAMS.ignore(
          "test_modexp.py::test_modexp\\[fork_Prague-blockchain_test_from_state_test-ModExpInput_base_0x-exponent_0x-modulus_0x-declared_exponent_length_4294967296-declared_modulus_length_1-ModExpOutput_call_success_False-returned_data_0x00\\]\\[Prague\\]");
      PARAMS.ignore(
          "test_modexp.py::test_modexp\\[fork_Prague-blockchain_test_from_state_test-large-exponent-length-0x10000000-out-of-gas\\]\\[Prague\\]");
      PARAMS.ignore(
          "test_modexp.py::test_modexp\\[fork_Prague-blockchain_test_from_state_test-large-exponent-length-0x20000000-out-of-gas\\]\\[Prague\\]");
      PARAMS.ignore(
          "test_modexp.py::test_modexp\\[fork_Prague-blockchain_test_from_state_test-large-exponent-length-0x40000000-out-of-gas\\]\\[Prague\\]");
      PARAMS.ignore(
          "test_modexp.py::test_modexp\\[fork_Prague-blockchain_test_from_state_test-large-exponent-length-0x80000000-out-of-gas\\]\\[Prague\\]");

      // byte sizes 512 < xbs ≤ 1024
      PARAMS.ignore(
          "test_modexp_thresholds.py::test_modexp_variable_gas_cost_exceed_tx_gas_cap\\[fork_.*-blockchain_test_from_state_test-Z16-gas-cap-test\\]");
      PARAMS.ignore(
          "test_modexp_thresholds.py::test_vectors_from_eip\\[fork_.*-blockchain_test_from_state_test-guido-3-even\\]");
      PARAMS.ignore(
          "test_modexp_thresholds.py::test_vectors_from_eip\\[fork_.*-blockchain_test_from_state_test-nagydani-5-pow0x10001\\]");
      PARAMS.ignore(
          "test_modexp_thresholds.py::test_vectors_from_eip\\[fork_.*-blockchain_test_from_state_test-nagydani-5-qube\\]");
      PARAMS.ignore(
          "test_modexp_thresholds.py::test_vectors_from_eip\\[fork_.*-blockchain_test_from_state_test-nagydani-5-square\\]");

      // massive xbs'
      PARAMS.ignore(
          "test_modexp_thresholds.py::test_modexp_invalid_inputs\\[fork_.*-blockchain_test_from_state_test--invalid-case-[1-3]\\]");

      // Osaka legal xbs's (at most one being 1024)
      PARAMS.ignore(
          "test_modexp_thresholds.py::test_modexp_variable_gas_cost\\[fork_.*-blockchain_test_from_state_test-Z[2347]\\]");
      PARAMS.ignore(
          "test_modexp_thresholds.py::test_modexp_variable_gas_cost\\[fork_.*-blockchain_test_from_state_test-Z1[2-5]\\]");
    }

    if (forkPredatesAmsterdam(fork)) {
      // EIP-7610 is proposed for Amsterdam (and will be supported by Linea) but previous fork
      // contains EIP-7610 in BlockchainReferenceTest_42 ...
      PARAMS.ignore("tests/paris/eip7610_*");
    }
  }

  public static CompletableFuture<Set<String>> getRecordedFailedTestsFromJson(
      String failedModule, String failedConstraint) {
    Set<String> failedTests = new HashSet<>();
    if (failedModule.isEmpty()) {
      return CompletableFuture.completedFuture(failedTests);
    }

    CompletableFuture<TestOutcome> modulesToConstraintsFutures =
        readBlockchainReferenceTestsOutput(JSON_INPUT_FILENAME)
            .thenApply(TestOutcomeWriterTool::parseTestOutcome);

    return modulesToConstraintsFutures.thenApply(
        blockchainReferenceTestOutcome -> {
          ConcurrentMap<String, ConcurrentSkipListSet<String>> filteredFailedTests =
              blockchainReferenceTestOutcome.getModulesToConstraintsToTests().get(failedModule);
          if (filteredFailedTests == null) {
            return failedTests;
          }
          if (!failedConstraint.isEmpty()) {
            return filteredFailedTests.get(failedConstraint);
          }
          return filteredFailedTests.values().stream()
              .flatMap(Set::stream)
              .collect(Collectors.toSet());
        });
  }

  public static Collection<Object[]> generateTestParametersForConfig(
      final String[] filePath, final String testSrcPath) {
    Arrays.stream(filePath).forEach(f -> log.info("checking file: {}", f));
    return PARAMS.generate(
        Arrays.stream(filePath).map(f -> Paths.get(testSrcPath + "/" + f).toFile()).toList());
  }

  public static Collection<Object[]> generateTestParametersForConfigForFailedTests(
      final String[] filePath,
      final String testSrcPath,
      String failedModule,
      String failedConstraint)
      throws ExecutionException, InterruptedException {
    Arrays.stream(filePath).forEach(f -> log.info("checking file: {}", f));
    Collection<Object[]> params =
        PARAMS.generate(
            Arrays.stream(filePath).map(f -> Paths.get(testSrcPath + "/" + f).toFile()).toList());

    return getRecordedFailedTestsFromJson(failedModule, failedConstraint)
        .thenApply(
            failedTests -> {
              List<Object[]> modifiedParams = new ArrayList<>();
              for (Object[] param : params) {
                Object[] modifiedParam = markTestToRun(param, failedTests);
                modifiedParams.add(modifiedParam);
              }
              return modifiedParams;
            })
        .get();
  }

  public static Object[] markTestToRun(Object[] param, Set<String> failedTests) {
    String testName = (String) param[0];
    param[2] = failedTests.contains(testName);

    return param;
  }

  // static boolean kzgIsInitialized = false;

  public static void executeTest(final BlockchainReferenceTestCaseSpec spec) {
    // TODO: dear developer of the future, this may be needed in case tests fail due to KZG
    //  / point evaluation is not initialized
    // Initialize KZG library once for all reference tests
    /*
    if (!kzgIsInitialized) {
      KZGPointEvalPrecompiledContract.init();
      kzgIsInitialized = true;
    }
     */

    final BlockHeader genesisBlockHeader = spec.getGenesisBlockHeader();
    final MutableBlockchain blockchain = spec.buildBlockchain();
    final ProtocolContext context = spec.buildProtocolContext(blockchain);
    final MutableWorldState worldState =
        context
            .getWorldStateArchive()
            .getWorldState(
                WorldStateQueryParams.withBlockHeaderAndNoUpdateNodeHead(genesisBlockHeader))
            .orElseThrow();
    log.info(
        "checking roothash {} is {}", worldState.rootHash(), genesisBlockHeader.getStateRoot());
    assertThat(worldState.rootHash()).isEqualTo(genesisBlockHeader.getStateRoot());

    final ProtocolSchedule schedule =
        REFERENCE_TEST_PROTOCOL_SCHEDULES.getByName(spec.getNetwork());
    final ChainConfig chain = ChainConfig.ETHEREUM_CHAIN(fork);

    // Add system accounts if the fork requires it.
    addSystemAccountsIfRequired(worldState.updater());

    final CorsetValidator corsetValidator = new CorsetValidator(chain);
    final ZkTracer zkTracer = new ZkTracer(chain);
    zkTracer.traceStartConflation(spec.getCandidateBlocks().length);

    for (var candidateBlock : spec.getCandidateBlocks()) {
      Assumptions.assumeTrue(
          candidateBlock.areAllTransactionsValid(),
          "Skipping the test because the block is not executable");
      Assumptions.assumeTrue(
          candidateBlock.isExecutable(), "Skipping the test because the block is not executable");
      Assumptions.assumeTrue(
          candidateBlock.getBlock().getBody().getTransactions().size() > 0,
          "Skipping the test because the block has no transaction");
      Assumptions.assumeTrue(
          Arrays.stream(spec.getCandidateBlocks()).filter(b -> !b.isValid()).count() == 0,
          "Skipping the test because it has invalid blocks");

      try {
        final Block block = candidateBlock.getBlock();

        zkTracer.traceStartBlock(worldState, block.getHeader(), block.getHeader().getCoinbase());

        final ProtocolSpec protocolSpec = schedule.getByBlockHeader(block.getHeader());

        final MainnetBlockImporter blockImporter =
            getMainnetBlockImporter(protocolSpec, schedule, zkTracer);

        final HeaderValidationMode validationMode =
            "NoProof".equalsIgnoreCase(spec.getSealEngine())
                ? HeaderValidationMode.LIGHT
                : HeaderValidationMode.FULL;

        // Note: somehow this function is calling traceEndBlock through
        // blockValidator.validateAndProcessBlock
        final BlockImportResult importResult =
            blockImporter.importBlock(context, block, validationMode, validationMode);
        log.info(
            "checking block is imported {} equals {}",
            importResult.isImported(),
            candidateBlock.isValid());
        assertThat(importResult.isImported())
            .isEqualTo(candidateBlock.isValid())
            .withFailMessage(
                "checking block is imported {} while expected {}",
                importResult.isImported(),
                candidateBlock.isValid());

      } catch (final RLPException e) {
        log.info("caught RLP exception, checking it's invalid {}", candidateBlock.isValid());
        assertThat(candidateBlock.isValid()).isFalse();
      }
    }

    zkTracer.traceEndConflation(worldState);
    ExecutionEnvironment.checkTracer(
        zkTracer,
        corsetValidator,
        Optional.of(log),
        // NOTE: just use 0 for start and end block here, since this information is not used.
        0,
        0,
        null);
    assertThat(blockchain.getChainHeadHash()).isEqualTo(spec.getLastBlockHash());
  }

  private static MainnetBlockImporter getMainnetBlockImporter(
      final ProtocolSpec protocolSpec, final ProtocolSchedule schedule, final ZkTracer zkTracer) {
    final CorsetBlockProcessor corsetBlockProcessor =
        new CorsetBlockProcessor(
            protocolSpec.getTransactionProcessor(),
            protocolSpec.getTransactionReceiptFactory(),
            protocolSpec.getBlockReward(),
            protocolSpec.getMiningBeneficiaryCalculator(),
            protocolSpec.isSkipZeroBlockRewards(),
            schedule,
            zkTracer);

    final BlockValidator blockValidator =
        MainnetBlockValidatorBuilder.frontier(
            protocolSpec.getBlockHeaderValidator(),
            protocolSpec.getBlockBodyValidator(),
            corsetBlockProcessor);

    return new MainnetBlockImporter(blockValidator);
  }
}

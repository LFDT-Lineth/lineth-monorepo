#!/usr/bin/env sh
set -eu

ROOT=$(git rev-parse --show-toplevel)
STACK_REL=docs/getting-started/lineth-stack
STACK="$ROOT/$STACK_REL"
FAILURES=0

fail() {
  printf '[quickstart-static] FAIL: %s\n' "$*" >&2
  FAILURES=$((FAILURES + 1))
}

pass() {
  printf '[quickstart-static] OK: %s\n' "$*"
}

check_no_tracked_generated_genesis() {
  tracked=$(git -C "$ROOT" ls-files "$STACK_REL/config" \
    | grep -E '/(genesis-(besu|maru)\.json|fork-timestamp\.txt)$' || true)
  if [ -n "$tracked" ]; then
    fail "generated L2 genesis artifacts are tracked: $(printf '%s' "$tracked" | tr '\n' ' ')"
  else
    pass "generated L2 genesis artifacts are not tracked"
  fi

  for template in genesis-besu.json.template genesis-maru.json.template; do
    if git -C "$ROOT" ls-files --error-unmatch "$STACK_REL/config/genesis/$template" >/dev/null 2>&1; then
      pass "$template is tracked"
    else
      fail "$template must remain tracked"
    fi
  done
}

check_restructured_layout_paths() {
  compose="$STACK/docker-compose.yml"
  old_config_path='config/''l2/'
  old_init_render='scripts/init/''config-render'
  old_postman_render='scripts/init/''render-postman-env'
  old_deploy_phase='scripts/internal/''deploy-contracts.sh'

  if grep -R -n "$old_config_path" "$compose" "$STACK/scripts" >/tmp/lineth-stack-old-config-paths.txt 2>/dev/null; then
    fail "runtime Compose/scripts still reference old L2 config paths: $(tr '\n' ' ' </tmp/lineth-stack-old-config-paths.txt)"
  else
    pass "runtime Compose/scripts use config/genesis and config/services paths"
  fi

  if grep -R -n -e "$old_init_render" -e "$old_postman_render" -e "$old_deploy_phase" "$compose" "$STACK/scripts" >/tmp/lineth-stack-old-script-paths.txt 2>/dev/null; then
    fail "runtime Compose/scripts still reference old phase/renderer paths: $(tr '\n' ' ' </tmp/lineth-stack-old-script-paths.txt)"
  else
    pass "runtime Compose/scripts use scripts/phases and scripts/services paths"
  fi

  if [ -d "$STACK/scripts/phases" ] \
    && [ -d "$STACK/scripts/services" ] \
    && [ -d "$STACK/config/genesis" ] \
    && [ -d "$STACK/config/services" ]; then
    pass "quickstart source layout has phases, services, genesis, and service config directories"
  else
    fail "quickstart source layout must have scripts/phases, scripts/services, config/genesis, and config/services"
  fi
}

check_runtime_helper_usage() {
  runtime_helper="$STACK/scripts/lib/runtime.sh"
  required_scripts="
links.sh
status.sh
export-output.sh
traffic-generation/send-l2-test-tx.sh
traffic-generation/send-l2-erc20-transfer.sh
traffic-generation/generate-l2-erc20-traffic.sh
smoke-test/smoke-bridge-message.sh
smoke-test/smoke-bridge-message-l2-to-l1.sh
smoke-test/smoke-bridge-erc20-l1-to-l2.sh
smoke-test/smoke-bridge-erc20-l2-to-l1.sh
"

  if [ -f "$runtime_helper" ] \
    && grep -q 'lineth_runtime_init()' "$runtime_helper" \
    && grep -q 'lineth_env_value()' "$runtime_helper" \
    && grep -q 'lineth_json_section_addr()' "$runtime_helper" \
    && grep -q 'lineth_require_address()' "$runtime_helper"; then
    pass "scripts/lib/runtime.sh centralizes quickstart runtime artifact/env helpers"
  else
    fail "scripts/lib/runtime.sh must define shared runtime artifact/env helpers"
  fi

  for script in $required_scripts; do
    script_path="$STACK/scripts/$script"
    if grep -q 'lib/runtime.sh' "$script_path" && grep -q 'lineth_runtime_init' "$script_path"; then
      pass "scripts/$script uses shared runtime helper"
    else
      fail "scripts/$script must source scripts/lib/runtime.sh and call lineth_runtime_init"
    fi

    top_level="$(sed -n '1,140p' "$script_path")"
    duplicate_helpers="$(printf '%s\n' "$top_level" \
      | grep -nE '^(env_value|with_default|json_addr|artifact_path|shared_addr|shared_file_exists|is_uint|hex_to_dec_small)\(\)' \
      || true)"
    if [ -n "$duplicate_helpers" ]; then
      fail "scripts/$script redefines runtime helper(s) near top-level: $(printf '%s' "$duplicate_helpers" | tr '\n' ' ')"
    else
      pass "scripts/$script does not redefine shared runtime helpers at top-level"
    fi
  done
}

check_generated_genesis_is_volume_scoped() {
  compose="$STACK/docker-compose.yml"
  deploy_contracts="$STACK/scripts/phases/04-deploy-contracts.sh"

  if grep -q './artifacts/genesis:/initialization:rw' "$compose" \
    && grep -q './artifacts/genesis:/generated-genesis:ro' "$compose" \
    && ! grep -q 'linea-l2-genesis:' "$compose"; then
    pass "generated L2 genesis is host-backed under artifacts/genesis"
  else
    fail "generated L2 genesis must live in host artifacts/genesis, not a Docker volume"
  fi

  if grep -q './config/genesis:/templates:ro' "$compose" \
    && grep -q './artifacts/genesis:/initialization:rw' "$compose"; then
    pass "l2-genesis-init reads templates from repo and writes generated genesis to volume"
  else
    fail "l2-genesis-init must separate read-only templates from generated genesis output"
  fi

  old_genesis_mount='./config/''l2/genesis-init:/initialization'
  old_besu_genesis='./config/''l2/genesis-init/genesis-besu.json'
  if grep -q "$old_genesis_mount" "$compose" \
    || grep -q "$old_besu_genesis" "$compose"; then
    fail "services must not bind-mount generated genesis files from the repo tree"
  else
    pass "services do not bind-mount generated genesis files from the repo tree"
  fi

  if grep -q './artifacts/genesis:/generated-genesis:ro' "$compose" \
    && grep -q '/generated-genesis/fork-timestamp.txt' "$deploy_contracts"; then
    pass "deploy-contracts reads fork timestamp from generated genesis artifacts"
  else
    fail "deploy-contracts must read fork timestamp from generated genesis artifacts"
  fi
}

check_init_scripts_and_compose_shell() {
  compose="$STACK/docker-compose.yml"
  init_dir="$STACK/scripts/init"
  phases_dir="$STACK/scripts/phases"
  services_dir="$STACK/scripts/services"
  internal_dir="$STACK/scripts/internal"
  expected_init_scripts="coordinator-entrypoint.sh blockscout-entrypoint.sh"
  expected_phase_scripts="00-prepare-deploy-tools.sh 01-generate-accounts.sh 02-generate-l2-genesis.sh 03-render-service-configs.sh 04-deploy-contracts.sh 05-render-postdeploy-configs.sh"
  expected_service_scripts="render-l2-genesis.sh render-postman-config.sh render-coordinator-postdeploy-config.sh render-coordinator-config.sh render-maru-config.sh render-sequencer-config.sh render-l2-node-besu-config.sh render-prover-config.sh"
  expected_internal_files="account-setup.ts account-setup.test.ts quickstart-preflight.ts deployer-wallet.ts deployer-wallet.test.ts sepolia-policy.ts sepolia-policy.test.ts deploy-timing.ts aggregate-addresses.ts deployBridgedTokenAndTokenBridgeV1_1.ts ensure-demo-erc20.sh ensure-demo-erc20.ts fund-runtime-accounts.ts traffic-account.sh traffic-account.ts traffic-account.test.ts claim-l2-to-l1.ts claim-l2-to-l1.test.ts DEPLOY-ENV-CONTRACT.md"

  for script in $expected_init_scripts; do
    script_path="$init_dir/$script"
    if [ -x "$script_path" ]; then
      pass "scripts/init/$script is executable"
    else
      fail "scripts/init/$script must exist and be executable"
    fi

    if [ -f "$script_path" ] && sh -n "$script_path"; then
      pass "scripts/init/$script has valid shell syntax"
    elif [ -f "$script_path" ]; then
      fail "scripts/init/$script has invalid shell syntax"
    fi
  done

  for script in $expected_phase_scripts; do
    script_path="$phases_dir/$script"
    if [ -x "$script_path" ]; then
      pass "scripts/phases/$script is executable"
    else
      fail "scripts/phases/$script must exist and be executable"
    fi

    if [ "$script" = "04-deploy-contracts.sh" ]; then
      if [ -f "$script_path" ] && bash -n "$script_path"; then
        pass "scripts/phases/$script has valid bash syntax"
      elif [ -f "$script_path" ]; then
        fail "scripts/phases/$script has invalid bash syntax"
      fi
    elif [ -f "$script_path" ] && sh -n "$script_path"; then
      pass "scripts/phases/$script has valid shell syntax"
    elif [ -f "$script_path" ]; then
      fail "scripts/phases/$script has invalid shell syntax"
    fi
  done

  for script in $expected_service_scripts; do
    script_path="$services_dir/$script"
    if [ -x "$script_path" ]; then
      pass "scripts/services/$script is executable"
    else
      fail "scripts/services/$script must exist and be executable"
    fi

    if [ -f "$script_path" ] && sh -n "$script_path"; then
      pass "scripts/services/$script has valid shell syntax"
    elif [ -f "$script_path" ]; then
      fail "scripts/services/$script has invalid shell syntax"
    fi
  done

  config_render="$phases_dir/03-render-service-configs.sh"
  missing_renderer_calls=""
  for script in \
    render-coordinator-config.sh \
    render-maru-config.sh \
    render-sequencer-config.sh \
    render-l2-node-besu-config.sh \
    render-prover-config.sh; do
    if ! grep -q "/scripts/services/$script" "$config_render"; then
      missing_renderer_calls="$missing_renderer_calls $script"
    fi
  done
  if [ -z "$missing_renderer_calls" ]; then
    pass "phase 03 orchestrates per-service config renderers"
  else
    fail "phase 03 must call per-service renderers:$missing_renderer_calls"
  fi

  if grep -q './scripts/services:/scripts/services:ro' "$compose"; then
    pass "config-render can mount scripts/services"
  else
    fail "config-render must mount ./scripts/services:/scripts/services:ro"
  fi

  if command -v shellcheck >/dev/null 2>&1; then
    if shellcheck "$init_dir"/*.sh "$phases_dir"/*.sh "$services_dir"/*.sh "$internal_dir"/*.sh "$STACK/scripts/lib"/*.sh; then
      pass "quickstart implementation shell scripts pass shellcheck"
    else
      fail "quickstart implementation shell scripts must pass shellcheck"
    fi
  else
    pass "shellcheck not installed; skipped implementation shellcheck"
  fi

  for file in $expected_internal_files; do
    file_path="$internal_dir/$file"
    if [ -f "$file_path" ]; then
      pass "scripts/internal/$file is present"
    else
      fail "scripts/internal/$file must exist"
    fi
  done

  if node - "$compose" <<'NODE'
const fs = require("fs");
const compose = process.argv[2];
const lines = fs.readFileSync(compose, "utf8").split(/\n/);
let current = null;
let failures = [];

for (let index = 0; index < lines.length; index++) {
  const line = lines[index];
  const key = line.match(/^(\s+)(entrypoint|command):\s*(.*)$/);
  if (key) {
    current = { name: key[2], indent: key[1].length };
    if (/^[|>]/.test(key[3].trim())) {
      failures.push(`${index + 1}: ${line}`);
    }
    continue;
  }

  if (!current || /^\s*$/.test(line)) continue;
  const indent = (line.match(/^(\s*)/) || ["", ""])[1].length;
  if (indent <= current.indent) {
    current = null;
    continue;
  }
  if (/^\s*-\s*[|>]$/.test(line) || /^\s*[|>]$/.test(line)) {
    failures.push(`${index + 1}: ${line}`);
  }
}

if (failures.length > 0) {
  console.error(failures.join("\n"));
  process.exit(1);
}
NODE
  then
    pass "docker-compose service entrypoint/command blocks do not contain inline shell bodies"
  else
    fail "docker-compose service entrypoint/command must not contain inline shell bodies"
  fi

  startup_shell="$(grep -nE 'sh -c|bash -c|-[[:space:]]+-c' "$compose" | grep -v 'CMD-SHELL' || true)"
  if [ -n "$startup_shell" ]; then
    fail "docker-compose service startup must not use sh -c/bash -c: $(printf '%s' "$startup_shell" | tr '\n' ' ')"
  else
    pass "docker-compose service startup does not use sh -c/bash -c"
  fi
}

check_generated_l2_deployer_genesis() {
  genesis_template="$STACK/config/genesis/genesis-besu.json.template"
  genesis_init="$STACK/scripts/services/render-l2-genesis.sh"

  if grep -q '__L2_DEPLOYER_ADDRESS__' "$genesis_template" && grep -q '__L2_DEPLOYER_ADDRESS__' "$genesis_init"; then
    pass "generated L2 deployer is injected into L2 genesis"
  else
    fail "generated L2 deployer must be injected into L2 genesis"
  fi

  if node - "$genesis_template" <<'NODE'
const fs = require("fs");
const template = fs.readFileSync(process.argv[2], "utf8")
  .replace(/__L2_CHAIN_ID__/g, "1337");
const genesis = JSON.parse(template);
const funded = Object.entries(genesis.alloc)
  .filter(([, entry]) => BigInt(entry.balance || "0") > 0n)
  .map(([addr]) => addr)
  .sort();
const expected = ["__L2_DEPLOYER_ADDRESS__", "__L2_MESSAGE_SERVICE_ADDRESS__"].sort();
if (JSON.stringify(funded) !== JSON.stringify(expected)) {
  console.error(`funded=${funded.join(",")}`);
  process.exit(1);
}
for (const [addr, entry] of Object.entries(genesis.alloc)) {
  if (Object.prototype.hasOwnProperty.call(entry, "privateKey")) {
    console.error(`privateKey still present in genesis alloc for ${addr}`);
    process.exit(1);
  }
}
NODE
  then
    pass "genesis only funds generated L2 deployer and L2MessageService"
  else
    fail "genesis must only fund generated L2 deployer and L2MessageService, with no privateKey entries"
  fi
}

check_l2_chain_id_wiring() {
  besu_genesis="$STACK/config/genesis/genesis-besu.json.template"
  maru_genesis="$STACK/config/genesis/genesis-maru.json.template"
  genesis_init="$STACK/scripts/services/render-l2-genesis.sh"
  prover_template="$STACK/config/services/prover/prover-config-partial.toml.template"
  compose="$STACK/docker-compose.yml"
  status_script="$STACK/scripts/status.sh"

  if grep -q '"chainId": __L2_CHAIN_ID__' "$besu_genesis" \
    && grep -q '"chainId": __L2_CHAIN_ID__' "$maru_genesis" \
    && grep -qF 's/__L2_CHAIN_ID__/$L2_CHAIN_ID/g' "$genesis_init"; then
    pass "L2_CHAIN_ID is templated into Besu and Maru genesis"
  else
    fail "L2_CHAIN_ID must be templated into both L2 genesis files"
  fi

  config_render="$STACK/scripts/phases/03-render-service-configs.sh"
  render_prover_config="$STACK/scripts/services/render-prover-config.sh"

  if grep -q '^chain_id = __L2_CHAIN_ID__' "$prover_template" \
    && grep -qF 's|__L2_CHAIN_ID__|$L2_CHAIN_ID|g' "$render_prover_config"; then
    pass "L2_CHAIN_ID is templated into prover public input config"
  else
    fail "L2_CHAIN_ID must feed prover-config-partial.toml.template"
  fi

  if grep -qF 'L2_CHAIN_ID: ${L2_CHAIN_ID:-1337}' "$compose" \
    && grep -qF 'NEXT_PUBLIC_NETWORK_ID: ${L2_CHAIN_ID:-1337}' "$compose" \
    && grep -qF 'CHAIN_ID: ${L2_CHAIN_ID:-1337}' "$compose"; then
    pass "docker compose propagates L2_CHAIN_ID to init, deploy, and explorer"
  else
    fail "docker compose must propagate L2_CHAIN_ID to init/deploy/explorer"
  fi

  if grep -q 'eth_chainId' "$status_script" \
    && grep -qF 'printf "%s.%s\n", section, $0' "$status_script"; then
    pass "status.sh reports section-qualified L2 chain ID and rendered prover mode"
  else
    fail "status.sh must report section-qualified L2 chain ID and rendered prover mode"
  fi

  if grep -q 'l1 data availability vs finalization' "$status_script" \
    && grep -q 'latest blob tx (DA only)' "$status_script" \
    && grep -q 'finalizeBlocks(bytes,uint256,tuple)' "$status_script" \
    && grep -q 'currentL2BlockNumber' "$status_script" \
    && grep -q 'DataFinalizedV3' "$status_script" \
    && grep -q 'FinalizedStateUpdated' "$status_script"; then
    pass "status.sh separates blob submission from rollup finalization"
  else
    fail "status.sh must distinguish blob submission from finalizeBlocks finalization"
  fi
}

check_account_setup_key_model() {
  account_setup="$STACK/scripts/phases/01-generate-accounts.sh"
  account_setup_ts="$STACK/scripts/internal/account-setup.ts"
  account_setup_test="$STACK/scripts/internal/account-setup.test.ts"

  for field in \
    l1BlobSubmitterAddress \
    l1FinalizationSubmitterAddress \
    l1PostmanAddress \
    l2DeployerAddress \
    l2MessageAnchoringAddress \
    l2PostmanAddress; do
    if grep -Eq "\"?$field\"?[[:space:]]*:" "$account_setup_ts"; then
      pass "addresses-precomputed.json includes signers.$field"
    else
      fail "addresses-precomputed.json must include signers.$field"
    fi
  done

  for contract in LineaRollupV8 L2MessageService; do
    if grep -Eq "\"?$contract\"?[[:space:]]*:" "$account_setup_ts"; then
      pass "addresses-precomputed.json includes boot-critical $contract"
    else
      fail "addresses-precomputed.json must include boot-critical $contract"
    fi
  done

  sepolia_policy_ts="$STACK/scripts/internal/sepolia-policy.ts"
  if grep -q 'l1AccountSetupBlockNumber' "$account_setup_ts" \
    && grep -q 'l1PostmanListenerStartBlock' "$account_setup_ts" \
    && grep -q 'getBlockNumber' "$sepolia_policy_ts"; then
    pass "addresses-precomputed.json includes safe Postman L1 listener start block"
  else
    fail "addresses-precomputed.json must include a safe Postman L1 listener start block"
  fi

  for contract in Verifier ForcedTransactionGateway BridgedToken TokenBridge TestERC20; do
    if grep -Eq "\"?$contract\"?[[:space:]]*:" "$account_setup_ts"; then
      fail "addresses-precomputed.json should not precompute non-boot contract $contract"
    else
      pass "addresses-precomputed.json does not precompute non-boot contract $contract"
    fi
  done

  if grep -q 'L2_DEPLOYER_PRIVATE_KEY:=0x' "$account_setup" "$account_setup_ts"; then
    fail "account-setup still defaults to a pre-baked L2 deployer private key"
  else
    pass "account-setup does not default to a pre-baked L2 deployer private key"
  fi

  if grep -q 'L2_MESSAGE_ANCHORING_PRIVATE_KEY:=0x' "$account_setup" "$account_setup_ts"; then
    fail "account-setup still defaults to a pre-baked L2 anchorer private key"
  else
    pass "account-setup does not default to a pre-baked L2 anchorer private key"
  fi

  if grep -q 'L2_LIVENESS_SIGNER_PRIVATE_KEY\|write_keystore "liveness-signer"' "$account_setup" "$account_setup_ts"; then
    fail "account-setup still generates or persists a liveness signer"
  else
    pass "account-setup does not generate a liveness signer"
  fi

  if grep -q '/accounts/runtime-keys.env\|OUT_RUNTIME_KEYS_ENV' "$account_setup_ts" \
    && grep -q 'runtime-keystores' "$account_setup_ts" \
    && grep -q 'Wallet.createRandom' "$account_setup_ts" \
    && grep -q 'encryptKeystoreJson' "$account_setup_ts" \
    && grep -q 'LINETH_KEYSTORE_PASSWORD' "$account_setup_ts" \
    && grep -q 'DEFAULT_RUNTIME_PASSWORD = "linea-local-dev"' "$account_setup_ts"; then
    pass "account-setup persists generated private keys for retry-safe consumers"
  else
    fail "account-setup must use ethers to generate encrypted runtime keystores with a non-empty Web3Signer-compatible password"
  fi

  if grep -q '0o644' "$account_setup_ts" && grep -q 'runtime-keys.env' "$account_setup_ts"; then
    pass "runtime key env is readable by non-root deploy/smoke containers"
  else
    fail "account-setup must chmod runtime-keys.env so deploy/smoke containers can read generated keys"
  fi

  if grep -q '0o644' "$account_setup_ts" && grep -q 'anchoring-signer' "$account_setup_ts"; then
    pass "web3signer key files are readable by the web3signer container"
  else
    fail "account-setup must chmod web3signer key files so web3signer can load generated signers"
  fi

  if grep -q 'type: "file-keystore"' "$account_setup_ts" \
    && grep -q 'keyType: "SECP256K1"' "$account_setup_ts" \
    && grep -q 'keystoreFile:' "$account_setup_ts" \
    && grep -q 'keystorePasswordFile:' "$account_setup_ts" \
    && grep -q 'OUT_KEYSTORE_PASSWORD_FILE' "$account_setup_ts" \
    && ! grep -q 'type: "file-raw"' "$account_setup_ts"; then
    pass "web3signer loads generated runtime signers from encrypted keystores"
  else
    fail "account-setup must generate Web3Signer file-keystore configs, not file-raw private-key configs"
  fi
}

check_typescript_quickstart_helpers() {
  account_setup="$STACK/scripts/phases/01-generate-accounts.sh"
  account_setup_ts="$STACK/scripts/internal/account-setup.ts"
  deployer_wallet_ts="$STACK/scripts/internal/deployer-wallet.ts"
  deployer_wallet_test="$STACK/scripts/internal/deployer-wallet.test.ts"
  preflight_ts="$STACK/scripts/internal/quickstart-preflight.ts"
  sepolia_policy_ts="$STACK/scripts/internal/sepolia-policy.ts"
  sepolia_policy_test="$STACK/scripts/internal/sepolia-policy.test.ts"
  quickstart_invariants_ts="$STACK/scripts/internal/quickstart-invariants.ts"
  quickstart_invariants_test="$STACK/scripts/internal/quickstart-invariants.test.ts"
  deploy_timing_ts="$STACK/scripts/internal/deploy-timing.ts"
  deploy_contracts="$STACK/scripts/phases/04-deploy-contracts.sh"
  compose="$STACK/docker-compose.yml"
  env_example="$STACK/.env.example"
  gas_profile="$STACK/profiles/gas-sepolia.env.example"

  if grep -q 'account-setup.ts' "$account_setup" \
    && grep -q 'JsonRpcProvider' "$account_setup_ts" \
    && grep -q 'computeBootPrecomputedAddresses' "$account_setup_ts" \
    && grep -q 'runL1PolicyCheck' "$account_setup_ts" \
    && grep -q 'resolveL1DeployerConfig' "$account_setup_ts" \
    && grep -q 'buildPrecomputedAddressPlan' "$account_setup_ts" \
    && grep -q 'addresses-precomputed.json' "$account_setup_ts" \
    && grep -q 'require.main === module' "$account_setup_ts" \
    && ! grep -q 'function checkGasCaps' "$account_setup_ts" \
    && ! grep -q 'maybeEnvBigInt' "$account_setup_ts"; then
    pass "account setup delegates wallet/RPC/gas policy to shared TypeScript"
  else
    fail "account setup must delegate L1 gas/balance/nonce policy to sepolia-policy.ts"
  fi

  if [ -f "$account_setup_test" ] \
    && grep -q 'reuses existing precomputed addresses when current deployer nonce is higher' "$account_setup_test" \
    && grep -q 'rejects incompatible existing runtime signer addresses' "$account_setup_test" \
    && grep -q 'rejects incompatible existing deterministic deploy addresses' "$account_setup_test"; then
    pass "account-setup tests restart-safe precomputed address reuse"
  else
    fail "account-setup.test.ts must cover restart-safe precomputed address reuse and incompatible artifact failures"
  fi

  if [ -f "$quickstart_invariants_ts" ] \
    && [ -f "$quickstart_invariants_test" ] \
    && grep -q 'computeGenesisShnarf' "$quickstart_invariants_ts" \
    && grep -q 'computeBootPrecomputedAddresses' "$quickstart_invariants_ts" \
    && grep -q 'genesis-shnarf' "$quickstart_invariants_ts" \
    && grep -q '0x6c66ebb91228a0e9f41ec5060e5b6fdf4d8310db928e3b84b2d2e609b426bd8c' "$quickstart_invariants_test" \
    && grep -q '0xDc64a140Aa3E981100a9becA4E685f962f0cF6C9' "$quickstart_invariants_test" \
    && grep -q '0x9A676e781A523b5d0C0e43731313A708CB607508' "$quickstart_invariants_test" \
    && grep -q '0x948B3c65b89DF0B4894ABE91E6D02FE579834F8F' "$quickstart_invariants_test"; then
    pass "quickstart shnarf and boot address invariants are centralized and tested"
  else
    fail "quickstart-invariants.ts/test must centralize genesis shnarf and boot address precompute"
  fi

  if grep -q 'runL1PolicyCheck' "$preflight_ts" \
    && grep -q 'resolveL1DeployerConfig' "$preflight_ts" \
    && grep -q 'readDotEnvFile' "$preflight_ts" \
    && grep -q 'JsonRpcProvider' "$preflight_ts" \
    && grep -q 'Sepolia deployer funding required before Docker startup' "$preflight_ts" \
    && ! grep -q 'function requireCapAbove' "$preflight_ts" \
    && ! grep -q 'eth_blobBaseFee' "$preflight_ts" \
    && grep -q 'quickstart-preflight.ts' "$STACK/scripts/start.sh"; then
    pass "quickstart preflight uses shared L1 policy"
  else
    fail "quickstart preflight must use sepolia-policy.ts instead of owning gas-cap logic"
  fi

  if awk '
    /^run_ts_preflight$/ { preflight = NR }
    /Pull Docker images/ { pull = NR }
    /Start services/ { up = NR }
    END { exit !(preflight && pull && up && preflight < pull && preflight < up) }
  ' "$STACK/scripts/start.sh"; then
    pass "start.sh runs host deployer preflight before Docker pull/start"
  else
    fail "start.sh must run host deployer preflight before Docker pull/start"
  fi

  if grep -q 'resolveL1DeployerConfig' "$deployer_wallet_ts" \
    && grep -q 'generated-keystore' "$deployer_wallet_ts" \
    && grep -q 'provided-keystore' "$deployer_wallet_ts" \
    && grep -q 'legacy-raw-key' "$deployer_wallet_ts" \
    && grep -q 'localL1HostRpcUrl' "$deployer_wallet_ts" \
    && grep -q 'HOST_PORT_L1_RPC' "$deployer_wallet_ts" \
    && grep -q 'emit-shell-env' "$deployer_wallet_ts" \
    && grep -q 'generates default Sepolia deployer keystore when missing' "$deployer_wallet_test" \
    && grep -q 'local mode host RPC honors HOST_PORT_L1_RPC override' "$deployer_wallet_test" \
    && grep -q 'deprecated raw private key compatibility still works' "$deployer_wallet_test"; then
    pass "shared deployer resolver supports generated, provided, local, and deprecated raw-key sources"
  else
    fail "deployer-wallet.ts must own all L1 deployer source selection"
  fi

  if grep -q -- '--forget-deployer' "$STACK/scripts/reset.sh" \
    && grep -q 'PRESERVED_DEPLOYER_DIR' "$STACK/scripts/reset.sh" \
    && grep -q 'deployer-keystore' "$STACK/scripts/reset.sh" \
    && ! grep -q '^L1_DEPLOYER_PRIVATE_KEY=' "$STACK/.env.example" \
    && ! grep -q '^# L1_DEPLOYER_PRIVATE_KEY=' "$STACK/.env.example"; then
    pass "reset preserves generated Sepolia deployer by default and .env.example omits raw deployer key"
  else
    fail "reset must preserve generated deployer by default and .env.example must not expose raw deployer key"
  fi

  if grep -q 'SEPOLIA_POLICY_DEFAULTS' "$sepolia_policy_ts" \
    && grep -q 'runSepoliaPolicyCheck' "$sepolia_policy_ts" \
    && grep -q 'sanitizeExternalError' "$sepolia_policy_ts" \
    && grep -q 'L1_DEPLOY_GAS_PRICE_WEI: "5000000000"' "$sepolia_policy_ts" \
    && grep -q 'L1_BLOB_MAX_FEE_PER_GAS_CAP_WEI: "100000000000"' "$sepolia_policy_ts" \
    && grep -q 'L1_FINALIZATION_MAX_FEE_PER_GAS_CAP_WEI: "200000000000"' "$sepolia_policy_ts" \
    && grep -q 'L2_GAS_PRICE_WEI: "100000000"' "$sepolia_policy_ts" \
    && grep -q 'policy defaults match compose and examples' "$sepolia_policy_test" \
    && grep -q 'blob base fee unavailable warns without leaking URLs' "$sepolia_policy_test"; then
    pass "Sepolia defaults and mocked policy tests are centralized"
  else
    fail "sepolia-policy.ts must own defaults and mocked policy tests"
  fi

  if grep -q 'quickstart-invariants.ts genesis-shnarf' "$deploy_contracts" \
    && grep -q 'contracts/test/hardhat/common/helpers/dataGeneration.ts::computeGenesisShnarf' "$deploy_contracts" \
    && ! grep -q 'verify against V8' "$deploy_contracts"; then
    pass "deploy-contracts computes genesis shnarf through the tested invariant helper"
  else
    fail "deploy-contracts must use quickstart-invariants.ts for genesis shnarf and not keep unverified V8 wording"
  fi

  default_drift=false
  for spec in \
    "L1_DEPLOYER_MIN_BALANCE_WEI=2000000000000000000" \
    "L1_DEPLOY_GAS_PRICE_WEI=5000000000" \
    "L1_ROLE_MIN_BALANCE_WEI=400000000000000000" \
    "L1_ROLE_TOP_UP_WEI=500000000000000000" \
    "L1_POSTMAN_MIN_BALANCE_WEI=50000000000000000" \
    "L1_POSTMAN_TOP_UP_WEI=100000000000000000" \
    "L1_DYNAMIC_GAS_PRICE_CAP_DISABLED=true" \
    "L1_BLOB_MAX_FEE_PER_GAS_CAP_WEI=100000000000" \
    "L1_BLOB_MAX_FEE_PER_BLOB_GAS_CAP_WEI=100000000000" \
    "L1_BLOB_MAX_PRIORITY_FEE_PER_GAS_CAP_WEI=20000000000" \
    "L1_FINALIZATION_MAX_FEE_PER_GAS_CAP_WEI=200000000000" \
    "L1_FINALIZATION_MAX_PRIORITY_FEE_PER_GAS_CAP_WEI=40000000000"; do
    name="${spec%%=*}"
    value="${spec#*=}"
    if ! grep -q "${name}: \"${value}\"" "$sepolia_policy_ts" \
      || ! grep -qF "${name}: \${${name}:-${value}}" "$compose" \
      || ! grep -qF "# ${name}=${value}" "$env_example" \
      || ! grep -qF "${name}=${value}" "$gas_profile"; then
      default_drift=true
    fi
  done
  if [ "$default_drift" = "false" ] \
    && grep -q 'L2_GAS_PRICE_WEI: "100000000"' "$sepolia_policy_ts" \
    && grep -qF 'L2_GAS_PRICE_WEI: ${L2_GAS_PRICE_WEI:-100000000}' "$compose" \
    && grep -qF '# L2_GAS_PRICE_WEI=100000000' "$env_example"; then
    pass "Compose, .env examples, and sepolia-policy defaults stay in sync"
  else
    fail "Compose, .env examples, gas profile, and sepolia-policy defaults must not drift"
  fi

  if grep -q 'deploy-timing.ts' "$deploy_contracts" \
    && grep -q 'DEPLOY_TIMING_PATH' "$deploy_contracts" \
    && grep -q 'appendFileSync' "$deploy_timing_ts" \
    && grep -q 'deploy-timing.jsonl' "$deploy_timing_ts"; then
    pass "deploy-contracts emits a TypeScript-backed deploy timing report"
  else
    fail "deploy-contracts must emit a TypeScript-backed deploy timing report"
  fi
}

check_postman_key_model() {
  postman_env="$STACK/config/services/postman/postman.env"
  compose="$STACK/docker-compose.yml"
  postman_entrypoint="$STACK/scripts/init/postman-entrypoint.sh"
  render_postman_env="$STACK/scripts/services/render-postman-config.sh"
  bootstrap_artifacts="$STACK/scripts/bootstrap-artifacts.sh"
  known_clients="$STACK/config/web3signer/tls-files/known-clients.txt"
  sequencer_template="$STACK/config/services/sequencer/sequencer.config.toml.template"
  postman_section="$(awk '
    /^  postman:/ { in_postman = 1 }
    /^  [A-Za-z0-9_-]+:/ && $1 != "postman:" { in_postman = 0 }
    in_postman { print }
  ' "$compose")"

  if grep -q '^L2_SIGNER_PRIVATE_KEY=0x' "$postman_env"; then
    fail "postman env still contains a pre-baked L2 signer private key"
  else
    pass "postman env does not contain a pre-baked L2 signer private key"
  fi

  if grep -q 'L1_WEB3_SIGNER_PUBLIC_KEY' "$render_postman_env" \
    && grep -q 'l1PostmanPubkey' "$render_postman_env" \
    && grep -q "L1_SIGNER_TYPE='web3signer'" "$render_postman_env" \
    && grep -q "L1_RPC_URL='%s'" "$render_postman_env" \
    && grep -q 'postman-config-render:' "$compose" \
    && grep -q './artifacts/config/postman:/postman-runtime:rw' "$compose" \
    && grep -q './artifacts/config/postman/postman.env' "$compose"; then
    pass "postman-config-render renders L1 RPC and generated L1 Web3Signer config into host artifacts"
  else
    fail "postman-config-render must render L1 RPC and L1 Web3Signer config into artifacts/config/postman/postman.env"
  fi

  if grep -q 'L2_WEB3_SIGNER_PUBLIC_KEY' "$render_postman_env" \
    && grep -q 'l2PostmanPubkey' "$render_postman_env" \
    && grep -q "L2_SIGNER_TYPE='web3signer'" "$render_postman_env" \
    && grep -q 'postman-config-render:' "$compose" \
    && grep -q './artifacts/config/postman:/postman-runtime:rw' "$compose" \
    && grep -q './artifacts/config/postman/postman.env' "$compose"; then
    pass "postman-config-render renders generated L2 Web3Signer config into host artifacts"
  else
    fail "postman-config-render must render L2 Web3Signer config into artifacts/config/postman/postman.env"
  fi

  if grep -q 'SIGNER_PRIVATE_KEY' "$render_postman_env"; then
    fail "postman-config-render must not render raw postman private keys"
  else
    pass "postman-config-render does not render raw postman private keys"
  fi

  if grep -q 'postman-client-keystore.p12' "$render_postman_env" \
    && grep -q '^postman ' "$known_clients" \
    && [ -f "$STACK/config/services/postman/tls-files/postman-client-keystore.p12" ] \
    && [ -f "$STACK/config/services/postman/tls-files/web3signer-truststore.p12" ] \
    && printf '%s\n' "$postman_section" | grep -q './config/services/postman/tls-files:/tls-files:ro' \
    && ! printf '%s\n' "$postman_section" | grep -q './config/services/coordinator/tls-files:/tls-files:ro'; then
    pass "postman uses its own mTLS client identity for Web3Signer"
  else
    fail "postman must use config/services/postman/tls-files and be listed in Web3Signer known-clients.txt"
  fi

  if grep -q 'addresses-precomputed.json' "$render_postman_env" \
    && grep -q 'l1PostmanListenerStartBlock' "$render_postman_env" \
    && ! grep -q 'deploy-runtime.env\|addresses.json\|deploy-logs\|step1-linea-rollup' "$render_postman_env"; then
    pass "postman config renders from early precomputed artifacts only"
  else
    fail "render-postman-config.sh must use addresses-precomputed.json only for deploy facts"
  fi

  if ! printf '%s\n' "$postman_section" | grep -q 'postman-config-render:' \
    && printf '%s\n' "$postman_section" | grep -q 'deploy-contracts:' \
    && ! printf '%s\n' "$postman_section" | grep -q 'runtime-config-finalize:'; then
    pass "postman waits for deploy completion and uses pre-created host env"
  else
    fail "postman must depend on deploy-contracts only; postman env is generated before compose up"
  fi

  if [ ! -e "$postman_entrypoint" ] \
    && ! printf '%s\n' "$postman_section" | grep -q 'entrypoint:' \
    && ! printf '%s\n' "$postman_section" | grep -q '/postman-runtime\|./scripts/init:/scripts/init' \
    && ! printf '%s\n' "$postman_section" | grep -q 'linea-shared-config:/shared'; then
    pass "postman has no custom entrypoint and consumes generated host env_file"
  else
    fail "postman must not use postman-entrypoint.sh, /postman-runtime, scripts/init, or /shared"
  fi

  if grep -q 'bootstrap-artifacts.sh' "$STACK/scripts/start.sh" \
    && grep -q 'postman-config-render' "$bootstrap_artifacts" \
    && grep -q 'artifacts/config/postman/postman.env' "$bootstrap_artifacts" \
    && grep -q 'artifacts/accounts/runtime-keystores' "$bootstrap_artifacts" \
    && grep -q 'artifacts/deployments/deploy-logs' "$bootstrap_artifacts" \
    && ! grep -q 'linea-postman-runtime-config' "$compose"; then
    pass "start.sh bootstraps generated Postman env before compose up"
  else
    fail "start.sh must bootstrap host artifacts before compose up and compose must not use linea-postman-runtime-config"
  fi

  if printf '%s\n' "$postman_section" | grep -q 'L1_RPC_URL:'; then
    fail "postman service must not override L1_RPC_URL; render it into postman.env instead"
  else
    pass "postman service does not override L1_RPC_URL"
  fi

  if grep -q '^plugin-linea-liveness-enabled=false' "$sequencer_template"; then
    pass "sequencer liveness is disabled for quickstart"
  else
    fail "sequencer liveness must be disabled for quickstart"
  fi

  extra_liveness_config="$(grep '^plugin-linea-liveness-' "$sequencer_template" | grep -v '^plugin-linea-liveness-enabled=false$' || true)"
  if [ -n "$extra_liveness_config" ]; then
    fail "sequencer template must not carry inactive liveness signer/contract settings"
  else
    pass "sequencer liveness config contains only enabled=false"
  fi

  if grep -q 'net.consensys.linea.sequencer.liveness' "$STACK/config/services/sequencer/log4j.xml"; then
    fail "sequencer log4j should not enable liveness-specific logging when liveness is disabled"
  else
    pass "sequencer log4j has no liveness-specific logger"
  fi
}

check_incremental_typescript_helpers() {
  deploy_contracts="$STACK/scripts/phases/04-deploy-contracts.sh"
  aggregate_ts="$STACK/scripts/internal/aggregate-addresses.ts"

  if [ -f "$aggregate_ts" ] && grep -q 'ts-node /scripts/internal/aggregate-addresses.ts' "$deploy_contracts"; then
    pass "addresses.json aggregation is implemented as an incremental TypeScript helper"
  else
    fail "deploy-contracts should call scripts/internal/aggregate-addresses.ts for addresses.json aggregation"
  fi
}

check_partial_prover_guardrails() {
  compose="$STACK/docker-compose.yml"
  config_render="$STACK/scripts/phases/03-render-service-configs.sh"
  render_prover_config="$STACK/scripts/services/render-prover-config.sh"
  prover_template="$STACK/config/services/prover/prover-config-partial.toml.template"
  coordinator_template="$STACK/config/services/coordinator/coordinator-config.toml.template"
  env_example="$STACK/.env.example"
  readme="$STACK/README.md"

  if grep -q 'PROVER_GOMEMLIMIT must be set explicitly when PROVER_DEV_OVERRIDE=false' "$render_prover_config"; then
    pass "partial prover mode requires explicit PROVER_GOMEMLIMIT"
  else
    fail "PROVER_DEV_OVERRIDE=false must require explicit PROVER_GOMEMLIMIT"
  fi

  if grep -q '^prover_mode = "partial"' "$prover_template" \
    && grep -q '^is_allowed_circuit_id = 483' "$prover_template" \
    && grep -q 'partial render verified: 2x partial, 2x dev, is_allowed_circuit_id=483' "$render_prover_config"; then
    pass "partial prover render is verified before prover starts"
  else
    fail "config-render must verify partial prover modes and bitmask"
  fi

  if grep -q '__L1_BLOB_MAX_FEE_PER_BLOB_GAS_CAP_WEI__' "$coordinator_template" \
    && grep -q '__L1_FINALIZATION_MAX_FEE_PER_GAS_CAP_WEI__' "$coordinator_template" \
    && grep -q 'L1_BLOB_MAX_FEE_PER_BLOB_GAS_CAP_WEI' "$compose" \
    && grep -q 'L1_FINALIZATION_MAX_FEE_PER_GAS_CAP_WEI' "$compose" \
    && grep -q 'L1_BLOB_MAX_FEE_PER_BLOB_GAS_CAP_WEI' "$env_example" \
    && grep -q 'L1_FINALIZATION_MAX_FEE_PER_GAS_CAP_WEI' "$env_example" \
    && grep -q 'L1 gas caps and Sepolia congestion' "$readme"; then
    pass "L1 blob/finalization gas caps are documented and configurable"
  else
    fail "L1 blob/finalization gas caps must be documented and configurable"
  fi
}

check_runtime_config_and_validium_guardrails() {
  compose="$STACK/docker-compose.yml"
  deploy_contracts="$STACK/scripts/phases/04-deploy-contracts.sh"
  config_render="$STACK/scripts/phases/03-render-service-configs.sh"
  coordinator_entrypoint="$STACK/scripts/init/coordinator-entrypoint.sh"
  render_coordinator_config="$STACK/scripts/services/render-coordinator-config.sh"
  finalize_coordinator="$STACK/scripts/services/render-coordinator-postdeploy-config.sh"
  render_postman_env="$STACK/scripts/services/render-postman-config.sh"
  runtime_finalize="$STACK/scripts/phases/05-render-postdeploy-configs.sh"
  coordinator_template="$STACK/config/services/coordinator/coordinator-config.toml.template"
  validium_assignment='LINEA_COORDINATOR_DATA_AVAILABILITY=VAL''IDIUM'

  bad_private_keys="$(grep -n 'private-key = "0x' "$coordinator_template" \
    | grep -v 'private-key = "0x0000000000000000000000000000000000000000000000000000000000000000"' || true)"
  if [ -z "$bad_private_keys" ]; then
    pass "coordinator template contains only zero placeholders for inactive web3j private keys"
  else
    fail "coordinator template must not contain real-looking web3j private keys: $(printf '%s' "$bad_private_keys" | tr '\n' ' ')"
  fi

  if grep -q 'LINEA_COORDINATOR_DATA_AVAILABILITY: ${LINEA_COORDINATOR_DATA_AVAILABILITY:-ROLLUP}' "$compose" \
    && grep -q 'is not supported by this quickstart; use ROLLUP' "$config_render" \
    && grep -q 'is not supported by this quickstart; use ROLLUP' "$deploy_contracts" \
    && grep -q 'data availability mode' "$coordinator_entrypoint" \
    && ! grep -q "$validium_assignment" "$STACK/README.md" \
    && ! grep -q "$validium_assignment" "$STACK/.env.example" \
    && ! find "$STACK/profiles" -name '*validium*' -print | grep -q .; then
    pass "validium mode fails loudly and is not advertised as a quickstart option"
  else
    fail "validium mode must fail loudly and must not be advertised as a quickstart option"
  fi

  if grep -q 'coordinator-config.predeploy.toml' "$render_coordinator_config" \
    && grep -q 'coordinator-config.predeploy.toml' "$finalize_coordinator" \
    && grep -q 'coordinator-config.toml' "$finalize_coordinator" \
    && grep -q 'GENESIS_STATE_ROOT_HASH' "$finalize_coordinator" \
    && grep -q 'L2_MESSAGE_SERVICE_DEPLOY_BLOCK' "$finalize_coordinator"; then
    pass "coordinator final config is rendered from predeploy config plus deploy-runtime.env"
  else
    fail "runtime finalizer must render final coordinator config from predeploy config plus deploy-runtime.env"
  fi

  if grep -q 'DEPLOY_RUNTIME_ENV' "$deploy_contracts" \
    && grep -q 'GENESIS_STATE_ROOT_HASH' "$deploy_contracts" \
    && grep -q 'GENESIS_SHNARF' "$deploy_contracts" \
    && grep -q 'L2_MESSAGE_SERVICE_DEPLOY_BLOCK' "$deploy_contracts" \
    && grep -q 'LINEA_ROLLUP_L1_DEPLOY_BLOCK' "$deploy_contracts" \
    && ! grep -q 'Patched coordinator-config.toml\|coordinator-config.toml.new' "$deploy_contracts"; then
    pass "deploy-contracts writes deploy-runtime.env without patching rendered coordinator config"
  else
    fail "deploy-contracts must write deploy-runtime.env and must not patch rendered coordinator config"
  fi

  if grep -q 'POSTMAN_ENV' "$render_postman_env" \
    && grep -q 'l1PostmanListenerStartBlock' "$render_postman_env" \
    && grep -q 'L1_WEB3_SIGNER_PUBLIC_KEY' "$render_postman_env" \
    && grep -q 'L2_WEB3_SIGNER_PUBLIC_KEY' "$render_postman_env" \
    && grep -q 'render-coordinator-postdeploy-config.sh' "$runtime_finalize" \
    && ! grep -q 'render-postman-config.sh' "$runtime_finalize" \
    && grep -q 'runtime-config-finalize:' "$compose" \
    && grep -q 'postman-config-render:' "$compose"; then
    pass "postman config renders early and runtime-config-finalize is coordinator-only"
  else
    fail "postman must render early and runtime-config-finalize must be coordinator-only"
  fi
}

check_reuse_guardrails() {
  account_setup="$STACK/scripts/phases/01-generate-accounts.sh"
  account_setup_ts="$STACK/scripts/internal/account-setup.ts"
  compose="$STACK/docker-compose.yml"
  deploy_contracts="$STACK/scripts/phases/04-deploy-contracts.sh"
  rollup_deploy_script="$ROOT/contracts/local-deployments-artifacts/deployPlonkVerifierAndLineaRollupV8.ts"
  ensure_demo_erc20="$STACK/scripts/internal/ensure-demo-erc20.ts"
  ensure_demo_erc20_sh="$STACK/scripts/internal/ensure-demo-erc20.sh"
  traffic_account_sh="$STACK/scripts/internal/traffic-account.sh"
  fund_runtime_accounts="$STACK/scripts/internal/fund-runtime-accounts.ts"
  status_script="$STACK/scripts/status.sh"
  logging_lib="$STACK/scripts/lib/logging.sh"

  if grep -q 'step_already_done_with_code' "$deploy_contracts" \
    && grep -q 'cast code' "$deploy_contracts" \
    && grep -q 'verify_address "$address" "$expected_address" "$label"' "$deploy_contracts" \
    && grep -q 'present but no code' "$deploy_contracts" \
    && grep -q 'step_already_done_with_code "$logfile" "$primary_contract" "$L1_RPC_URL" "L1" "$PRECOMPUTED_LINEA_ROLLUP"' "$deploy_contracts" \
    && grep -q 'step_already_done_with_code "$logfile" "L2MessageService" "$L2_RPC_URL" "L2" "$PRECOMPUTED_L2_MS"' "$deploy_contracts" \
    && grep -q 'step_already_done_with_code "$logfile" "TokenBridge" "$L1_RPC_URL" "L1" "$EXPECTED_L1_TOKEN_BRIDGE"' "$deploy_contracts" \
    && grep -q 'step_already_done_with_code "$logfile" "TokenBridge" "$L2_RPC_URL" "L2" "$EXPECTED_L2_TOKEN_BRIDGE"' "$deploy_contracts" \
    && grep -q 'getCode(existing)' "$ensure_demo_erc20"; then
    pass "deploy-contracts verifies expected addresses and on-chain code before trusting prior deploy logs"
  else
    fail "deploy-contracts must verify expected address and on-chain code before skipping from prior deploy logs"
  fi

  if grep -q 'DEPLOY_FORCED_TRANSACTION_GATEWAY: ${DEPLOY_FORCED_TRANSACTION_GATEWAY:-false}' "$compose" \
    && grep -q 'DEPLOY_FORCED_TRANSACTION_GATEWAY' "$rollup_deploy_script" \
    && grep -q 'default true' "$rollup_deploy_script" \
    && grep -q 'DEPLOY_FORCED_TRANSACTION_GATEWAY="${DEPLOY_FORCED_TRANSACTION_GATEWAY:-false}"' "$deploy_contracts" \
    && grep -q 'validate_bool "DEPLOY_FORCED_TRANSACTION_GATEWAY"' "$deploy_contracts" \
    && grep -q 'L1_TOKEN_BRIDGE_NONCE_OFFSET=5' "$deploy_contracts" \
    && grep -q 'L1_TOKEN_BRIDGE_NONCE_OFFSET=8' "$deploy_contracts" \
    && grep -q 'Step 1 deploy log was created with DEPLOY_FORCED_TRANSACTION_GATEWAY' "$deploy_contracts" \
    && grep -q 'DEPLOY_FORCED_TRANSACTION_GATEWAY=%s' "$deploy_contracts" \
    && grep -Fq 'DEPLOY_FORCED_TRANSACTION_GATEWAY=*)' "$STACK/scripts/services/render-coordinator-postdeploy-config.sh" \
    && ! grep -q 'ForcedTxGateway unused' "$STACK/scripts/links.sh"; then
    pass "ForcedTransactionGateway can be disabled by default without breaking deploy nonce guardrails"
  else
    fail "quickstart must default DEPLOY_FORCED_TRANSACTION_GATEWAY=false while preserving upstream compatibility and nonce guardrails"
  fi

  if grep -q 'check_linux_native_optional_deps' "$deploy_contracts" \
    && grep -q '@chainsafe/blst-linux-' "$deploy_contracts" \
    && grep -q '@chainsafe/hashtree-linux-' "$deploy_contracts" \
    && grep -q -- '--filter linea-monorepo' "$deploy_contracts" \
    && grep -q -- '--filter contracts...' "$deploy_contracts" \
    && grep -q 'lineth-contracts-pnpm-store:/workspace/.pnpm-store' "$compose" \
    && grep -q 'lineth-contracts-root-node-modules:/workspace/node_modules' "$compose" \
    && grep -q 'lineth-contracts-package-node-modules:/workspace/contracts/node_modules' "$compose" \
    && grep -q 'lineth-eslint-config-node-modules:/workspace/ts-libs/eslint-config/node_modules' "$compose" \
    && grep -q 'lineth-contracts-artifacts:/workspace/contracts/artifacts' "$compose" \
    && grep -q 'lineth-contracts-cache:/workspace/contracts/cache' "$compose"; then
    pass "deploy-contracts keeps Linux dependency and Hardhat caches in Docker volumes"
  else
    fail "deploy-contracts must not reuse host node_modules or throw away the Hardhat cache"
  fi

  if grep -q 'corepack prepare pnpm@10.32.1 --activate' "$ensure_demo_erc20_sh" \
    && grep -q 'LINETH_ACCOUNTS_DIR' "$ensure_demo_erc20_sh" \
    && grep -q 'resolveL1DeployerConfig' "$ensure_demo_erc20" \
    && ! grep -q ': "${L1_DEPLOYER_PRIVATE_KEY' "$ensure_demo_erc20_sh"; then
    pass "on-demand ERC20 deploy helper prepares pnpm and exports generated runtime keys for TypeScript"
  else
    fail "on-demand ERC20 deploy helper must prepare pnpm and export generated runtime keys before invoking TypeScript"
  fi

  if grep -q 'corepack prepare pnpm@10.32.1 --activate' "$traffic_account_sh"; then
    pass "traffic account helper prepares pnpm before invoking TypeScript"
  else
    fail "traffic account helper must prepare pnpm before invoking TypeScript"
  fi

  if grep -q 'runL1PolicyCheck' "$account_setup_ts" \
    && grep -q 'resolveL1DeployerConfig' "$account_setup_ts" \
    && grep -q 'L1_DEPLOYER_MIN_BALANCE_WEI' "$compose" \
    && grep -q 'L1_ROLE_TOP_UP_WEI' "$compose" \
    && grep -q 'L1_POSTMAN_TOP_UP_WEI' "$compose" \
    && grep -q 'L1_DEPLOYER_MIN_BALANCE_WEI' "$deploy_contracts" \
    && grep -q 'Late L1 deployer balance safety check' "$deploy_contracts" \
    && grep -q 'Late L1 deployer balance safety check failed' "$deploy_contracts" \
    && grep -q 'fund-runtime-accounts.ts' "$deploy_contracts" \
    && grep -q 'Cannot fund' "$fund_runtime_accounts" \
    && grep -q 'Sepolia deployer funding required before Docker startup' "$preflight_ts" \
    && grep -q 'L1_DEPLOYER_MIN_BALANCE_WEI' "$STACK/.env.example" \
    && grep -q 'L1_DEPLOYER_MIN_BALANCE_WEI' "$STACK/README.md"; then
    pass "quickstart fails fast on underfunded Sepolia deployer"
  else
    fail "quickstart must fail fast when the Sepolia deployer cannot cover deploy/runtime funding"
  fi

  if [ -f "$fund_runtime_accounts" ] \
    && grep -q 'Runtime funding: L1 batch' "$fund_runtime_accounts" \
    && grep -q 'Runtime funding: L2 batch' "$fund_runtime_accounts" \
    && grep -q 'getTransactionCount(.*"pending"' "$fund_runtime_accounts" \
    && grep -q 'Promise.all' "$fund_runtime_accounts" \
    && grep -q 'gasLimit: 21000n' "$fund_runtime_accounts" \
    && grep -q 'buildSepoliaPolicyConfig' "$fund_runtime_accounts" \
    && grep -q 'l2GasPriceWei' "$fund_runtime_accounts" \
    && grep -q 'fund-runtime-accounts.ts' "$deploy_contracts" \
    && ! grep -q 'fund_l1_account()' "$deploy_contracts" \
    && ! grep -q 'fund_l2_account()' "$deploy_contracts" \
    && ! grep -q 'cast send "$addr" --value' "$deploy_contracts"; then
    pass "runtime signer funding is batched in TypeScript with explicit nonces"
  else
    fail "runtime signer funding must use scripts/internal/fund-runtime-accounts.ts and must not use serial cast send funding"
  fi

  if grep -q 'l2 rpc latest block' "$status_script" \
    && grep -q 'rollup finalized block is ahead of local L2 latest block' "$status_script" \
    && grep -q 'local chain state does not match the preserved L1 rollup state' "$status_script" \
    && grep -q 'state mismatch guardrails' "$status_script" \
    && grep -q 'eth_getCode' "$status_script" \
    && grep -q 'deploy logs reference L2 block' "$status_script" \
    && grep -q 'addresses.json l2ChainId=' "$status_script"; then
    pass "status.sh warns when preserved L1 artifact state does not match local L2"
  else
    fail "status.sh must warn when preserved L1 artifact state does not match local L2"
  fi

  if grep -q 'boot failure' "$status_script" \
    && grep -q 'lineth_error "$init_container $init_state"' "$status_script" \
    && grep -q 'insufficient funds' "$status_script"; then
    pass "status.sh surfaces failed init containers with actionable log tails"
  else
    fail "status.sh must surface failed init containers with actionable log tails"
  fi

  if grep -q 'print_deploy_progress' "$status_script" \
    && grep -q 'deploy-contracts is still running; latest useful deploy log lines' "$status_script" \
    && grep -q 'addresses.json' "$status_script" \
    && grep -q 'Compiling contracts' "$status_script" \
    && grep -q 'Funding ' "$status_script"; then
    pass "status.sh surfaces active deploy progress before addresses.json exists"
  else
    fail "status.sh must surface active deploy progress while addresses.json is missing"
  fi

  if [ -f "$logging_lib" ] \
    && sh -n "$logging_lib" \
    && grep -q 'lineth_banner' "$logging_lib" \
    && grep -q 'lineth_section' "$logging_lib" \
    && grep -q 'lib/logging.sh' "$status_script" \
    && grep -q 'lineth_banner "status' "$status_script"; then
    pass "status.sh uses the shared Lineth terminal logger"
  else
    fail "status.sh must use scripts/lib/logging.sh for the Lineth terminal banner"
  fi
}

check_smoke_and_traffic_scripts() {
  user_facing_scripts="start.sh check-ports.sh links.sh watch.sh export-output.sh traffic-generation/send-l2-test-tx.sh traffic-generation/send-l2-erc20-transfer.sh traffic-generation/generate-l2-erc20-traffic.sh smoke-test/smoke-bridge-message.sh smoke-test/smoke-bridge-erc20-l1-to-l2.sh smoke-test/smoke-bridge-message-l2-to-l1.sh smoke-test/smoke-bridge-erc20-l2-to-l1.sh"

  for script in $user_facing_scripts; do
    script_path="$STACK/scripts/$script"
    if [ -x "$script_path" ]; then
      pass "$script is executable"
    else
      fail "$script must exist and be executable"
    fi

    if [ -f "$script_path" ] && sh -n "$script_path"; then
      pass "$script has valid shell syntax"
    elif [ -f "$script_path" ]; then
      fail "$script has invalid shell syntax"
    fi

    if [ -f "$script_path" ] \
      && grep -q 'lib/logging.sh' "$script_path" \
      && grep -q 'lineth_banner' "$script_path"; then
      pass "$script uses the shared Lineth terminal logger"
    elif [ -f "$script_path" ]; then
      fail "$script must use scripts/lib/logging.sh for terminal output"
    fi
  done

  for script in \
    traffic-generation/send-l2-test-tx.sh \
    traffic-generation/send-l2-erc20-transfer.sh \
    traffic-generation/generate-l2-erc20-traffic.sh \
    smoke-test/smoke-bridge-message-l2-to-l1.sh \
    smoke-test/smoke-bridge-erc20-l2-to-l1.sh; do
    script_path="$STACK/scripts/$script"
    if [ -f "$script_path" ] \
      && grep -q 'L2_GAS_PRICE_WEI' "$script_path" \
      && grep -q -- '--legacy' "$script_path" \
      && grep -q -- '--gas-price "$L2_GAS_PRICE_WEI"' "$script_path"; then
      pass "$script pins local L2 gas price instead of using RPC auto-fee selection"
    else
      fail "$script must use explicit local L2 legacy gas pricing via L2_GAS_PRICE_WEI"
    fi
  done

  if [ -f "$STACK/scripts/check-ports.sh" ] \
    && grep -q 'HOST_PORT_L2_RPC' "$STACK/scripts/check-ports.sh" \
    && grep -q 'HOST_PORT_L2_BLOCKSCOUT_FRONTEND' "$STACK/scripts/check-ports.sh" \
    && grep -q 'HOST_PORT_COORDINATOR' "$STACK/scripts/check-ports.sh" \
    && grep -q './scripts/check-ports.sh' "$STACK/README.md"; then
    pass "check-ports.sh checks expected host ports"
  else
    fail "check-ports.sh must check expected host ports and be documented"
  fi

  if [ -f "$STACK/scripts/export-output.sh" ] \
    && grep -q 'lineth-output' "$ROOT/.gitignore" \
    && grep -q 'links.json' "$STACK/scripts/export-output.sh" \
    && grep -q 'finality-report.json' "$STACK/scripts/export-output.sh" \
    && grep -q 'smoke-report.json' "$STACK/scripts/export-output.sh" \
    && grep -q 'postmanMessageSummary' "$STACK/scripts/export-output.sh" \
    && grep -q 'deploy-logs' "$STACK/scripts/export-output.sh" \
    && grep -q 'rm -rf "$OUTPUT_DIR/deploy-logs"' "$STACK/scripts/export-output.sh" \
    && grep -q 'support bundle' "$STACK/scripts/export-output.sh" \
    && grep -q './scripts/export-output.sh' "$STACK/README.md"; then
    pass "export-output.sh writes a fresh local lineth-output support bundle"
  else
    fail "export-output.sh must write a fresh lineth-output bundle with addresses, links, finality/smoke reports, and deploy logs"
  fi

  if [ -f "$STACK/scripts/watch.sh" ] \
    && grep -q 'Deploy contracts' "$STACK/scripts/watch.sh" \
    && grep -q 'Wait for finality' "$STACK/scripts/watch.sh" \
    && grep -q 'Show links' "$STACK/scripts/watch.sh" \
    && grep -q 'Step 1 reused: L1 Verifier + LineaRollup' "$STACK/scripts/watch.sh" \
    && grep -q 'Step 2 reused: L2 MessageService' "$STACK/scripts/watch.sh" \
    && grep -q 'Step 3 reused: L1 TokenBridge' "$STACK/scripts/watch.sh" \
    && grep -q 'Step 4 reused: L2 TokenBridge' "$STACK/scripts/watch.sh" \
    && grep -q 'existing deployment addresses found; waiting for deploy-contracts to verify/reuse them' "$STACK/scripts/watch.sh" \
    && grep -q 'deploy_addresses_ready' "$STACK/scripts/watch.sh" \
    && grep -q 'Coordinator retry noise' "$STACK/scripts/watch.sh" \
    && grep -q 'first L1 finalization observed' "$STACK/scripts/watch.sh" \
    && grep -q './scripts/watch.sh' "$STACK/README.md"; then
    pass "watch.sh provides the guided deploy/proof/finality progress view"
  else
    fail "watch.sh must provide a documented guided deploy/proof/finality progress view"
  fi

  if [ -f "$STACK/scripts/start.sh" ] \
    && grep -q 'LINETH_SUPPRESS_BANNER=1 "$SCRIPT_DIR/watch.sh"' "$STACK/scripts/start.sh" \
    && grep -q 'Check ports' "$STACK/scripts/start.sh" \
    && grep -q 'Check L1 network' "$STACK/scripts/start.sh" \
    && grep -q 'Generate accounts and configs' "$STACK/scripts/start.sh" \
    && grep -q 'Pull Docker images' "$STACK/scripts/start.sh" \
    && grep -q 'Start services' "$STACK/scripts/start.sh" \
    && grep -q -- '--profile local-l1 up -d l1-node-genesis-generator l1-el-node l1-cl-node' "$STACK/scripts/start.sh" \
    && ! grep -q 'lineth_section "Start stack"' "$STACK/scripts/start.sh" \
    && ! grep -q 'lineth_section "port preflight"' "$STACK/scripts/start.sh" \
    && ! grep -q 'lineth_section "Sepolia preflight"' "$STACK/scripts/start.sh" \
    && ! grep -q 'lineth_section "artifacts"' "$STACK/scripts/start.sh" \
    && ! grep -q 'lineth_section "compose"' "$STACK/scripts/start.sh" \
    && ! grep -q 'lineth_section "Prepare generated files"' "$STACK/scripts/start.sh" \
    && ! grep -q 'lineth_section "Start Docker containers"' "$STACK/scripts/start.sh" \
    && ! grep -q 'logs -f --tail=120' "$STACK/scripts/start.sh" \
    && grep -q 'logs -f --tail=120' "$STACK/README.md" \
    && ! grep -q 'run ./scripts/export-output.sh when ready' "$STACK/scripts/start.sh" "$STACK/scripts/watch.sh"; then
    pass "start.sh --tail uses the guided watcher; raw Docker logs stay documented separately"
  else
    fail "start.sh --tail must use watch.sh for guided output and keep raw Docker logs out of the default path"
  fi

  if grep -q 'mode                          ' "$STACK/scripts/internal/quickstart-preflight.ts" \
    && grep -q 'local dev mode; Sepolia gas/blob gates skipped' "$STACK/scripts/internal/quickstart-preflight.ts" \
    && grep -q 'gas                           execution' "$STACK/scripts/internal/quickstart-preflight.ts" \
    && grep -q 'blob fee                      blob base' "$STACK/scripts/internal/quickstart-preflight.ts" \
    && grep -q 'current Sepolia execution fee' "$STACK/README.md" \
    && grep -q 'current Sepolia blob base fee' "$STACK/README.md"; then
    pass "L1 check explains local mode and Sepolia execution/blob-base fee checks with readable labels"
  else
    fail "quickstart L1 check must explain local mode and Sepolia execution/blob-base fee checks with readable labels"
  fi

  if grep -q 'L1_MODE=sepolia' "$STACK/.env.example" \
    && grep -q 'L1_MODE=local' "$STACK/profiles/local-l1.env.example" \
    && grep -q '31648428' "$STACK/scripts/internal/deployer-wallet.ts" \
    && grep -q 'DEFAULT_LOCAL_L1_HOST_RPC_PORT = "8445"' "$STACK/scripts/internal/deployer-wallet.ts" \
    && grep -q 'localL1HostRpcUrl' "$STACK/scripts/internal/deployer-wallet.ts" \
    && grep -q 'http://l1-el-node:8545' "$STACK/scripts/internal/deployer-wallet.ts" \
    && grep -q '0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80' "$STACK/scripts/internal/deployer-wallet.ts" \
    && grep -q 'profiles: \["local-l1"\]' "$STACK/docker-compose.yml" \
    && grep -q 'l1-el-node' "$STACK/docker-compose.yml" \
    && grep -q 'l1-cl-node' "$STACK/docker-compose.yml" \
    && grep -q 'lineth-stack-local-l1-data' "$STACK/docker-compose.yml" \
    && grep -q 'lineth_l1_address_link' "$STACK/scripts/links.sh" \
    && grep -q 'lineth_l1_address_link' "$STACK/scripts/export-output.sh" \
    && grep -q 'lineth_l1_host_rpc_url' "$STACK/scripts/status.sh"; then
    pass "local L1 mode constants, profile, services, and mode-aware output helpers are present"
  else
    fail "local L1 mode must document constants, add local-l1 Compose services, and route L1 output through helpers"
  fi

  if grep -q 'local L1 mode ignores stale Sepolia RPC URL' "$STACK/scripts/internal/sepolia-policy.test.ts" \
    && grep -q 'local L1 mode honors HOST_PORT_L1_RPC for host checks only' "$STACK/scripts/internal/sepolia-policy.test.ts" \
    && ! grep -q 'lineth_env_or_default L1_RPC_URL "http://localhost' "$STACK/scripts/lib/runtime.sh" \
    && ! grep -q 'lineth_env_or_default L1_RPC_URL "http://l1-el-node:8545"' "$STACK/scripts/lib/runtime.sh" \
    && ! grep -q 'L1_RPC_URL="${L1_RPC_URL:-http://l1-el-node:8545}"' "$STACK/scripts/phases/03-render-service-configs.sh" \
    && ! grep -q 'L1_RPC_URL="${L1_RPC_URL:-http://l1-el-node:8545}"' "$STACK/scripts/services/render-postman-config.sh" \
    && ! grep -q 'L1_RPC_URL="${1:-${L1_RPC_URL:-$LOCAL_L1_CONTAINER_RPC_URL}}"' "$STACK/scripts/phases/04-deploy-contracts.sh"; then
    pass "local L1 mode ignores stale L1_RPC_URL in runtime and container resolver paths"
  else
    fail "local L1 mode must ignore stale L1_RPC_URL and use local L1 defaults"
  fi

  if grep -q 'local L1 mode ignores stale Sepolia deployer private key' "$STACK/scripts/internal/sepolia-policy.test.ts" \
    && grep -q 'local mode ignores Sepolia deployer artifact and stale config' "$STACK/scripts/internal/deployer-wallet.test.ts" \
    && grep -q 'source: "local-genesis"' "$STACK/scripts/internal/deployer-wallet.ts" \
    && grep -q 'printf.*0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80' "$STACK/scripts/lib/runtime.sh" \
    && ! grep -q 'envValue("L1_DEPLOYER_PRIVATE_KEY", env, LOCAL_L1_DEPLOYER_PRIVATE_KEY)' "$STACK/scripts/internal/sepolia-policy.ts" \
    && ! grep -q 'L1_DEPLOYER_PRIVATE_KEY="${L1_DEPLOYER_PRIVATE_KEY:-$LOCAL_L1_DEPLOYER_PRIVATE_KEY}"' "$STACK/scripts/phases/04-deploy-contracts.sh" \
    && ! grep -q 'L1_DEPLOYER_PRIVATE_KEY="${L1_DEPLOYER_PRIVATE_KEY:-0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80}"' "$STACK/scripts/internal/ensure-demo-erc20.sh"; then
    pass "local L1 mode ignores stale L1_DEPLOYER_PRIVATE_KEY in policy, runtime, and deploy paths"
  else
    fail "local L1 mode must ignore stale L1_DEPLOYER_PRIVATE_KEY and use the local genesis deployer"
  fi

  if grep -q 'lineth_l1_address_link' "$STACK/scripts/phases/04-deploy-contracts.sh" \
    && awk '
      /deploy-contracts:/ { in_service = 1 }
      in_service && /entrypoint:/ { exit }
      in_service && index($0, "./scripts/lib:/scripts/lib:ro") { found = 1 }
      END { exit found ? 0 : 1 }
    ' "$STACK/docker-compose.yml"; then
    pass "deploy-contracts mounts scripts/lib before using shared L1 link helpers"
  else
    fail "deploy-contracts must mount ./scripts/lib:/scripts/lib:ro when deploy output uses shared L1 link helpers"
  fi

  if grep -q 'verify_address "$LINEA_ROLLUP_ADDRESS" "$PRECOMPUTED_LINEA_ROLLUP"' "$STACK/scripts/phases/04-deploy-contracts.sh" \
    && grep -q 'verify_address "$L2_MESSAGE_SERVICE_ADDRESS" "$PRECOMPUTED_L2_MS"' "$STACK/scripts/phases/04-deploy-contracts.sh" \
    && grep -q 'verify_bridge_step_addresses' "$STACK/scripts/phases/04-deploy-contracts.sh" \
    && grep -q 'step_already_done_with_code "$logfile" "$primary_contract" "$L1_RPC_URL" "L1" "$PRECOMPUTED_LINEA_ROLLUP"' "$STACK/scripts/phases/04-deploy-contracts.sh" \
    && grep -q 'step_already_done_with_code "$logfile" "L2MessageService" "$L2_RPC_URL" "L2" "$PRECOMPUTED_L2_MS"' "$STACK/scripts/phases/04-deploy-contracts.sh" \
    && grep -q 'step_already_done_with_code "$logfile" "TokenBridge" "$L1_RPC_URL" "L1" "$EXPECTED_L1_TOKEN_BRIDGE"' "$STACK/scripts/phases/04-deploy-contracts.sh" \
    && grep -q 'step_already_done_with_code "$logfile" "TokenBridge" "$L2_RPC_URL" "L2" "$EXPECTED_L2_TOKEN_BRIDGE"' "$STACK/scripts/phases/04-deploy-contracts.sh"; then
    pass "deploy-contracts verifies precomputed addresses on both fresh deploy and deploy-log reuse"
  else
    fail "deploy-contracts must verify precomputed addresses when reusing prior deploy logs"
  fi

  if grep -q 'l1-el-node' "$STACK/scripts/start.sh" \
    && grep -q 'l1-cl-node' "$STACK/scripts/start.sh" \
    && grep -q 'eth_blockNumber' "$STACK/scripts/start.sh" \
    && grep -q 'advance' "$STACK/scripts/start.sh"; then
    pass "start.sh waits for local L1 EL, CL, and block production before preflight"
  else
    fail "start.sh must wait for local L1 EL, CL, and advancing eth_blockNumber before preflight"
  fi

  if grep -q 'HOST_PORT_L1_RPC' "$STACK/scripts/check-ports.sh" \
    && grep -q 'HOST_PORT_L1_WS' "$STACK/scripts/check-ports.sh" \
    && grep -q 'HOST_PORT_L1_ENGINE' "$STACK/scripts/check-ports.sh" \
    && grep -q 'HOST_PORT_L1_P2P' "$STACK/scripts/check-ports.sh" \
    && grep -q 'HOST_PORT_L1_DISCOVERY' "$STACK/scripts/check-ports.sh" \
    && grep -q 'HOST_PORT_L1_CL_P2P' "$STACK/scripts/check-ports.sh" \
    && grep -q 'HOST_PORT_L1_CL_METRICS' "$STACK/scripts/check-ports.sh" \
    && grep -q 'HOST_PORT_L1_CL_REST' "$STACK/scripts/check-ports.sh"; then
    pass "check-ports.sh checks every local L1 port exposed by Compose in local mode"
  else
    fail "check-ports.sh must check every local L1 port exposed by Compose when L1_MODE=local"
  fi

  hardcoded_etherscan="$(grep -R 'https://sepolia\.etherscan\.io' "$STACK/scripts" 2>/dev/null \
    | grep -v 'check-quickstart-static.sh' \
    | grep -v 'lib/runtime.sh' || true)"
  if [ -z "$hardcoded_etherscan" ]; then
    pass "generic quickstart scripts do not hardcode Sepolia Etherscan URLs"
  else
    fail "generic quickstart scripts must route Sepolia Etherscan URLs through scripts/lib/runtime.sh"
  fi

  if grep -q 'retry_noise_line' "$STACK/scripts/watch.sh" \
    && grep -q 'replacement transaction underpriced' "$STACK/scripts/watch.sh" \
    && grep -q 'ShnarfAlreadySubmitted' "$STACK/scripts/watch.sh" \
    && grep -q 'retry noise is only a blocker if finalized L2 block stops advancing' "$STACK/scripts/watch.sh" \
    && grep -q 'The guided watcher classifies common transient coordinator retry noise' "$STACK/README.md"; then
    pass "watch.sh classifies transient coordinator retry noise without hiding raw logs"
  else
    fail "watch.sh must classify known transient coordinator retry noise and README must document it"
  fi

  if [ -f "$STACK/scripts/phases/04-deploy-contracts.sh" ] \
    && grep -q 'useful links:' "$STACK/scripts/phases/04-deploy-contracts.sh" \
    && grep -q 'L2 Blockscout UI' "$STACK/scripts/phases/04-deploy-contracts.sh" \
    && grep -q 'L1 LineaRollupV8' "$STACK/scripts/phases/04-deploy-contracts.sh"; then
    pass "deploy-contracts prints useful links after addresses.json is written"
  else
    fail "deploy-contracts must print useful links after addresses.json is written"
  fi

  if [ -f "$STACK/scripts/traffic-generation/send-l2-test-tx.sh" ] && grep -q 'L2_DEPLOYER_PRIVATE_KEY' "$STACK/scripts/traffic-generation/send-l2-test-tx.sh" && grep -q 'cast send' "$STACK/scripts/traffic-generation/send-l2-test-tx.sh"; then
    pass "send-l2-test-tx.sh sends a simple L2 transaction from the generated L2 deployer"
  else
    fail "send-l2-test-tx.sh must send a simple L2 transaction from the generated L2 deployer"
  fi

  if [ -f "$STACK/scripts/traffic-generation/send-l2-erc20-transfer.sh" ] \
    && grep -q 'ERC20Example' "$STACK/scripts/traffic-generation/send-l2-erc20-transfer.sh" \
    && grep -q 'ensure-demo-erc20.sh l2' "$STACK/scripts/traffic-generation/send-l2-erc20-transfer.sh" \
    && grep -q 'traffic-account.sh ensure' "$STACK/scripts/traffic-generation/send-l2-erc20-transfer.sh" \
    && grep -q '/traffic-accounts:rw' "$STACK/scripts/traffic-generation/send-l2-erc20-transfer.sh" \
    && grep -q 'DEMO_TRAFFIC_ENV="/traffic-accounts/demo-traffic.env"' "$STACK/scripts/traffic-generation/send-l2-erc20-transfer.sh" \
    && grep -q 'TRAFFIC_ERC20_ADDRESS' "$STACK/scripts/traffic-generation/send-l2-erc20-transfer.sh" \
    && grep -q 'transfer(address,uint256)' "$STACK/scripts/traffic-generation/send-l2-erc20-transfer.sh" \
    && grep -q 'DEMO_TRAFFIC_ENV="/accounts/demo-traffic.env"' "$STACK/scripts/traffic-generation/send-l2-erc20-transfer.sh" \
    && grep -q 'L2_TRAFFIC_PRIVATE_KEY' "$STACK/scripts/traffic-generation/send-l2-erc20-transfer.sh" \
    && grep -q 'L2_TRAFFIC_ETH_TOP_UP_WEI' "$STACK/scripts/traffic-generation/send-l2-erc20-transfer.sh" \
    && grep -q 'L2_TRAFFIC_ERC20_TOP_UP_WEI' "$STACK/scripts/traffic-generation/send-l2-erc20-transfer.sh" \
    && ! grep -q 'cast wallet new' "$STACK/scripts/traffic-generation/send-l2-erc20-transfer.sh" \
    && ! grep -q 'funding traffic account ETH from L2 deployer' "$STACK/scripts/traffic-generation/send-l2-erc20-transfer.sh" \
    && ! grep -q 'funding traffic account ERC20Example' "$STACK/scripts/traffic-generation/send-l2-erc20-transfer.sh"; then
    pass "send-l2-erc20-transfer.sh uses the shared disposable L2 traffic account helper"
  else
    fail "send-l2-erc20-transfer.sh must use traffic-account.sh for disposable L2 account funding"
  fi

  if [ -f "$STACK/scripts/traffic-generation/generate-l2-erc20-traffic.sh" ] \
    && grep -q 'docker run -d' "$STACK/scripts/traffic-generation/generate-l2-erc20-traffic.sh" \
    && grep -q 'ensure-demo-erc20.sh l2' "$STACK/scripts/traffic-generation/generate-l2-erc20-traffic.sh" \
    && grep -q 'traffic-account.sh ensure' "$STACK/scripts/traffic-generation/generate-l2-erc20-traffic.sh" \
    && grep -q '/traffic-accounts:rw' "$STACK/scripts/traffic-generation/generate-l2-erc20-traffic.sh" \
    && grep -q 'DEMO_TRAFFIC_ENV="/traffic-accounts/demo-traffic.env"' "$STACK/scripts/traffic-generation/generate-l2-erc20-traffic.sh" \
    && grep -q 'TRAFFIC_ERC20_ADDRESS' "$STACK/scripts/traffic-generation/generate-l2-erc20-traffic.sh" \
    && grep -q 'while \[ "$MAX_TXS" -eq 0 \]' "$STACK/scripts/traffic-generation/generate-l2-erc20-traffic.sh" \
    && grep -q 'transfer(address,uint256)' "$STACK/scripts/traffic-generation/generate-l2-erc20-traffic.sh" \
    && grep -q 'DEMO_TRAFFIC_ENV="/accounts/demo-traffic.env"' "$STACK/scripts/traffic-generation/generate-l2-erc20-traffic.sh" \
    && grep -q 'L2_TRAFFIC_PRIVATE_KEY' "$STACK/scripts/traffic-generation/generate-l2-erc20-traffic.sh" \
    && grep -q 'L2_TRAFFIC_ETH_TOP_UP_WEI' "$STACK/scripts/traffic-generation/generate-l2-erc20-traffic.sh" \
    && grep -q 'L2_TRAFFIC_ERC20_TOP_UP_WEI' "$STACK/scripts/traffic-generation/generate-l2-erc20-traffic.sh" \
    && ! grep -q 'cast wallet new' "$STACK/scripts/traffic-generation/generate-l2-erc20-traffic.sh" \
    && ! grep -q 'funding traffic account ETH from L2 deployer' "$STACK/scripts/traffic-generation/generate-l2-erc20-traffic.sh" \
    && ! grep -q 'funding traffic account ERC20Example' "$STACK/scripts/traffic-generation/generate-l2-erc20-traffic.sh"; then
    pass "generate-l2-erc20-traffic.sh runs continuous traffic from the shared disposable account helper"
  else
    fail "generate-l2-erc20-traffic.sh must use traffic-account.sh for disposable L2 account funding"
  fi

  if [ -f "$STACK/scripts/smoke-test/smoke-bridge-message-l2-to-l1.sh" ] \
    && grep -q 'traffic-account.sh ensure' "$STACK/scripts/smoke-test/smoke-bridge-message-l2-to-l1.sh" \
    && grep -q '/traffic-accounts:rw' "$STACK/scripts/smoke-test/smoke-bridge-message-l2-to-l1.sh" \
    && grep -q 'DEMO_TRAFFIC_ENV="/traffic-accounts/demo-traffic.env"' "$STACK/scripts/smoke-test/smoke-bridge-message-l2-to-l1.sh" \
    && ! grep -q 'cast wallet new' "$STACK/scripts/smoke-test/smoke-bridge-message-l2-to-l1.sh" \
    && ! grep -q 'funding traffic account ETH from L2 deployer' "$STACK/scripts/smoke-test/smoke-bridge-message-l2-to-l1.sh"; then
    pass "smoke-bridge-message-l2-to-l1.sh uses the shared disposable account helper"
  else
    fail "smoke-bridge-message-l2-to-l1.sh must use traffic-account.sh for L2 sender funding"
  fi

  if [ -f "$STACK/scripts/smoke-test/smoke-bridge-erc20-l2-to-l1.sh" ] \
    && grep -q 'traffic-account.sh require-existing' "$STACK/scripts/smoke-test/smoke-bridge-erc20-l2-to-l1.sh" \
    && grep -q '/traffic-accounts:rw' "$STACK/scripts/smoke-test/smoke-bridge-erc20-l2-to-l1.sh" \
    && grep -q 'DEMO_TRAFFIC_ENV="/traffic-accounts/demo-traffic.env"' "$STACK/scripts/smoke-test/smoke-bridge-erc20-l2-to-l1.sh" \
    && ! grep -q 'cast wallet new' "$STACK/scripts/smoke-test/smoke-bridge-erc20-l2-to-l1.sh" \
    && ! grep -q 'funding withdraw account ETH from L2 deployer' "$STACK/scripts/smoke-test/smoke-bridge-erc20-l2-to-l1.sh"; then
    pass "smoke-bridge-erc20-l2-to-l1.sh uses the shared existing withdraw account helper"
  else
    fail "smoke-bridge-erc20-l2-to-l1.sh must use traffic-account.sh for L2 withdraw account funding"
  fi

  claim_helper="$STACK/scripts/internal/claim-l2-to-l1.ts"
  claim_test="$STACK/scripts/internal/claim-l2-to-l1.test.ts"
  if [ -f "$claim_helper" ] \
    && [ -f "$claim_test" ] \
    && grep -q 'getL2ToL1MessageStatus' "$claim_helper" \
    && grep -q 'getMessageProof' "$claim_helper" \
    && grep -q 'claimOnL1' "$claim_helper" \
    && grep -q 'L2->L1 message is' "$claim_helper" \
    && grep -q 'SDK errors are redacted' "$claim_test"; then
    pass "claim-l2-to-l1.ts centralizes L2-to-L1 SDK proof and claim logic"
  else
    fail "claim-l2-to-l1.ts and its test must centralize L2-to-L1 SDK proof and claim logic"
  fi

  for script in smoke-bridge-message-l2-to-l1.sh smoke-bridge-erc20-l2-to-l1.sh; do
    script_path="$STACK/scripts/smoke-test/$script"
	  if [ -f "$script_path" ] \
	    && grep -q 'claim_l2_to_l1()' "$script_path" \
	    && grep -q 'claim-l2-to-l1.ts' "$script_path" \
	    && grep -q 'docker exec -i' "$script_path" \
	    && grep -q '. "$runtime_keys_env"' "$script_path" \
	    && ! grep -q 'sed -nE .*L1_POSTMAN_PRIVATE_KEY' "$script_path" \
	    && ! grep -q 'getMessageProof' "$script_path" \
      && ! grep -q 'claimOnL1' "$script_path" \
      && ! grep -q "node --input-type=module.*<<'NODE'" "$script_path"; then
      pass "$script delegates manual L1 claims to claim-l2-to-l1.ts"
    else
      fail "$script must delegate manual L1 claims to claim-l2-to-l1.ts without embedded SDK heredocs"
    fi
  done

  if [ -f "$STACK/scripts/smoke-test/smoke-bridge-message.sh" ] \
    && grep -q 'CLAIMED_SUCCESS' "$STACK/scripts/smoke-test/smoke-bridge-message.sh" \
    && grep -q 'claim_tx_hash' "$STACK/scripts/smoke-test/smoke-bridge-message.sh" \
    && grep -q 'MessageClaimed' "$STACK/scripts/smoke-test/smoke-bridge-message.sh" \
    && ! grep -q 'not a pass/fail bridge smoke test yet' "$STACK/scripts/smoke-test/smoke-bridge-message.sh"; then
    pass "smoke-bridge-message.sh verifies a real L1-to-L2 claim"
  else
    fail "smoke-bridge-message.sh must send and verify a real L1-to-L2 claim"
  fi

  if [ -f "$STACK/scripts/smoke-test/smoke-bridge-erc20-l1-to-l2.sh" ] \
    && grep -q 'ensure-demo-erc20.sh l1' "$STACK/scripts/smoke-test/smoke-bridge-erc20-l1-to-l2.sh" \
    && grep -q 'bridgeToken(address,uint256,address)' "$STACK/scripts/smoke-test/smoke-bridge-erc20-l1-to-l2.sh" \
    && grep -q 'approve(address,uint256)' "$STACK/scripts/smoke-test/smoke-bridge-erc20-l1-to-l2.sh" \
    && grep -q 'nativeToBridgedToken(uint256,address)' "$STACK/scripts/smoke-test/smoke-bridge-erc20-l1-to-l2.sh" \
    && grep -q 'balanceOf(address)(uint256)' "$STACK/scripts/smoke-test/smoke-bridge-erc20-l1-to-l2.sh" \
    && grep -q 'CLAIMED_SUCCESS' "$STACK/scripts/smoke-test/smoke-bridge-erc20-l1-to-l2.sh" \
    && grep -q 'scripts/smoke-test/smoke-bridge-erc20-l1-to-l2.sh' "$STACK/README.md"; then
    pass "smoke-bridge-erc20-l1-to-l2.sh verifies a real ERC20 TokenBridge L1-to-L2 transfer"
  else
    fail "smoke-bridge-erc20-l1-to-l2.sh must bridge ERC20 through TokenBridge and verify the L2 balance"
  fi

  if [ -f "$STACK/scripts/smoke-test/smoke-bridge-erc20-l2-to-l1.sh" ] \
    && grep -q 'bridgeToken(address,uint256,address)' "$STACK/scripts/smoke-test/smoke-bridge-erc20-l2-to-l1.sh" \
    && grep -q 'approve(address,uint256)' "$STACK/scripts/smoke-test/smoke-bridge-erc20-l2-to-l1.sh" \
    && grep -q 'L2_SEND_RPC_URL' "$STACK/scripts/smoke-test/smoke-bridge-erc20-l2-to-l1.sh" \
    && grep -q 'claim_l2_to_l1' "$STACK/scripts/smoke-test/smoke-bridge-erc20-l2-to-l1.sh" \
    && grep -q 'postman claimed while manual claim raced' "$STACK/scripts/smoke-test/smoke-bridge-erc20-l2-to-l1.sh" \
    && grep -q 'balanceOf(address)(uint256)' "$STACK/scripts/smoke-test/smoke-bridge-erc20-l2-to-l1.sh" \
    && grep -q 'BridgingFinalizedV2' "$STACK/scripts/smoke-test/smoke-bridge-erc20-l2-to-l1.sh" \
    && grep -q 'scripts/smoke-test/smoke-bridge-erc20-l2-to-l1.sh' "$STACK/README.md"; then
    pass "smoke-bridge-erc20-l2-to-l1.sh verifies a real ERC20 TokenBridge L2-to-L1 withdrawal"
  else
    fail "smoke-bridge-erc20-l2-to-l1.sh must bridge ERC20 through TokenBridge, claim on L1, and verify the L1 balance"
  fi

  if [ -f "$STACK/scripts/smoke-test/smoke-bridge-message-l2-to-l1.sh" ] \
    && grep -q 'L2_TO_L1' "$STACK/scripts/smoke-test/smoke-bridge-message-l2-to-l1.sh" \
    && grep -q 'L2MessageService' "$STACK/scripts/smoke-test/smoke-bridge-message-l2-to-l1.sh" \
    && grep -q 'sendMessage(address,uint256,bytes)' "$STACK/scripts/smoke-test/smoke-bridge-message-l2-to-l1.sh" \
    && grep -q 'L2_SEND_RPC_URL' "$STACK/scripts/smoke-test/smoke-bridge-message-l2-to-l1.sh" \
    && grep -q 'claim_l2_to_l1' "$STACK/scripts/smoke-test/smoke-bridge-message-l2-to-l1.sh" \
    && grep -q 'CLAIMED_SUCCESS' "$STACK/scripts/smoke-test/smoke-bridge-message-l2-to-l1.sh" \
    && grep -q 'MessageClaimed' "$STACK/scripts/smoke-test/smoke-bridge-message-l2-to-l1.sh" \
    && grep -q 'scripts/smoke-test/smoke-bridge-message-l2-to-l1.sh' "$STACK/README.md"; then
    pass "smoke-bridge-message-l2-to-l1.sh verifies a real L2-to-L1 claim"
  else
    fail "smoke-bridge-message-l2-to-l1.sh must send, finalize, claim, and verify a real L2-to-L1 message"
  fi
}

check_pinned_utility_images_and_docs() {
  versions="$STACK/versions.env"
  readme="$STACK/README.md"
  compose="$STACK/docker-compose.yml"
  bootstrap="$STACK/scripts/bootstrap-artifacts.sh"
  deploy_contracts="$STACK/scripts/phases/04-deploy-contracts.sh"

  if grep -q '^FOUNDRY_TAG=' "$versions" && ! grep -q '^FOUNDRY_TAG=latest$' "$versions"; then
    pass "Foundry image tag is pinned"
  else
    fail "FOUNDRY_TAG must be pinned, not latest"
  fi

  if grep -q 'lineth-foundry-home:' "$compose" \
    && grep -q 'lineth-foundry-home:/root/.foundry' "$compose" \
    && grep -q 'foundry-tools:' "$compose" \
    && grep -q 'ghcr.io/foundry-rs/foundry:${FOUNDRY_TAG}' "$compose" \
    && grep -q '00-prepare-deploy-tools.sh' "$compose" \
    && grep -q 'FOUNDRY_TAG: ${FOUNDRY_TAG:' "$compose" \
    && grep -q 'Foundry binaries missing' "$deploy_contracts" \
    && grep -q 'corepack prepare pnpm@10.32.1' "$deploy_contracts" \
    && ! grep -q 'pnpm@latest' "$deploy_contracts" \
    && ! grep -q 'foundryup\|foundry.paradigm.xyz' "$deploy_contracts"; then
    pass "deploy-contracts uses pinned cached Foundry and pnpm tooling"
  else
    fail "deploy-contracts must use pinned cached Foundry image tooling and must not prepare pnpm@latest or run foundryup"
  fi

  if grep -q '^BUSYBOX_TAG=' "$versions" \
    && grep -q 'BUSYBOX_TAG=' "$bootstrap" \
    && grep -q '"busybox:${BUSYBOX_TAG}"' "$bootstrap" \
    && ! grep -q 'docker run .* busybox sh' "$bootstrap"; then
    pass "bootstrap migration helpers use pinned busybox tag"
  else
    fail "bootstrap-artifacts.sh migration helpers must use busybox:${BUSYBOX_TAG}, not unpinned busybox"
  fi

  if grep -q 'Accounts and funding model' "$readme" \
    && grep -q 'generated L2 deployer' "$readme" \
    && grep -q 'L2MessageService' "$readme" \
    && grep -q 'ERC20Example' "$readme" \
    && grep -q 'disposable demo traffic account' "$readme" \
    && grep -q 'bootstrap/admin' "$readme"; then
    pass "README documents L2 ETH and ERC20Example funding model"
  else
    fail "README must document L2 ETH, ERC20Example, and disposable traffic-account funding model"
  fi

  if grep -q 'Config model' "$readme" \
    && grep -q 'profiles/ports.env.example' "$readme" \
    && grep -q 'profiles/gas-sepolia.env.example' "$readme" \
    && grep -q 'profiles/prover-partial.env.example' "$readme"; then
    pass "README documents the .env plus recipe-file config model"
  else
    fail "README must document the .env plus recipe-file config model"
  fi

  if grep -q 'CI maintenance path' "$readme" \
    && grep -q 'Do not put full Sepolia finality in normal PR CI' "$readme" \
    && grep -q 'Latest verified dev-proof fresh boot' "$readme" \
    && grep -q 'Latest verified local traffic checks' "$readme"; then
    pass "README documents verified timing, local traffic checks, and CI maintenance path"
  else
    fail "README must document verified timing, local traffic checks, and CI maintenance path"
  fi
}

check_quickstart_review_fixes() {
  deploy_contracts="$STACK/scripts/phases/04-deploy-contracts.sh"
  account_setup_ts="$STACK/scripts/internal/account-setup.ts"
  token_bridge_ts="$STACK/scripts/internal/deployBridgedTokenAndTokenBridgeV1_1.ts"
  fund_runtime_accounts="$STACK/scripts/internal/fund-runtime-accounts.ts"
  rollup_deploy_script="$ROOT/contracts/local-deployments-artifacts/deployPlonkVerifierAndLineaRollupV8.ts"
  fee_overrides_ts="$ROOT/contracts/common/helpers/feeOverrides.ts"
  fee_overrides_test="$ROOT/contracts/common/helpers/feeOverrides.test.ts"
  readme="$STACK/README.md"

  # Task 2: restart-safe redeploy nonce guard.
  if grep -q 'guard_redeploy_nonce_window()' "$deploy_contracts" \
    && grep -q 'This artifact set is partial or stale; run ./scripts/reset.sh before redeploying.' "$deploy_contracts" \
    && grep -q 'guard_redeploy_nonce_window "L1" "$L1_RPC_URL" "$L1_DEPLOYER_ADDRESS" \\' "$deploy_contracts" \
    && grep -q 'EXPECTED_LINEA_ROLLUP_NONCE' "$deploy_contracts" \
    && grep -q 'EXPECTED_L2_MESSAGE_SERVICE_NONCE' "$deploy_contracts" \
    && [ "$(grep -c 'guard_redeploy_nonce_window ' "$deploy_contracts")" -ge 4 ] \
    && grep -q 'current_nonce > expected_nonce' "$deploy_contracts"; then
    pass "deploy-contracts guards stale/partial redeploys when the live deployer nonce passed the expected nonce"
  else
    fail "deploy-contracts must guard each deploy step with guard_redeploy_nonce_window and a reset hint"
  fi

  # Task 3: shared single-fee-model deploy helper.
  if [ -f "$fee_overrides_ts" ] \
    && grep -q 'resolveOneModelFeeOverrides' "$fee_overrides_ts" \
    && grep -q 'assertSingleFeeModel' "$fee_overrides_ts" \
    && grep -q 'feeBudgetPricePerGas' "$fee_overrides_ts" \
    && grep -q 'must not set both legacy gasPrice and EIP-1559 fields' "$fee_overrides_ts" \
    && [ -f "$fee_overrides_test" ]; then
    pass "shared deploy fee helper enforces a single fee model and is unit-tested"
  else
    fail "contracts/common/helpers/feeOverrides.ts must own single-fee-model deploy logic with a unit test"
  fi

  if grep -q 'resolveOneModelFeeOverrides' "$rollup_deploy_script" \
    && grep -q 'L1_DEPLOY_GAS_PRICE_WEI' "$rollup_deploy_script" \
    && ! grep -q 'getDeployFeeOverrides' "$rollup_deploy_script"; then
    pass "V8 rollup deploy routes fees through the shared single-fee-model helper"
  else
    fail "V8 rollup deploy must use resolveOneModelFeeOverrides instead of a local fee resolver"
  fi

  if grep -q 'resolveOneModelFeeOverrides' "$token_bridge_ts" \
    && grep -q 'assertSingleFeeModel' "$token_bridge_ts" \
    && grep -q 'L1_DEPLOY_GAS_PRICE_WEI' "$token_bridge_ts" \
    && ! grep -q 'let fees = {}' "$token_bridge_ts"; then
    pass "TokenBridge wrapper uses one fee model and follows the L1 deploy gas policy"
  else
    fail "TokenBridge wrapper must use the shared single-fee-model helper and must not mix fee fields"
  fi

  # Task 4: runtime funding uses the same single-fee-model shape.
  if grep -q 'feeBudgetPricePerGas' "$fund_runtime_accounts" \
    && grep -q 'FeeOverrides' "$fund_runtime_accounts" \
    && grep -q 'assertSingleFeeModel' "$fund_runtime_accounts" \
    && ! grep -q 'gasPrice: params.gasPrice' "$fund_runtime_accounts"; then
    pass "runtime funding sizes gas from the shared single-fee-model helper"
  else
    fail "fund-runtime-accounts must route fees through the shared single-fee-model helper"
  fi

  # Task 5: dev-only world-readable runtime key permissions are documented.
  if grep -q 'DEV-ONLY file mode' "$account_setup_ts" \
    && grep -q 'Do not present 0o644 as secure' "$account_setup_ts" \
    && grep -q 'written world-readable' "$readme"; then
    pass "world-readable runtime key permissions are documented as a dev-only exception"
  else
    fail "account-setup.ts and README must document the dev-only world-readable runtime key permissions"
  fi
}

check_no_tracked_generated_genesis
check_restructured_layout_paths
check_runtime_helper_usage
check_generated_genesis_is_volume_scoped
check_init_scripts_and_compose_shell
check_generated_l2_deployer_genesis
check_l2_chain_id_wiring
check_account_setup_key_model
check_typescript_quickstart_helpers
check_postman_key_model
check_incremental_typescript_helpers
check_partial_prover_guardrails
check_runtime_config_and_validium_guardrails
check_reuse_guardrails
check_smoke_and_traffic_scripts
check_pinned_utility_images_and_docs
check_quickstart_review_fixes

if [ "$FAILURES" -ne 0 ]; then
  printf '[quickstart-static] %s failure(s)\n' "$FAILURES" >&2
  exit 1
fi

printf "[quickstart-static] all checks passed\n"

import { hexlify, parseUnits } from "ethers";
import { ethers } from "hardhat";
import {
  BeaconBlockHeader,
  BeaconProofWitness,
  EIP4788Witness,
  PendingPartialWithdrawal,
  PendingPartialWithdrawalsWitness,
  ValidatorContainer,
  ValidatorContainerWitness,
} from "./types";
import { SecretKey } from "@chainsafe/blst";
import { SSZMerkleTree, TestValidatorContainerProofVerifier } from "contracts/typechain-types";
import {
  FAR_FUTURE_EXIT_EPOCH,
  GI_PENDING_PARTIAL_WITHDRAWALS_ROOT,
  SHARD_COMMITTEE_PERIOD,
  SLOTS_PER_EPOCH,
} from "../../common/constants";
import { randomBytes32 } from "../../../../common/helpers/encoding";

export interface LocalMerkleTree {
  sszMerkleTree: SSZMerkleTree;
  gIFirstValidator: string;
  firstValidatorLeafIndex: bigint;
  readonly totalValidators: number;
  addValidator: (validator: ValidatorContainer) => Promise<{ validatorIndex: number }>;
  validatorAtIndex: (index: number) => ValidatorContainer;
  commitChangesToBeaconRoot: (
    slot?: number,
  ) => Promise<{ childBlockTimestamp: number; beaconBlockHeader: BeaconBlockHeader }>;
  buildProof: (validatorIndex: number, beaconBlockHeader: BeaconBlockHeader) => Promise<string[]>;
}

// min = 0 will cause flaky test with NoValidatorExitForUnstakePermissionless() error
export const randomInt = (max: number, min = 1): number => Math.floor(Math.random() * (max - min)) + min;

export const generateBeaconHeader = (stateRoot: string, slot?: number): BeaconBlockHeader => {
  return {
    slot: slot ?? randomInt(1743359),
    proposerIndex: randomInt(1337),
    parentRoot: randomBytes32(),
    stateRoot,
    bodyRoot: randomBytes32(),
  };
};

const ikm = Uint8Array.from(Buffer.from("test test test test test test test", "utf-8"));
const masterSecret = SecretKey.deriveMasterEip2333(ikm);
let secretIndex = 0;

export const generateValidatorContainer = (customWC?: string): ValidatorContainer => {
  const secretKey = masterSecret.deriveChildEip2333(secretIndex++);
  const activationEligibilityEpoch = BigInt(randomInt(343000));

  return {
    pubkey: secretKey.toPublicKey().toHex(true),
    withdrawalCredentials: customWC ?? hexlify(randomBytes32()),
    effectiveBalance: parseUnits(randomInt(2048).toString(), "gwei"),
    slashed: false,
    activationEligibilityEpoch: activationEligibilityEpoch,
    activationEpoch: activationEligibilityEpoch + BigInt(randomInt(300)),
    exitEpoch: FAR_FUTURE_EXIT_EPOCH,
    withdrawableEpoch: FAR_FUTURE_EXIT_EPOCH,
  };
};

export const generatePendingPartialWithdrawal = (
  validatorIndex?: number | bigint,
  amount?: number | bigint,
  withdrawableEpoch?: number | bigint,
): PendingPartialWithdrawal => {
  return {
    validatorIndex: validatorIndex ?? BigInt(randomInt(1000000)),
    amount: amount ?? parseUnits(randomInt(2016).toString(), "gwei"), // Random amount up to 32 ETH in gwei
    withdrawableEpoch: withdrawableEpoch ?? BigInt(randomInt(343000)),
  };
};

export const generatePendingPartialWithdrawals = (
  ...withdrawals: PendingPartialWithdrawal[]
): PendingPartialWithdrawal[] => {
  const randomCount = randomInt(1000, 0); // Generate 0-5 random withdrawals
  const randomWithdrawals: PendingPartialWithdrawal[] = [];
  for (let i = 0; i < randomCount; i++) {
    randomWithdrawals.push(generatePendingPartialWithdrawal());
  }
  return [...withdrawals, ...randomWithdrawals];
};

export const setBeaconBlockRoot = async (root: string): Promise<number> => {
  const systemAddress = "0xfffffffffffffffffffffffffffffffffffffffe";
  const initialBalance = 999999999999999999999999999n;
  await ethers.provider.send("hardhat_impersonateAccount", [systemAddress]);
  await ethers.provider.send("hardhat_setBalance", [systemAddress, ethers.toBeHex(initialBalance)]);
  const systemSigner = await ethers.getSigner(systemAddress);
  const BEACON_ROOTS = "0x000F3df6D732807Ef1319fB7B8bB8522d0Beac02";
  const block = await systemSigner
    .sendTransaction({
      to: BEACON_ROOTS,
      value: 0,
      data: root,
    })
    .then((tx) => tx.getBlock());
  if (!block) throw new Error("invariant");
  return block.timestamp;
};

// Default mainnet values for validator state tree
export const prepareLocalMerkleTree = async (
  gIndex = "0x0000000000000000000000000000000000000000000000000096000000000028",
): Promise<LocalMerkleTree> => {
  const sszMerkleTree: SSZMerkleTree = await ethers.deployContract("SSZMerkleTree", [gIndex], {});
  const firstValidator = generateValidatorContainer();

  await sszMerkleTree.addValidatorLeaf(firstValidator);
  const validators: ValidatorContainer[] = [firstValidator];

  const firstValidatorLeafIndex = (await sszMerkleTree.leafCount()) - 1n;
  const gIFirstValidator = await sszMerkleTree.getGeneralizedIndex(firstValidatorLeafIndex);

  // compare GIndex.index()
  if (BigInt(gIFirstValidator) >> 8n !== BigInt(gIndex) >> 8n)
    throw new Error("Invariant: sszMerkleTree implementation is broken");

  const addValidator = async (validator: ValidatorContainer) => {
    await sszMerkleTree.addValidatorLeaf(validator);
    validators.push(validator);

    return {
      validatorIndex: validators.length - 1,
    };
  };

  const validatorAtIndex = (index: number) => {
    return validators[index];
  };

  const commitChangesToBeaconRoot = async (slot?: number) => {
    const beaconBlockHeader = generateBeaconHeader(await sszMerkleTree.getMerkleRoot(), slot);
    const beaconBlockHeaderHash = await sszMerkleTree.beaconBlockHeaderHashTreeRoot(beaconBlockHeader);
    return {
      childBlockTimestamp: await setBeaconBlockRoot(beaconBlockHeaderHash),
      beaconBlockHeader,
    };
  };

  const buildProof = async (validatorIndex: number, beaconBlockHeader: BeaconBlockHeader): Promise<string[]> => {
    const [validatorProof, stateProof, beaconBlockProof] = await Promise.all([
      sszMerkleTree.getValidatorPubkeyWCParentProof(validators[Number(validatorIndex)]).then((r) => r.proof),
      sszMerkleTree.getMerkleProof(BigInt(validatorIndex) + firstValidatorLeafIndex),
      sszMerkleTree.getBeaconBlockHeaderProof(beaconBlockHeader).then((r) => r.proof),
    ]);

    return [...validatorProof, ...stateProof, ...beaconBlockProof];
  };

  return {
    sszMerkleTree,
    gIFirstValidator,
    firstValidatorLeafIndex,
    get totalValidators(): number {
      return validators.length;
    },
    addValidator,
    validatorAtIndex,
    commitChangesToBeaconRoot,
    buildProof,
  };
};

export const ACTIVE_0X01_VALIDATOR_PROOF = {
  blockRoot: "0xeb961eae87c614e11a7959a529a59db3c9d825d284dc30e0d12e43ba6daf4cca",
  gIFirstValidator: "0x0000000000000000000000000000000000000000000000000096000000000028",
  beaconBlockHeader: {
    slot: 22140000,
    proposerIndex: 1337,
    parentRoot: "0x8576d3eb5ef5b3a460b85e5493d0d0510b7d1a2943a4e51add5227c9d3bffa0f",
    stateRoot: "0xa802c5f4f818564a2774a19937fdfafc0241f475d7f28312c3609c6e5995d980",
    bodyRoot: "0xca4f98890bc98a59f015d06375a5e00546b8f2ac1e88d31b1774ea28d4b3e7d1",
  },
  witness: {
    validatorIndex: 129n,
    validator: {
      pubkey: "0xaffd606a767b69df617169824f10baf992c407b059e18d8db2cf4f8cc7432dfe36477e94c9a14e87e1d13e5b22202b92",
      withdrawalCredentials: "0x010000000000000000000000b3e29c46ee1745724417c0c51eb2351a1c01cf36",
      effectiveBalance: 32000000000n,
      activationEligibilityEpoch: 10n,
      activationEpoch: 16n,
      exitEpoch: 18446744073709551615n,
      withdrawableEpoch: 18446744073709551615n,
      slashed: false,
    },
    // Should be proof for the Validator Container
    proof: [
      "0x0edd708eb0bcfc6f5c6c86579867e00b50938ae4656b566c1525385cf0e17d99",
      "0x4598d399eb5a13129ec7df15bf7f67ce62894f8dde56e20e01576d4b1c85d4c1",
      "0x250ab3879dec53f13d60b1fcba79ce887f2626863c4e1e678bbd0d0f1d1e9beb",
      "0xb71de3abf0dcb360fa49c4f512c5bfc5d513ecbd1dbc76c57ac29de173a65c23",
      "0x4eca0ad10870d33a08f0d4608d7ac506fa484876c884ed10887d46b5e1e694ca",
      "0xbed6b05d39560f0ebfd42fcaa33ddbdd9daf2efc3e0002fb327beae8088a4dad",
      "0xed185c1976880109a80043b032ff1bbedf74c501dc6e9a2785dabb438513a7dc",
      "0xc37a16340d7620558ce6048ce5913bfdfb4122b13185e99b6643a64d8f7e033c",
      "0xcbfafa05858b8aa3c8ff0312e723037a38985efea6528963b583aa7927907633",
      "0x73e9bb21041240d959b1fc6951a11b78cc5bf2955801663bf49cc397dfeef4dd",
      "0xe563a45ae8fd94663ad9b3cb0ac3e25c69827aa4b7ee39e5390b3bd1143e04bb",
      "0xbf617e7d2bd6314bb2fe5f525b95f42e629ee88a4d5075ed4841fe1165e2e633",
      "0x552485bed4a23db34514b36b78eca98f5933d4c874845042db9420c20e25bb19",
      "0xe72547a4304ee482db2d7fc7d8663b139889b1fc17d15cd29fd41fc8da7e4405",
      "0xc549ff35940423c68e7241a05ca1aa3af69fc4765232c04e575205fa57b8c3b1",
      "0x071f50189feb9a59c48c8d3a5b22bc42576ca404065aad4c3c34e662143fb35e",
      "0xfd1bd6dd9b73b8d2dfeebd1a0b45eb8903da8db371b72e3042f668a13ba0c43e",
      "0x4d8f1bb6f91e4161c180f9ecaad2f15c66cb6ec7f2cdedc3bcb93c274aad5fa2",
      "0x3b6350121535a5c9588561919e9772a84b549602680679766f6de26de339926e",
      "0xbb3b90338675e2e6bbb5057f462829234b29a9ab6ae17f1bb1e64c76bce64970",
      "0x3072ffd933e00085269e510dea720c2ac88a7853fdb3231ee5fb27e1e9a934cc",
      "0x8a8d7fe3af8caa085a7639a832001457dfb9128a8061142ad0335629ff23ff9c",
      "0xfeb3c337d7a51a6fbf00b9e34c52e1c9195c969bd4e7a0bfd51d5c5bed9c1167",
      "0xe71f0aa83cc32edfbefa9f4d3e0174ca85182eec9f3a09f6a6c0df6377a510d7",
      "0x31206fa80a50bb6abe29085058f16212212a60eec8f049fecb92d8c8e0a84bc0",
      "0x21352bfecbeddde993839f614c3dac0a3ee37543f9b412b16199dc158e23b544",
      "0x619e312724bb6d7c3153ed9de791d764a366b389af13c58bf8a8d90481a46765",
      "0x7cdd2986268250628d0c10e385c58c6191e6fbe05191bcc04f133f2cea72c1c4",
      "0x848930bd7ba8cac54661072113fb278869e07bb8587f91392933374d017bcbe1",
      "0x8869ff2c22b28cc10510d9853292803328be4fb0e80495e8bb8d271f5b889636",
      "0xb5fe28e79f1b850f8658246ce9b6a1e7b49fc06db7143e8fe0b4f2b0c5523a5c",
      "0x985e929f70af28d0bdd1a90a808f977f597c7c778c489e98d3bd8910d31ac0f7",
      "0xc6f67e02e6e4e1bdefb994c6098953f34636ba2b6ca20a4721d2b26a886722ff",
      "0x1c9a7e5ff1cf48b4ad1582d3f4e4a1004f3b20d8c5a2b71387a4254ad933ebc5",
      "0x2f075ae229646b6f6aed19a5e372cf295081401eb893ff599b3f9acc0c0d3e7d",
      "0x328921deb59612076801e8cd61592107b5c67c79b846595cc6320c395b46362c",
      "0xbfb909fdb236ad2411b4e4883810a074b840464689986c3f8a8091827e17c327",
      "0x55d8fb3687ba3ba49f342c77f5a1f89bec83d811446e1a467139213d640b6a74",
      "0xf7210d4f8e7e1039790e7bf4efa207555a10a6db1dd4b95da313aaa88b88fe76",
      "0xad21b516cbc645ffe34ab5de1c8aef8cd4e7f8d2b51e8e1456adc7563cda206f",
      "0x908a1e0000000000000000000000000000000000000000000000000000000000",
      "0xc6341f0000000000000000000000000000000000000000000000000000000000",
      "0x41e1bba24366f5cf6502295fea29d06396dd5d0031241811264f5212de2feb00",
      "0xc965aa7691807a7649ae6690e1a1d4871fc9ac6c3d96625fde856e152ea236d1",
      "0x85d66de5e59bf58a58e8e7334299a7fedcd87d23f97cf1579eae74e9fc0f0eaa",
      "0x5c76c5ff78ad80ef4bf12d6adb8df3b15f9c23798bda1aa1e110041254e76cce",
      "0x1c4a401fbd320fd7b848c9fc6118444da8797c2e41c525eb475ff76dbf44500b",
    ],
  },
} as const;

// Take the known working EIP4788 proof from ACTIVE_0X01_VALIDATOR_PROOF
// Create a new 0x02 ValidatorContainer based on the address input
// Re-hash back up the Merkle tree to derive a new state root and new beacon root
// Fine for unit tests where we need to stub the hash values anyway
export const generateEIP4478Witness = async (
  sszMerkleTree: SSZMerkleTree,
  verifier: TestValidatorContainerProofVerifier,
  address: string,
  effectiveBalance?: bigint,
  pendingPartialWithdrawalsGwei: bigint[] = [],
  randomPendingPartialWithdrawalsCount: number = 0,
): Promise<EIP4788Witness> => {
  // ============================================================================
  // Create ValidatorContainer
  // ============================================================================
  const lowercaseAddress = address.toLowerCase().replace(/^0x/, "");
  if (lowercaseAddress.length !== 40 || !/^[0-9a-f]+$/.test(lowercaseAddress)) {
    throw new Error("Invalid address format");
  }
  // Create 0x02 withdrawal credentials
  const withdrawalCredentials = `0x020000000000000000000000${lowercaseAddress}`;
  const validatorContainer = generateValidatorContainer(withdrawalCredentials);
  if (effectiveBalance) {
    validatorContainer.effectiveBalance = effectiveBalance;
  }

  const validatorIndex = ACTIVE_0X01_VALIDATOR_PROOF.witness.validatorIndex;

  // ValidatorContainer -> StateRoot
  const originalValidatorContainerProof = [...ACTIVE_0X01_VALIDATOR_PROOF.witness.proof];

  // ============================================================================
  // Generate pending partial withdrawals
  // ============================================================================
  const mappedPendingPartialWithdrawals: PendingPartialWithdrawal[] = pendingPartialWithdrawalsGwei.map(
    (amountGwei) => ({
      validatorIndex: validatorIndex,
      amount: amountGwei,
      withdrawableEpoch: BigInt(randomInt(343000)),
    }),
  );

  // Generate random pending partial withdrawals if requested
  const randomPendingPartialWithdrawals: PendingPartialWithdrawal[] = [];
  for (let i = 0; i < randomPendingPartialWithdrawalsCount; i++) {
    randomPendingPartialWithdrawals.push(generatePendingPartialWithdrawal());
  }

  // Combine mapped and randomly generated pending partial withdrawals
  const pendingPartialWithdrawals: PendingPartialWithdrawal[] = [
    ...mappedPendingPartialWithdrawals,
    ...randomPendingPartialWithdrawals,
  ];

  const pendingPartialWithdrawalsRoot = await sszMerkleTree.hashTreeRoot(pendingPartialWithdrawals);

  // We have two fixed nodes in the BeaconState Merkle tree:
  // - validators subtree at generalized index 75
  // - pending_partial_withdrawals subtree at generalized index 99
  //
  // Goal: construct a BeaconState root that is consistent with BOTH nodes.
  //
  // Observation:
  // - GI 75 and GI 99 lie in different halves of the tree
  // - Their lowest common ancestor (LCA) is GI = 1 (the state root)
  //
  // Construction approach:
  // 1. Starting from GI 75, hash upward toward the root, but STOP at GI=2 - the direct child of GI=1,
  //    choosing arbitrary sibling node values along the path.
  // 2. Do the same independently for GI 99, but STOP at GI=3
  // 3. Hash GI=2 and GI=3 nodes together -> get BeaconState root at GI=1

  // ============================================================================
  // Generate GI=2 from Validator Container
  // ============================================================================

  const validatorMerkleSubtree = await sszMerkleTree.getValidatorPubkeyWCParentProof(validatorContainer);
  const validatorGIndexInLeftSubtree = await verifier.getValidatorGIInLeftSubtree(validatorIndex);
  // Remove last element gI3
  const proofForGI2 = originalValidatorContainerProof.slice(0, -1);
  const gi2Root = await sszMerkleTree.getRoot(proofForGI2, validatorMerkleSubtree.root, validatorGIndexInLeftSubtree);

  // Verify (ValidatorContainer) leaf against (StateRoot) Merkle root
  await sszMerkleTree.verifyProof(proofForGI2, gi2Root, validatorMerkleSubtree.root, validatorGIndexInLeftSubtree);

  // ============================================================================
  // Generate GI=3 from Pending Partial Withdrawals
  // ============================================================================

  // Algorithm to find GI in the right subtree:
  // 1. Find # of leafs in original tree = 64
  // 2. Find the GI of the leftmost node = 96
  // 3. Find offset from this GI: 99 - 96 = 3
  // 4. The leftmost node will become GI 32 in the right subtree
  // 5. So this node becomes GI 32 + 3 = 35
  // gI99 -> gI35 in the right subtree
  const GI_PENDING_PARTIAL_WITHDRAWALS_ROOT_IN_RIGHT_SUBTREE =
    "0x000000000000000000000000000000000000000000000000000000000000231b";

  // each subtree has depth 5 -> create proof of 5 elements
  const proofForGI3 = Array.from({ length: 5 }, () => randomBytes32());
  const gi3Root = await sszMerkleTree.getRoot(
    proofForGI3,
    pendingPartialWithdrawalsRoot,
    GI_PENDING_PARTIAL_WITHDRAWALS_ROOT_IN_RIGHT_SUBTREE,
  );

  await sszMerkleTree.verifyProof(
    proofForGI3,
    gi3Root,
    pendingPartialWithdrawalsRoot,
    GI_PENDING_PARTIAL_WITHDRAWALS_ROOT_IN_RIGHT_SUBTREE,
  );

  // ============================================================================
  // Generate state root
  // ============================================================================

  const stateRoot = await sszMerkleTree.getRoot(
    [gi3Root],
    gi2Root,
    "0x0000000000000000000000000000000000000000000000000000000000000200",
  );

  await sszMerkleTree.verifyProof(
    [...proofForGI3, gi2Root],
    stateRoot,
    pendingPartialWithdrawalsRoot,
    GI_PENDING_PARTIAL_WITHDRAWALS_ROOT,
  );

  const validatorGIndex = await verifier.getValidatorGI(validatorIndex);
  await sszMerkleTree.verifyProof([...proofForGI2, gi3Root], stateRoot, validatorMerkleSubtree.root, validatorGIndex);

  // ============================================================================
  // Generate beacon chain header and beacon root
  // ============================================================================
  const slot = SLOTS_PER_EPOCH * (validatorContainer.activationEpoch + SHARD_COMMITTEE_PERIOD) + 1n;
  const beaconHeader = generateBeaconHeader(stateRoot, Number(slot));
  const beaconHeaderMerkleSubtree = await sszMerkleTree.getBeaconBlockHeaderProof(beaconHeader);
  const beaconRoot = await sszMerkleTree.getRoot(
    [...beaconHeaderMerkleSubtree.proof],
    stateRoot,
    beaconHeaderMerkleSubtree.index,
  );

  // Verify computed beacon root
  await sszMerkleTree.verifyProof(
    [...beaconHeaderMerkleSubtree.proof],
    beaconRoot,
    stateRoot,
    beaconHeaderMerkleSubtree.index,
  );

  // ============================================================================
  // Generate BeaconProofWitness
  // ============================================================================

  const validatorContainerWitness: ValidatorContainerWitness = {
    proof: [...proofForGI2, gi3Root, ...beaconHeaderMerkleSubtree.proof],
    effectiveBalance: validatorContainer.effectiveBalance,
    activationEpoch: validatorContainer.activationEpoch,
    activationEligibilityEpoch: validatorContainer.activationEligibilityEpoch,
  };

  const pendingPartialWithdrawalsWitness: PendingPartialWithdrawalsWitness = {
    proof: [...proofForGI3, gi2Root, ...beaconHeaderMerkleSubtree.proof],
    pendingPartialWithdrawals: pendingPartialWithdrawals,
  };

  const timestamp = await setBeaconBlockRoot(beaconRoot);

  const beaconProofWitness: BeaconProofWitness = {
    childBlockTimestamp: BigInt(timestamp),
    proposerIndex: BigInt(beaconHeader.proposerIndex),
    validatorContainerWitness: validatorContainerWitness,
    pendingPartialWithdrawalsWitness: pendingPartialWithdrawalsWitness,
  };

  const eip4788Witness: EIP4788Witness = {
    blockRoot: beaconRoot,
    gIFirstValidator: ACTIVE_0X01_VALIDATOR_PROOF.gIFirstValidator,
    beaconBlockHeader: beaconHeader,
    validatorIndex: ACTIVE_0X01_VALIDATOR_PROOF.witness.validatorIndex,
    pubkey: validatorContainer.pubkey,
    withdrawalCredentials,
    beaconProofWitness: beaconProofWitness,
  };

  return eip4788Witness;
};

export const generateLidoUnstakePermissionlessWitness = async (
  sszMerkleTree: SSZMerkleTree,
  verifier: TestValidatorContainerProofVerifier,
  withdrawalAddress: string,
  effectiveBalance: bigint = parseUnits("100", "gwei"),
  pendingPartialWithdrawalsGwei: bigint[] = [],
  randomPendingPartialWithdrawalsCount: number = 0,
): Promise<{
  eip4788Witness: EIP4788Witness;
  pubkey: string;
  validatorIndex: bigint;
  slot: bigint;
  pendingPartialWithdrawals: PendingPartialWithdrawal[];
}> => {
  const eip4788Witness = await generateEIP4478Witness(
    sszMerkleTree,
    verifier,
    withdrawalAddress,
    effectiveBalance,
    pendingPartialWithdrawalsGwei,
    randomPendingPartialWithdrawalsCount,
  );

  // Get all pending partial withdrawals from the witness
  const { validatorIndex } = eip4788Witness;
  const { slot } = eip4788Witness.beaconBlockHeader;
  return {
    eip4788Witness,
    validatorIndex,
    slot: BigInt(slot),
    pubkey: eip4788Witness.pubkey,
    pendingPartialWithdrawals:
      eip4788Witness.beaconProofWitness.pendingPartialWithdrawalsWitness.pendingPartialWithdrawals,
  };
};

// Got from this tx - https://hoodi.etherscan.io/tx/0x765837701107347325179c5510959482686456b513776346977617062c294522
// export const ACTIVE_0X02_VALIDATOR = {
//   blockRoot: "0xfbf3b28e51fc89c0c19d8cb3a1f07c250eaee4636385414ae06e180810996191",
//   gIFirstValidator: "0x0000000000000000000000000000000000000000000000000096000000000028",
//   beaconBlockHeader: {
//     slot: 1333408,
//     proposerIndex: "1024600",
//     parentRoot: "0x65136e7c4be80e655e6129b149243379667f8313b77e8bc0f250de8c85df49db",
//     stateRoot: "0x895c3ef806f6f4ab60794f59f80b1eef521ea422c7e135cbb13ae21263845b1a",
//     bodyRoot: "0x333d6031b14d52117f60014892c4893cb73c05231e0c89cc8e1551df7fe6e28f",
//   },
//   witness: {
//     validatorIndex: 1222814n,
//     validator: {
//       pubkey: "0xad8965664bc156ecd1a21ae277dd5b7deb9b6f494f2645aac95528dfd1453ccaab9291cad5d1c0660646d902ab658cb7",
//       withdrawalCredentials: "0x020000000000000000000000ecb181f8607abc3f4fc3a599f8500d40a1f228ec",
//       effectiveBalance: 32000000000n,
//       activationEligibilityEpoch: 46084n,
//       activationEpoch: 46090n,
//       exitEpoch: 18446744073709551615n,
//       withdrawableEpoch: 18446744073709551615n,
//       slashed: false,
//     },
//     // Should be proof for the Validator Container
//     proof: [
//       // "0x083d993e840e913d2cb98799d46c5631598dfbb681c1af772a0d5fb62a301f97",
//       // "0x2c84ba62dc4e7011c24fb0878e3ef2245a9e2cf2cacbbaf2978a4efa47037283",
//       "0x22aa3425047f9c6e9b0502b5c60d5f3695339a7f47f48d90e5276fc989083671",
//       "0x6217b598443727999c73fadc7b37144d9cb8876b0309aca8345718cadc7f61cf",
//       "0x9fc5f0b960be0cb4ae032289bf13620fabf742a403dbaf34920c8253f2e49cc9",
//       "0xdbceb5635a94718cd16a3d58b769bdebfa2227d48df6044218e481e9bfefa37e",
//       "0x5df93c239801f15c47197d02af8dabebff8f178884ea45a8aeca5639b99db997",
//       "0x9a55d08b1b67ab7e1ed6ffbe14ba8c17c4fc4c0abf2c83401cd7be1b82be2b71",
//       "0xd88ddfeed400a8755596b21942c1497e114c302e6118290f91e6772976041fa1",
//       "0xd0284afb3609f38e3773138f33d4bc6063f748c83d01a16f0d3589d02d0751db",
//       "0x26846476fd5fc54a5d43385167c95144f2643f533cc85bb9d16b782f8d7db193",
//       "0x506d86582d252405b840018792cad2bf1259f1ef5aa5f887e13cb2f0094f51e1",
//       "0xffff0ad7e659772f9534c195c815efc4014ef1e1daed4404c06385d11192e92b",
//       "0xf82dd03320a6c097ad8311304e89343042fa6397a327dd627e5fe2381fbb1560",
//       "0xb7d05f875f140027ef5118a2247bbb84ce8f2f0f1123623085daf7960c329f5f",
//       "0xaf7ad7369d77feb1b240ac12c461e317c0dfb43289731acfbb31dadb5793580a",
//       "0xb58d900f5e182e3c50ef74969ea16c7726c549757cc23523c369587da7293784",
//       "0x1ab3b5c00f73c9086e88cff1a17035f355d74b9657ff508175427e0ca45f4c85",
//       "0x8fe6b1689256c0d385f42f5bbe2027a22c1996e110ba97c171d3e5948de92beb",
//       "0x86fbd026185408ab9fb33a698a468b5c294c1f50068b82dae934bb09e4257a6c",
//       "0x95eec8b2e541cad4e91de38385f2e046619f54496c2382cb6cacd5b98c26f5a4",
//       "0xf893e908917775b62bff23294dbbe3a1cd8e6cc1c35b4801887b646a6f81f17f",
//       "0xeb21bddb4062641984fed45ffaa7b89fdb02e02323bf842fbaa2689e6d36212b",
//       "0x8a8d7fe3af8caa085a7639a832001457dfb9128a8061142ad0335629ff23ff9c",
//       "0xfeb3c337d7a51a6fbf00b9e34c52e1c9195c969bd4e7a0bfd51d5c5bed9c1167",
//       "0xe71f0aa83cc32edfbefa9f4d3e0174ca85182eec9f3a09f6a6c0df6377a510d7",
//       "0x31206fa80a50bb6abe29085058f16212212a60eec8f049fecb92d8c8e0a84bc0",
//       "0x21352bfecbeddde993839f614c3dac0a3ee37543f9b412b16199dc158e23b544",
//       "0x619e312724bb6d7c3153ed9de791d764a366b389af13c58bf8a8d90481a46765",
//       "0x7cdd2986268250628d0c10e385c58c6191e6fbe05191bcc04f133f2cea72c1c4",
//       "0x848930bd7ba8cac54661072113fb278869e07bb8587f91392933374d017bcbe1",
//       "0x8869ff2c22b28cc10510d9853292803328be4fb0e80495e8bb8d271f5b889636",
//       "0xb5fe28e79f1b850f8658246ce9b6a1e7b49fc06db7143e8fe0b4f2b0c5523a5c",
//       "0x985e929f70af28d0bdd1a90a808f977f597c7c778c489e98d3bd8910d31ac0f7",
//       "0xc6f67e02e6e4e1bdefb994c6098953f34636ba2b6ca20a4721d2b26a886722ff",
//       "0x1c9a7e5ff1cf48b4ad1582d3f4e4a1004f3b20d8c5a2b71387a4254ad933ebc5",
//       "0x2f075ae229646b6f6aed19a5e372cf295081401eb893ff599b3f9acc0c0d3e7d",
//       "0x328921deb59612076801e8cd61592107b5c67c79b846595cc6320c395b46362c",
//       "0xbfb909fdb236ad2411b4e4883810a074b840464689986c3f8a8091827e17c327",
//       "0x55d8fb3687ba3ba49f342c77f5a1f89bec83d811446e1a467139213d640b6a74",
//       "0xf7210d4f8e7e1039790e7bf4efa207555a10a6db1dd4b95da313aaa88b88fe76",
//       "0xad21b516cbc645ffe34ab5de1c8aef8cd4e7f8d2b51e8e1456adc7563cda206f",
//       "0xaba8120000000000000000000000000000000000000000000000000000000000",
//       "0x7c67000000000000000000000000000000000000000000000000000000000000",
//       "0xf6edabb80cc0d25b4d0e8ba53e1e8a0f6b9b91213a3bca55dbb2dd1661f224ed",
//       "0x794c6c8b156380dfcf2c558b457aaf77a1f059a6f06587f94e73c140cb1a84ad",
//       "0xee437c561340d685e3ac323f5e1c105a8893ee544baa34e8ba254d7e3a75b83c",
//       "0x0e68c1b4a96842b53f4d1160bc0e3d74f7732d23b29aa37f0b1f10f77d477225",
//       "0x31e51151bb6cecd737c458f1f17799ff94f5dfb7e56a4d235096c6711fd01d35",
//       // "0x65136e7c4be80e655e6129b149243379667f8313b77e8bc0f250de8c85df49db",
//       // "0x74aec4537af1c57333b3f7b16ff62fb53f04e29a7fd1e6563143997f7f786936",
//       // "0x15412c764a122c7377c0e6bc249d679ae27712c8115245e277678eecb00cb590",
//     ],
//   },
// };

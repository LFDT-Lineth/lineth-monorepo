import { describe, expect, it } from "@jest/globals";
import { NonceManager } from "ethers";
import {
  getMessageSentEventFromLogs,
  sendMessage,
  waitForEvents,
  wait,
  getBlockByNumberOrBlockTag,
  etherToWei,
  generateRoleAssignments,
  convertStringToPaddedHexBytes,
  getEvents,
} from "./common/utils";
import { config } from "./config/tests-config";
import { deployContract } from "./common/deployments";
import { LineaRollupV6__factory } from "./typechain";
import { getInitializerData, upgradeContractAndCall } from "./common/upgrades";
import {
  LINEA_ROLLUP_PAUSE_TYPES_ROLES,
  LINEA_ROLLUP_UNPAUSE_TYPES_ROLES,
  LINEA_ROLLUP_V6_ROLES,
} from "./common/constants";

const l1AccountManager = config.getL1AccountManager();

describe("Submission and finalization test suite", () => {
  const sendMessages = async () => {
    const messageFee = etherToWei("0.0001");
    const messageValue = etherToWei("0.0051");
    const destinationAddress = "0x8D97689C9818892B700e27F316cc3E41e17fBeb9";

    const l1MessageSender = new NonceManager(await l1AccountManager.generateAccount());
    const lineaRollup = config.getLineaRollupContract();

    console.log("Sending messages on L1");

    // Send L1 messages
    const l1MessagesPromises = [];

    for (let i = 0; i < 5; i++) {
      l1MessagesPromises.push(
        sendMessage(
          l1MessageSender,
          lineaRollup,
          {
            to: destinationAddress,
            fee: messageFee,
            calldata: "0x",
          },
          {
            value: messageValue,
          },
        ),
      );
    }

    const l1Receipts = await Promise.all(l1MessagesPromises);

    console.log("Messages sent on L1.");

    // Extract message events
    const l1Messages = getMessageSentEventFromLogs(lineaRollup, l1Receipts);

    return { l1Messages, l1Receipts };
  };

  describe("Contracts v5", () => {
    it.concurrent(
      "Check L2 anchoring",
      async () => {
        const lineaRollup = config.getLineaRollupContract();
        const l2MessageService = config.getL2MessageServiceContract();

        const { l1Messages } = await sendMessages();

        // Wait for the last L1->L2 message to be anchored on L2
        const lastNewL1MessageNumber = l1Messages.slice(-1)[0].messageNumber;

        console.log("Waiting for the anchoring using rolling hash...");
        const [rollingHashUpdatedEvent] = await waitForEvents(
          l2MessageService,
          l2MessageService.filters.RollingHashUpdated(),
          1_000,
          0,
          "latest",
          async (events) => events.filter((event) => event.args.messageNumber >= lastNewL1MessageNumber),
        );

        const [lastNewMessageRollingHash, lastAnchoredL1MessageNumber] = await Promise.all([
          lineaRollup.rollingHashes(rollingHashUpdatedEvent.args.messageNumber),
          l2MessageService.lastAnchoredL1MessageNumber(),
        ]);
        expect(lastNewMessageRollingHash).toEqual(rollingHashUpdatedEvent.args.rollingHash);
        expect(lastAnchoredL1MessageNumber).toEqual(rollingHashUpdatedEvent.args.messageNumber);

        console.log("New anchoring using rolling hash done.");
      },
      150_000,
    );

    it.concurrent(
      "Check L1 data submission and finalization",
      async () => {
        const lineaRollup = config.getLineaRollupContract();

        const [currentL2BlockNumber, startingRootHash] = await Promise.all([
          lineaRollup.currentL2BlockNumber(),
          lineaRollup.stateRootHashes(await lineaRollup.currentL2BlockNumber()),
        ]);

        console.log("Waiting for data submission used to finalize with proof...");
        // Waiting for data submission starting from migration block number
        await waitForEvents(
          lineaRollup,
          lineaRollup.filters.DataSubmittedV2(undefined, currentL2BlockNumber + 1n),
          1_000,
        );

        console.log("Waiting for the first DataFinalized event with proof...");
        // Waiting for first DataFinalized event with proof
        const [dataFinalizedEvent] = await waitForEvents(
          lineaRollup,
          lineaRollup.filters.DataFinalized(undefined, startingRootHash),
          1_000,
        );

        const [lastBlockFinalized, newStateRootHash] = await Promise.all([
          lineaRollup.currentL2BlockNumber(),
          lineaRollup.stateRootHashes(dataFinalizedEvent.args.lastBlockFinalized),
        ]);

        expect(lastBlockFinalized).toBeGreaterThanOrEqual(dataFinalizedEvent.args.lastBlockFinalized);
        expect(newStateRootHash).toEqual(dataFinalizedEvent.args.finalRootHash);
        expect(dataFinalizedEvent.args.withProof).toBeTruthy();

        console.log("Finalization with proof done.");
      },
      150_000,
    );

    it.concurrent(
      "Check L2 safe/finalized tag update on sequencer",
      async () => {
        const sequencerEndpoint = config.getSequencerEndpoint();
        if (!sequencerEndpoint) {
          console.log('Skipped the "Check L2 safe/finalized tag update on sequencer" test');
          return;
        }

        const lastFinalizedL2BlockNumberOnL1 = 0;
        console.log(`lastFinalizedL2BlockNumberOnL1=${lastFinalizedL2BlockNumberOnL1}`);

        let safeL2BlockNumber = -1,
          finalizedL2BlockNumber = -1;
        while (
          safeL2BlockNumber < lastFinalizedL2BlockNumberOnL1 ||
          finalizedL2BlockNumber < lastFinalizedL2BlockNumberOnL1
        ) {
          safeL2BlockNumber =
            (await getBlockByNumberOrBlockTag(sequencerEndpoint, "safe"))?.number || safeL2BlockNumber;
          finalizedL2BlockNumber =
            (await getBlockByNumberOrBlockTag(sequencerEndpoint, "finalized"))?.number || finalizedL2BlockNumber;
          await wait(1_000);
        }

        console.log(`safeL2BlockNumber=${safeL2BlockNumber} finalizedL2BlockNumber=${finalizedL2BlockNumber}`);

        expect(safeL2BlockNumber).toBeGreaterThanOrEqual(lastFinalizedL2BlockNumberOnL1);
        expect(finalizedL2BlockNumber).toBeGreaterThanOrEqual(lastFinalizedL2BlockNumberOnL1);

        console.log("L2 safe/finalized tag update on sequencer done.");
      },
      150_000,
    );
  });

  describe("LineaRollup v6 upgrade", () => {
    // note: we cannot move the chain forward by 6 months, so in theory, this could be any address for e2e test purposes
    const fallbackoperatorAddress = "0xcA11bde05977b3631167028862bE2a173976CA11";

    beforeAll(async () => {
      const l1DeployerAccount = l1AccountManager.whaleAccount(0);
      const l1SecurityCouncil = l1AccountManager.whaleAccount(3);
      const l1Provider = config.getL1Provider();
      const proxyAdmin = config.getLineaRollupProxyAdminContract(l1DeployerAccount);

      console.log("Deploying LineaRollup v6 implementation contract...");
      const { maxFeePerGas, maxPriorityFeePerGas } = await l1Provider.getFeeData();

      const lineaRollupV6Implementation = await deployContract(new LineaRollupV6__factory(), l1DeployerAccount, [
        { maxPriorityFeePerGas, maxFeePerGas },
      ]);

      console.log("Upgrading LineaRollup contract to V6...");
      const newRoleAddresses = generateRoleAssignments(LINEA_ROLLUP_V6_ROLES, await l1SecurityCouncil.getAddress(), []);

      const initializerData = getInitializerData(
        LineaRollupV6__factory.createInterface(),
        "reinitializeLineaRollupV6",
        [newRoleAddresses, LINEA_ROLLUP_PAUSE_TYPES_ROLES, LINEA_ROLLUP_UNPAUSE_TYPES_ROLES, fallbackoperatorAddress],
      );

      await upgradeContractAndCall(
        l1DeployerAccount,
        await proxyAdmin.getAddress(),
        await config.getLineaRollupContract().getAddress(),
        await lineaRollupV6Implementation.getAddress(),
        initializerData,
      );
    });

    it("Check LineaRollupVersionChanged and FallbackOperatorAddressSet events are emitted", async () => {
      const lineaRollupAddress = await config.getLineaRollupContract().getAddress();
      const lineaRollupV6 = LineaRollupV6__factory.connect(lineaRollupAddress, config.getL1Provider());

      console.log("Waiting for FallbackOperatorAddressSet event...");
      await waitForEvents(
        lineaRollupV6,
        lineaRollupV6.filters.FallbackOperatorAddressSet(undefined, fallbackoperatorAddress),
        1_000,
      );

      console.log("Waiting for LineaRollupVersionChanged event...");
      const expectedVersion5Bytes8 = convertStringToPaddedHexBytes("5.0", 8);
      const expectedVersion6Bytes8 = convertStringToPaddedHexBytes("6.0", 8);
      await waitForEvents(
        lineaRollupV6,
        lineaRollupV6.filters.LineaRollupVersionChanged(expectedVersion5Bytes8, expectedVersion6Bytes8),
        1_000,
      );
    });

    it("Check all new roles have been granted and pauseTypes/unpauseTypes are assigned to specific roles", async () => {
      const lineaRollupAddress = await config.getLineaRollupContract().getAddress();
      const l1SecurityCouncilAddress = await l1AccountManager.whaleAccount(3).getAddress();

      const lineaRollupV6 = LineaRollupV6__factory.connect(lineaRollupAddress, config.getL1Provider());

      for (const role of LINEA_ROLLUP_V6_ROLES) {
        const hasRole = await lineaRollupV6.hasRole(role, l1SecurityCouncilAddress);
        expect(hasRole).toBeTruthy();
      }

      const [pauseTypeRoleSetEvents, unPauseTypeRoleSetEvents] = await Promise.all([
        getEvents(lineaRollupV6, lineaRollupV6.filters.PauseTypeRoleSet()),
        getEvents(lineaRollupV6, lineaRollupV6.filters.UnPauseTypeRoleSet()),
      ]);

      expect(pauseTypeRoleSetEvents.length).toEqual(LINEA_ROLLUP_PAUSE_TYPES_ROLES.length);
      expect(unPauseTypeRoleSetEvents.length).toEqual(LINEA_ROLLUP_UNPAUSE_TYPES_ROLES.length);
    });

    it("Check L1 data submission and finalization", async () => {
      const lineaRollupAddress = await config.getLineaRollupContract().getAddress();
      const lineaRollupV6 = LineaRollupV6__factory.connect(lineaRollupAddress, config.getL1Provider());

      const currentL2BlockNumber = await lineaRollupV6.currentL2BlockNumber();

      console.log("Waiting for DataSubmittedV3 used to finalize with proof...");
      await waitForEvents(lineaRollupV6, lineaRollupV6.filters.DataSubmittedV3(), 1_000);

      console.log("Waiting for the first DataFinalizedV3 event with proof...");
      const [dataFinalizedEvent] = await waitForEvents(
        lineaRollupV6,
        lineaRollupV6.filters.DataFinalizedV3(currentL2BlockNumber + 1n),
        1_000,
      );

      const [lastBlockFinalized, newStateRootHash] = await Promise.all([
        lineaRollupV6.currentL2BlockNumber(),
        lineaRollupV6.stateRootHashes(dataFinalizedEvent.args.endBlockNumber),
      ]);

      expect(lastBlockFinalized).toBeGreaterThanOrEqual(dataFinalizedEvent.args.endBlockNumber);
      expect(newStateRootHash).toEqual(dataFinalizedEvent.args.finalStateRootHash);

      console.log("Finalization with proof done.");
    }, 150_000);
  });
});

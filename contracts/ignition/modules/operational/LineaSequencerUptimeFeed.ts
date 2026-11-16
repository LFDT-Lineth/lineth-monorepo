import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

/**
 * Parameters: `initialStatus`, `admin`, `feedUpdater`.
 */
export default buildModule("LineaSequencerUptimeFeed", (m) => {
  const lineaSequencerUptimeFeed = m.contract(
    "LineaSequencerUptimeFeed",
    [m.getParameter<boolean>("initialStatus"), m.getParameter<string>("admin"), m.getParameter<string>("feedUpdater")],
    { id: "LineaSequencerUptimeFeed" },
  );
  return { lineaSequencerUptimeFeed };
});

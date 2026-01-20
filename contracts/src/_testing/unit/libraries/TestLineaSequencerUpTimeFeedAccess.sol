// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { AggregatorV2V3Interface } from "@chainlink/contracts/src/v0.8/shared/interfaces/AggregatorV2V3Interface.sol";

contract TestLineaSequencerUptimeFeedAccess {
  AggregatorV2V3Interface private target;

  constructor(address _targetAddress) {
    target = AggregatorV2V3Interface(_targetAddress);
  }

  function callLatestAnswer() external view {
    target.latestAnswer();
  }

  function callLatestRoundData() external view {
    target.latestRoundData();
  }

  function callLatestTimestamp() external view {
    target.latestTimestamp();
  }

  function callLatestRound() external view {
    target.latestRound();
  }

  function callGetAnswer(uint256 roundId) external view {
    target.getAnswer(roundId);
  }

  function callGetTimestamp(uint256 roundId) external view {
    target.getTimestamp(roundId);
  }

  function callGetRoundData(uint80 roundId) external view {
    target.getRoundData(roundId);
  }
}

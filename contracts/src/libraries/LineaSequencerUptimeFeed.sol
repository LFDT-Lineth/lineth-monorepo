// SPDX-License-Identifier: MIT
pragma solidity ^0.8.4;
 
import {AggregatorInterface} from "@chainlink/contracts/src/v0.8/shared/interfaces/AggregatorInterface.sol";
import {AggregatorV3Interface} from "@chainlink/contracts/src/v0.8/shared/interfaces/AggregatorV3Interface.sol";
import {AggregatorV2V3Interface} from "@chainlink/contracts/src/v0.8/shared/interfaces/AggregatorV2V3Interface.sol";
import {TypeAndVersionInterface} from "@chainlink/contracts/src/v0.8/interfaces/TypeAndVersionInterface.sol";
import {LineaSequencerUptimeFeedInterface} from "./LineaSequencerUptimeFeedInterface.sol";
import {SimpleReadAccessController} from "@chainlink/contracts/src/v0.8/shared/access/SimpleReadAccessController.sol";


/**
* @title LineaSequencerUptimeFeed - L2 sequencer uptime status aggregator
* @notice L2 contract that receives status updates,
*  and records a new answer if the status changed
*/
contract LineaSequencerUptimeFeed is
 AggregatorV2V3Interface,
 LineaSequencerUptimeFeedInterface,
 TypeAndVersionInterface,
 SimpleReadAccessController
{

 /// @dev Packed state struct to save sloads
 struct FeedState {
	 uint80 latestRoundId; // Dummy roundId for backward compatibility
	 	 /* 
		  latestStatus: A variable with a value of either true or false
		  false: The sequencer is up
		  true: The sequencer is down
		 */
	 bool latestStatus; 
	 uint64 startedAt;
	 uint64 updatedAt;
 }

 /// @notice Sender is not allowed to update the status
 error InvalidSender();

 /// @dev Emitted when an `updateStatus` call is ignored due to unchanged status or stale timestamp
 event UpdateIgnored(bool latestStatus, uint64 latestTimestamp, bool incomingStatus, uint64 incomingTimestamp);
 /// @dev Emitted when a updateStatus is called without the status changing
 event RoundUpdated(int256 status, uint64 updatedAt);

 // solhint-disable-next-line chainlink-solidity/all-caps-constant-storage-variables
 uint8 public constant override decimals = 0;
 // solhint-disable-next-line chainlink-solidity/all-caps-constant-storage-variables
 string public constant override description = "L2 Sequencer Uptime Status Feed";
 // solhint-disable-next-line chainlink-solidity/all-caps-constant-storage-variables
 uint256 public constant override version = 1;
 
 /// @dev s_latestRoundId == 0 means this contract is uninitialized.
 FeedState private s_feedState = FeedState({latestRoundId: 0, latestStatus: false, startedAt: 0, updatedAt: 0});

 /**
  * @param initialStatus The initial status of the feed
  */
 constructor(bool initialStatus) {
   uint64 timestamp = uint64(block.timestamp);

   // Initialise dummy roundId == 0 
   _recordRound(initialStatus, timestamp);
 }


 /**
  * @notice versions:
  *
  * - LineaSequencerUptimeFeed 1.0.0: initial release
  *
  * @inheritdoc TypeAndVersionInterface
  */
 function typeAndVersion() external pure virtual override returns (string memory) {
   return "LineaSequencerUptimeFeed 1.0.0";
 }


 /**
  * @dev Returns an AggregatorV2V3Interface compatible answer from status flag
  *
  * @param status The status flag to convert to an aggregator-compatible answer
  */
 function _getStatusAnswer(bool status) private pure returns (int256) {
   return status ? int256(1) : int256(0);
 }

 /**
  * @notice Helper function to check if the sender is allowed to update the status.
  *
  * @param sender Sender address
  */
 function isValidSender(address sender) private returns (bool) {
   // Optional: Implement this function for access control.
	 return true;
 }

 /**
  * @notice Helper function to record a round and set the latest feed state.
  *
  * @param status Sequencer status
  * @param timestamp The L2 block timestamp of status update
  */
 function _recordRound(bool status, uint64 timestamp) private {
   uint64 updatedAt = uint64(block.timestamp);
   FeedState memory feedState = FeedState(0, status, timestamp, updatedAt);
   s_feedState = feedState;
   emit AnswerUpdated(_getStatusAnswer(status), 0, timestamp);
 }

 /**
  * @notice Helper function to update when a round was last updated
  *
  * @param status Sequencer status
  */
 function _updateRound(bool status) private {
   uint64 updatedAt = uint64(block.timestamp);
   s_feedState.updatedAt = updatedAt;
   emit RoundUpdated(_getStatusAnswer(status), updatedAt);
 }

 /**
  * @notice Record a new status and timestamp if it has changed since the last round.
  * @dev This function will revert if not called from `l1Sender` via the L1->L2 messenger.
  *
  * @param status Sequencer status
  * @param timestamp Block timestamp of status update
  */
 function updateStatus(bool status, uint64 timestamp) external override {
   FeedState memory feedState = s_feedState;
	 // Optional access control check
   if (!isValidSender(msg.sender)) {
     revert InvalidSender();
   }

   // Ignore if latest recorded timestamp is newer
   if (feedState.startedAt > timestamp) {
     emit UpdateIgnored(feedState.latestStatus, feedState.startedAt, status, timestamp);
     return;
   }

   if (feedState.latestStatus == status) {
     _updateRound(status);
   } else {
     _recordRound(status, timestamp);
   }
 }

 /// @inheritdoc AggregatorInterface
 function latestAnswer() external view override checkAccess returns (int256) {
   FeedState memory feedState = s_feedState;
   return _getStatusAnswer(feedState.latestStatus);
 }
 
 /// @inheritdoc AggregatorV3Interface
 function latestRoundData()
	   external
	   view
	   override
	   checkAccess
	   returns (uint80 roundId, int256 answer, uint256 startedAt, uint256 updatedAt, uint80 answeredInRound)
	 {
	   FeedState memory feedState = s_feedState;
	
	   roundId = feedState.latestRoundId;
	   answer = _getStatusAnswer(feedState.latestStatus);
	   startedAt = feedState.startedAt;
	   updatedAt = feedState.updatedAt;
	   answeredInRound = roundId;
	 }

   function getAnswer(uint256 roundId) external view override returns (int256) {
     return _getStatusAnswer(s_feedState.latestStatus);
   }

   function getRoundData(uint80 _roundId) external view override returns (uint80 roundId, int256 answer, uint256 startedAt, uint256 updatedAt, uint80 answeredInRound) {
     return (0, _getStatusAnswer(s_feedState.latestStatus), s_feedState.startedAt, s_feedState.updatedAt, 0);
   }

   function getTimestamp(uint256 roundId) external view override returns (uint256) {
     return s_feedState.updatedAt;
   }

   function latestTimestamp() external view override returns (uint256) {
     return s_feedState.updatedAt;
   }

   function latestRound() external view override returns (uint256) {
     return s_feedState.latestRoundId;
   }
}

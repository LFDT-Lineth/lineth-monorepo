// SPDX-License-Identifier: MIT
pragma solidity ^0.4.24;

import "./FundsSender.sol";

contract FundsSenderRoot {
    function invokeFundsSender(
        address _contractFS,
        address _contractFR1,
        address _contractFR2,
        bool _mustRevert,
        FundsSender.CallCase _callCase
    ) external {
        FundsSender(_contractFS).transferFunds(
            _contractFR1,
            _contractFR2,
            _mustRevert,
            _callCase
        );
    }
}

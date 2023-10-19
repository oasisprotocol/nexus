// SPDX-License-Identifier: Apache-2.0

pragma solidity ^0.8.18;

error E(uint16 n, string s);

contract CustomErrors {
    function test() public pure {
        revert E(1, "a");
    }
}

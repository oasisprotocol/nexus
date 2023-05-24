// SPDX-License-Identifier: Apache-2.0

pragma solidity ^0.8.18;

struct O {
    uint16 n;
    string s;
}

interface Varied {
    function test(
        int8 i8,
        uint8 u8,
        int256 i,
        uint256 u,
        bool b,
        // not supported in go-ethereum
        // fixed128x18 f18,
        // ufixed128x18 uf18,
        bytes32 b32,
        address a,
        function (uint16) external returns (uint16) f,
        uint16[2] calldata xy,
        bytes calldata buf,
        string calldata s,
        uint16[] calldata l,
        O calldata o
    ) external;
}

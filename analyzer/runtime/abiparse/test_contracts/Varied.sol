// SPDX-License-Identifier: Apache-2.0

pragma solidity ^0.8.18;

struct O {
    uint16 n;
    string s;
}

error E(address a, uint16 n);

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
        uint8[2] calldata xy8,
        bytes calldata buf,
        string calldata s,
        uint16[] calldata l,
        uint8[] calldata l8,
        O calldata o
    ) external;
    function testUnnamed(uint16, uint16) external returns (uint16, uint16);
    event TestUnnamed(uint16 indexed, uint16 indexed, uint16, uint16);
}

// SPDX-License-Identifier: GPL-3.0

pragma solidity >=0.8.2 <0.9.0;

contract EcDataTestContract {
    uint256 constant p  = 0x30644e72e131a029b85045b68181585d97816a916871ca8d3c208c16d87cfd47;

    // generator of G2
    uint256 constant p2xRe = 0x1800DEEF121F1E76426A00665E5C4479674322D4F75EDADD46DEBD5CD992F6ED;
    uint256 constant p2xIm = 0x198E9393920D483A7260BFB731FB5D25F1AA493335A9E71297E485B7AEF312C2;
    uint256 constant p2yRe = 0x12C85EA5DB8C6DEB4AAB71808DCB408FE3D1E7690C43D37B4CE6CC0166FA7DAA;
    uint256 constant p2yIm = 0x90689D0585FF075EC9E99AD690C3395BC4B313370B38EF355ACDADCD122975B;

    struct Pairing {
        uint256 x1;
        uint256 y1;
        uint256 x2Re;
        uint256 x2Im;
        uint256 y2Re;
        uint256 y2Im;
    }

    constructor() {

       // EcRecover
        assert(ecRecover(1, 27, 1, 0xf0000, -2));
        assert(ecRecover(1, 25, 0, 1, 0));
        assert(ecRecover(1, 30, 1, 1, 0));
        assert(ecRecover(1, 27, 1, type(uint256).max, 0));

        // EcAdd
        assert(ecAdd(1, p-2, 1, p-2, 3));
        assert(ecAdd(0, 0, 1, 2, 0));
        assert(!ecAdd(0, 1, 1, p-2, 0));
        assert(!ecAdd(0, 0, 1, p+1, 0));

        // EcMul
        assert(ecMul(1, p-2, 0xf00, -1));
        assert(ecMul(1, p-2, 0, 0)); 
        assert(ecMul(0, 0, 3, 0));
        assert(!ecMul(0, 4, 3, 0));
        assert(!ecMul(0, 4, 0, 0));

        // EcPairing
        Pairing memory validPairing0 = Pairing({x1: 1, y1: p-2, x2Re: 0, x2Im: 0, y2Re: 0, y2Im: 0}); 
        Pairing memory validPairing1 = Pairing({x1: 1, y1: p-2, x2Re: p2xRe, x2Im: p2xIm, y2Re: p2yRe, y2Im: p2yIm}); 
        Pairing memory validPairing2 = Pairing({x1: 0, y1: 0, x2Re: p2xRe, x2Im: p2xIm, y2Re: p2yRe, y2Im: p2yIm}); 
        
        Pairing memory invalidPairing0 = Pairing({x1: 0, y1: 0, x2Re: 1, x2Im: p2xIm, y2Re: p2yRe, y2Im: p2yIm}); 
        Pairing memory invalidPairing1 = Pairing({x1: 0, y1: 12, x2Re: p2xRe, x2Im: p2xIm, y2Re: p2yRe, y2Im: p2yIm}); 
        
        Pairing[] memory pairings = new Pairing[](0);
        assert(ecPairing(pairings, 0));

        pairings = new Pairing[](1);
        pairings[0] = validPairing0;
        assert(ecPairing(pairings, 0));
        pairings[0] = validPairing0;

        pairings = new Pairing[](2);
        pairings[0] = validPairing0;
        pairings[1] = validPairing1;
        assert(ecPairing(pairings, 0));
        assert(!ecPairing(pairings, 10));
        pairings[1] = invalidPairing0;
        assert(!ecPairing(pairings, 0));

        pairings = new Pairing[](3);
        pairings[0] = validPairing0;
        pairings[1] = validPairing1;
        pairings[2] = validPairing2;
        assert(ecPairing(pairings, 0));
        assert(!ecPairing(pairings, 1));
        assert(!ecPairing(pairings, -1));
        pairings[2] = invalidPairing1;
        assert(!ecPairing(pairings, 0));

        pairings = new Pairing[](10);
        for(uint i; i < pairings.length; i++) {
            if(i%3 == 0) {
                pairings[i] = validPairing0;
            } else if(i%3 == 1) {
                pairings[i] = validPairing1;
            } else {
                pairings[i] = validPairing2;
            }
        }
        assert(ecPairing(pairings, 0));
        
        string memory res = "zk-evm is life";

        assembly {
            return(add(res, 32), mload(res))
        }
    }

    function ecRecover(uint256 hash, uint256 v, uint256 r, uint256 s, int256 sizeBias) internal view returns (bool success) {
        assembly {
            let ptr := mload(0x40)
            mstore(ptr, hash)
            mstore(add(ptr, 32), v)
            mstore(add(ptr, 64), r)
            mstore(add(ptr, 96), s)
            success := staticcall(3000, 0x1, ptr, add(128, sizeBias), 0, 0)
            mstore(0x40, add(ptr, 128))
        }
    }

    function ecAdd(uint256 x1, uint256 y1, uint256 x2, uint256 y2, int256 sizeBias) internal view returns (bool success) {
        assembly {
            let ptr := mload(0x40)
            mstore(ptr, x1)
            mstore(add(ptr, 32), y1)
            mstore(add(ptr, 64), x2)
            mstore(add(ptr, 96), y2)
            success := staticcall(150, 0x6, ptr, add(128, sizeBias), 0, 0)
            mstore(0x40, add(ptr, 128))
        }
    }

    function ecMul(uint256 x, uint256 y, uint256 s, int256 sizeBias) internal view returns (bool success) {
        assembly {
            let ptr := mload(0x40)
            mstore(ptr, x)
            mstore(add(ptr, 32), y)
            mstore(add(ptr, 64), s)
            success := staticcall(6000, 0x7, ptr, add(96, sizeBias), 0, 0)
            mstore(0x40, add(ptr, 96))
        }
    }

    function ecPairing(Pairing[] memory inputs, int256 sizeBias) internal view returns (bool success) {
        uint256 gasForCall = 34000 * inputs.length + 45000;
        uint256 ptr;
        assembly {
            ptr := mload(0x40)
        }

        for (uint256 i; i < inputs.length; i++) {
            uint256 x1 = inputs[i].x1;
            uint256 y1 = inputs[i].y1;
            uint256 x2Re = inputs[i].x2Re;
            uint256 x2Im = inputs[i].x2Im;
            uint256 y2Re = inputs[i].y2Re;
            uint256 y2Im = inputs[i].y2Im;
            assembly {
                let j := mul(192, i)
                mstore(add(ptr, j), x1)
                mstore(add(ptr, add(j, 32)), y1)
                mstore(add(ptr, add(j, 64)), x2Im)
                mstore(add(ptr, add(j, 96)), x2Re)
                mstore(add(ptr, add(j, 128)), y2Im)
                mstore(add(ptr, add(j, 160)), y2Re)
            }
        }
        uint256 size = inputs.length * 192;
        uint256 newPtr = ptr + size;
        assembly {
            success := staticcall(gasForCall, 0x8, ptr, add(size, sizeBias), 0, 0)
            mstore(0x40, newPtr)
        }
    }
}
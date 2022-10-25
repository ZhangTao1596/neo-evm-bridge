using System;
using System.Numerics;
using Neo;

namespace EvmLayerContract
{
    public class DepositState
    {
        public UInt256 TxHash;
        public UInt160 Address;
        public BigInteger Amount;
        public UInt160 To;
    }
}

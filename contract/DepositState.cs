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

        public byte[] Serialize()
        {
            var data = (byte[])TxHash;
            data = Helper.BytesConcat(data, (byte[])Address);
            var num = Amount.ToByteArray();
            data = Helper.BytesAppend(data, (byte)num.Length);
            data = Helper.BytesConcat(data, num);
            data = Helper.BytesConcat(data, (byte[])To);
            return data;
        }
    }
}

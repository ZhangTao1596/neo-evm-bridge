using System;
using Neo;
using Neo.Cryptography.ECC;
using Neo.SmartContract.Framework;

namespace ManageContract
{
    public class ValidatorsState
    {
        public UInt256 TxHash;
        public ECPoint[] Validators;

        public byte[] Serialize()
        {
            var data = (byte[])TxHash;
            data = Util.BytesAppend(data, (byte)Validators.Length);
            foreach (var p in Validators)
            {
                data = Util.BytesAppend(data, (byte)p.Length);
                data = Helper.Concat(data, (byte[])p);
            }
            return data;
        }

        public ECPoint[] Deserialize(byte[] data)
        {
            if (data.Length < 33) throw new Exception("invalid validators state");
            TxHash = (UInt256)data[..32];
            int offset = 32;
            var count = data[offset++];
            var r = new ECPoint[count];
            for (int i = 0; i < count && offset < data.Length; i++)
            {
                var len = data[offset++];
                if (offset + len > data.Length) throw new Exception("unexpected end of bytes");
                r[i] = (ECPoint)data[offset..(offset + len)];
                offset += len;
            }
            return r;
        }
    }
}

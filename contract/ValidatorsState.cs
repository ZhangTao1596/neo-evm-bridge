using System;
using Neo;
using Neo.Cryptography.ECC;

namespace EvmLayerContract
{
    public class ValidatorsState
    {
        public UInt256 TxHash;
        public ECPoint[] Validators;

        public byte[] Serialize()
        {
            var data = (byte[])TxHash;
            data = BytesAppend(data, (byte)Validators.Length);
            foreach (var p in Validators)
            {
                data = BytesAppend(data, (byte)p.Length);
                data = BytesConcat(data, (byte[])p);
            }
            return data;
        }

        private byte[] BytesAppend(byte[] a, byte x)
        {
            var r = new byte[a.Length + 1];
            int i = 0;
            for (; i < a.Length; i++)
                r[i] = a[i];
            r[i] = x;
            return r;
        }

        private byte[] BytesConcat(byte[] a, byte[] b)
        {
            var r = new byte[a.Length + b.Length];
            int i = 0;
            for (; i < a.Length; i++)
                r[i] = a[i];
            for (; i < a.Length + b.Length; i++)
                r[i] = b[i - a.Length];
            return r;
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

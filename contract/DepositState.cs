using System;
using System.Numerics;
using Neo;
using Neo.SmartContract.Framework;

namespace ManageContract
{
    public class DepositState
    {
        public UInt256 TxHash;
        public UInt160 From;
        public BigInteger Amount;
        public UInt160 To;

        public void Serialize(BufferWriter writer)
        {
            writer.WriteUint256(TxHash);
            writer.WriteUint160(From);
            writer.WriteVarBytes(Amount.ToByteArray());
            writer.WriteUint160(To);
        }

        public void Deserialize(BufferReader reader)
        {
            TxHash = reader.ReadUint256();
            From = reader.ReadUint160();
            Amount = new BigInteger(reader.ReadVarBytes());
            To = reader.ReadUint160();
        }

        public byte[] ToByteArray()
        {
            var writer = new BufferWriter();
            Serialize(writer);
            return writer.GetBytes();
        }

        public static DepositState FromByteArray(byte[] b)
        {
            var ds = new DepositState();
            var reader = new BufferReader(b);
            ds.Deserialize(reader);
            return ds;
        }
    }
}

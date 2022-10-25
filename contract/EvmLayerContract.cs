using System;
using System.ComponentModel;
using System.Numerics;
using Neo;
using Neo.Cryptography.ECC;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;

namespace EvmLayerContract
{
    [DisplayName("EvmLayerContract")]
    [ManifestExtra("Author", "NGD")]
    [ManifestExtra("Email", "developer@neo.ngd.org")]
    [ManifestExtra("Description", "This is a contract in Neo for evm layer")]
    public class EvmLayerContract : SmartContract
    {
        private const byte depositedPrefix = 0x01;
        private static readonly byte[] OwnerKey = new byte[] { 0x02 };
        private static readonly byte[] ValidatorsKey = new byte[] { 0x03 };
        private static readonly byte[] DepositIdKey = new byte[] { 0x04 };

        public delegate void OnDeployedDelegate(UInt160 owner);
        public static event OnDeployedDelegate OnDeployed;
        public delegate void OnDepositedDelegate(BigInteger id, UInt160 from, BigInteger amount, UInt160 to);
        public static event OnDepositedDelegate OnDeposited;
        public delegate void OnValidatorsChangedDelegate(ECPoint[] validators);
        public static event OnValidatorsChangedDelegate OnValidatorsChanged;

        public static string Name()
        {
            return "EvmLayerContract";
        }

        public static void _deploy(object _, bool isUpdate)
        {
            if (Runtime.CallingScriptHash != ContractManagement.Hash)
                throw new Exception($"{nameof(_deploy)} can't be called directly");
            if (!isUpdate)
            {
                var owner = ((Transaction)Runtime.ScriptContainer).Sender;
                Storage.Put(Storage.CurrentContext, OwnerKey, owner);
                OnDeployed(owner);
            }
        }

        private static BigInteger NewDepositId()
        {
            var context = Storage.CurrentContext;
            var id = (BigInteger)Storage.Get(context, DepositIdKey);
            Storage.Put(context, DepositIdKey, id + 1);
            return id;
        }

        public static void OnNEP17Payment(UInt160 from, BigInteger amount, object data)
        {
            if (Runtime.CallingScriptHash != GAS.Hash || from == null)
                throw new Exception("only receive gas");
            Deposit(from, amount, data);
        }

        private static void Deposit(UInt160 from, BigInteger amount, object data)
        {
            var to = (UInt160)data;
            if (!to.IsValid || to.IsZero)
                throw new Exception("invalid address on evm layer");
            var depositedMap = new StorageMap(depositedPrefix);
            var txHash = ((Transaction)Runtime.ScriptContainer).Hash;
            var state = new DepositState
            {
                TxHash = txHash,
                Address = from,
                Amount = amount,
                To = to,
            };
            var id = NewDepositId();
            depositedMap.Put((ByteString)id, StdLib.Serialize(state));
            OnDeposited(id, from, amount, to);
        }

        /// Need test
        public static bool Verify()
        {
            var owner = (UInt160)Storage.Get(Storage.CurrentContext, OwnerKey);
            return Runtime.CheckWitness(owner);
        }

        public static void DesignateValidators(ECPoint[] pks)
        {
            if (!Verify())
                throw new Exception("permission denied");
            if (!ECPointsCheck(pks))
                throw new Exception("invalid public key");
            var ps = ECPointUnique(pks);
            if (pks.Length != 1 && pks.Length != 4 && pks.Length != 7)// Consistency check with side chain config
                throw new Exception("invalid validators count");
            OnValidatorsChanged(ps);
            Storage.Put(Storage.CurrentContext, ValidatorsKey, ECPointsSerialize(ps));
        }

        private static bool ECPointsCheck(ECPoint[] ps)
        {
            foreach (var p in ps)
                if (!p.IsValid) return false;
            return true;
        }

        private static ECPoint[] ECPointUnique(ECPoint[] ps)
        {
            for (int i = 0; i < ps.Length; i++)
            {
                for (int j = i + 1; j < ps.Length; j++)
                {
                    if (ECPointEqual(ps[i], ps[j]))
                        ps = ECPointsRemove(ps, j);
                }
            }
            return ps;
        }

        private static bool ECPointEqual(ECPoint a, ECPoint b)
        {
            if (a.Length != b.Length) return false;
            for (int i = 0; i < a.Length; i++)
            {
                if (a[i] != b[i]) return false;
            }
            return true;
        }

        private static ECPoint[] ECPointsRemove(ECPoint[] ps, int index)
        {
            if (index >= ps.Length) throw new Exception($"{nameof(ECPointsRemove)} {nameof(index)}");
            var r = new ECPoint[ps.Length - 1];
            for (int i = 0, j = 0; i < ps.Length; i++)
            {
                if (i != index)
                    r[j++] = ps[i];
            }
            return r;
        }

        private static byte[] ECPointsSerialize(ECPoint[] ps)
        {
            var data = new byte[] { (byte)ps.Length };
            foreach (var p in ps)
            {
                data = BytesAppend(data, (byte)p.Length);
                data = BytesConcat(data, (byte[])p);
            }
            return data;
        }

        private static ECPoint[] ECPointDeserialize(byte[] data)
        {
            if (data.Length < 1) throw new Exception("invalid raw ECPoints");
            int offset = 0;
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

        private static byte[] BytesAppend(byte[] a, byte x)
        {
            var r = new byte[a.Length + 1];
            int i = 0;
            for (; i < a.Length; i++)
                r[i] = a[i];
            r[i] = x;
            return r;
        }

        private static byte[] BytesConcat(byte[] a, byte[] b)
        {
            var r = new byte[a.Length + b.Length];
            int i = 0;
            for (; i < a.Length; i++)
                r[i] = a[i];
            for (; i < a.Length + b.Length; i++)
                r[i] = b[i - a.Length];
            return r;
        }

        public static void Update(ByteString nefFile, string manifest)
        {
            ContractManagement.Update(nefFile, manifest, null);
        }

        public static UInt160 Owner()
        {
            return (UInt160)Storage.Get(Storage.CurrentContext, OwnerKey);
        }

        public static ECPoint[] Validators()
        {
            var raw = (byte[])Storage.Get(Storage.CurrentContext, ValidatorsKey);
            if (raw != null && raw.Length > 0)
            {
                return ECPointDeserialize(raw);
            }
            return new ECPoint[0];
        }

        public static BigInteger Deposited(UInt160 address)
        {
            var depositedMap = new StorageMap(depositedPrefix);
            return (BigInteger)depositedMap.Get(address);
        }
    }
}

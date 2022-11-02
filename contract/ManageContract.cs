using System;
using System.ComponentModel;
using System.Numerics;
using Neo;
using Neo.Cryptography.ECC;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;

namespace ManageContract
{
    [DisplayName("EvmLayerContract")]
    [ManifestExtra("Author", "NGD")]
    [ManifestExtra("Email", "developer@neo.ngd.org")]
    [ManifestExtra("Description", "This is a contract in Neo for evm layer")]
    public class ManageContract : SmartContract
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
            depositedMap.Put((ByteString)id, (ByteString)state.Serialize());
            OnDeposited(id, from, amount, to);
        }

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
            var txHash = ((Transaction)Runtime.ScriptContainer).Hash;
            var state = new ValidatorsState
            {
                TxHash = txHash,
                Validators = ps,
            };
            Storage.Put(Storage.CurrentContext, ValidatorsKey, state.Serialize());
            OnValidatorsChanged(ps);
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
                var state = new ValidatorsState();
                state.Deserialize(raw);
                return state.Validators;
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

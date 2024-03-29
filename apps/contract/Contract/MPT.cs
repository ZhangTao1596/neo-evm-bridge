using System;
using Neo;
using Neo.SmartContract.Framework;

namespace Bridge
{
    public class MPT
    {
        class ProofWithKey
        {
            public byte[] Key;
            public byte[][] Proof;

            public void Deserialize(byte[] b)
            {
                var reader = new BufferReader(b);
                Key = reader.ReadVarBytes();
                var count = reader.ReadVarUint();
                Proof = new byte[count][];
                for (int i = 0; i < (int)count; i++)
                    Proof[i] = reader.ReadVarBytes();
            }
        }

        enum NodeType : byte
        {
            Branch = 0,
            Extension = 1,
            Leaf = 2,
            Hash = 3,
            Empty = 4,
        }

        class Node
        {
            public NodeType Type;
            public byte[] Key;
            public byte[] Value;
            public UInt256 Hash;
            public Node[] Children = new Node[16];

            public void Deserialize(BufferReader reader)
            {
                Type = (NodeType)reader.ReadByte();
                switch (Type)
                {
                    case NodeType.Branch:
                        {
                            for (int i = 0; i <= 16; i++)
                            {
                                var n = new Node();
                                n.Deserialize(reader);
                                Children[i] = n;
                            }
                            break;
                        }
                    case NodeType.Extension:
                        {
                            Key = reader.ReadVarBytes();
                            var n = new Node();
                            n.Deserialize(reader);
                            Children[0] = n;
                            break;
                        }
                    case NodeType.Leaf:
                        Value = reader.ReadVarBytes();
                        break;
                    case NodeType.Hash:
                        Hash = (UInt256)reader.ReadBytes(32);
                        break;
                    case NodeType.Empty:
                    default:
                        throw new Exception("MPT: invalid node type");
                }
            }
        }

        public static bool VerifyProof(byte[] proof, UInt256 root, out byte[] key, out byte[] value)
        {
            var proofWithKey = new ProofWithKey();
            proofWithKey.Deserialize(proof);
            key = proofWithKey.Key;
            var store = new Map<UInt256, byte[]>();
            foreach (var n in proofWithKey.Proof)
            {
                store[(UInt256)Util.DoubleSha256(n)] = n;
            }
            var path = ToNibbles(proofWithKey.Key);
            return Get(store, root, path, out value);
        }

        private static byte[] ToNibbles(byte[] path)
        {
            var result = new byte[path.Length * 2];
            for (int i = 0; i < path.Length; i++)
            {
                result[i * 2] = (byte)(path[i] >> 4);
                result[i * 2 + 1] = (byte)(path[i] & 0x0F);
            }
            return result;
        }

        private static bool Get(Map<UInt256, byte[]> store, UInt256 root, byte[] path, out byte[] value)
        {
            path = ToNibbles(path);
            value = null;
            Node n = new()
            {
                Type = NodeType.Hash,
                Hash = root,
            };
            var offset = 0;
            while (true)
            {
                switch (n.Type)
                {
                    case NodeType.Branch:
                        {
                            if (offset >= path.Length)
                                return false;
                            n = n.Children[path[offset]];
                            offset += 1;
                            break;
                        }
                    case NodeType.Extension:
                        {
                            if (!Util.StartWith(path, n.Key))
                                return false;
                            n = n.Children[0];
                            offset += n.Key.Length;
                            break;
                        }
                    case NodeType.Leaf:
                        {
                            if (offset < path.Length)
                                return false;
                            value = n.Value;
                            return true;
                        }
                    case NodeType.Hash:
                        {
                            var raw = store[n.Hash];
                            if (raw is null) return false;
                            var reader = new BufferReader(raw);
                            n.Deserialize(reader);
                            break;
                        }
                    case NodeType.Empty:
                        return false;
                    default:
                        throw new Exception("MPT: invalid node type");
                }
            }

        }
    }
}

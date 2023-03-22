using Bridge;
using Microsoft.VisualStudio.TestTools.UnitTesting;

namespace ContractTest
{
    [TestClass]
    public class TestBufferReader
    {
        [TestMethod]
        public void TestReadByte()
        {
            var reader = new BufferReader(new byte[] { 1 });
            Assert.AreEqual(1, reader.ReadByte());
            Assert.ThrowsException<Exception>(() => reader.ReadByte());
        }

        [TestMethod]
        public void ReadBytes()
        {
            var reader = new BufferReader(new byte[] { 3, 0, 1, 2 });
            var a = reader.ReadBytes(3);
            Assert.AreEqual(3, a.Length);
            Assert.AreEqual(3, a[0]);
            Assert.AreEqual(1, a[2]);
            reader.ReadBytes(1);
        }

        [TestMethod]
        public void ReadUint16()
        {
            var reader = new BufferReader(BitConverter.GetBytes(UInt16.MaxValue));
            Assert.AreEqual(UInt16.MaxValue, reader.ReadUint16());
        }

        [TestMethod]
        public void ReadUint32()
        {
            var reader = new BufferReader(BitConverter.GetBytes(UInt32.MaxValue));
            Assert.AreEqual(UInt32.MaxValue, reader.ReadUint32());
        }

        [TestMethod]
        public void ReadUint64()
        {
            var reader = new BufferReader(BitConverter.GetBytes(UInt64.MaxValue));
            Assert.AreEqual(UInt64.MaxValue, reader.ReadUint64());
        }

        [TestMethod]
        public void TestReadVarInt()
        {
            var b = new byte[] { 0xf0 };
            var reader = new BufferReader(b);
            Assert.AreEqual((ulong)0xf0, reader.ReadVarUint());
            b = new byte[] { 0xfd, 0xff, 0xff };
            reader = new BufferReader(b);
            Assert.AreEqual((ulong)0xffff, reader.ReadVarUint());
            b = new byte[] { 0xfe, 0xff, 0xff, 0xff, 0xff };
            reader = new BufferReader(b);
            Assert.AreEqual((ulong)0xffffffff, reader.ReadVarUint());
            b = new byte[] { 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff };
            reader = new BufferReader(b);
            Assert.AreEqual((ulong)0xffffffffffffffff, reader.ReadVarUint());
        }

        [TestMethod]
        public void TestReadVarBytes()
        {
            var b = new byte[] { 2, 1, 2 };
            var reader = new BufferReader(b);
            Assert.AreEqual("0102", reader.ReadVarBytes().ToHexString());
        }

        [TestMethod]
        public void TestReaded()
        {
            var b = new byte[] { 2, 1, 2, 3 };
            var reader = new BufferReader(b);
            reader.ReadBytes(3);
            Assert.AreEqual("020102", reader.Readed().ToHexString());
        }
    }
}

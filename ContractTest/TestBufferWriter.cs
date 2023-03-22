using Bridge;
using Microsoft.VisualStudio.TestTools.UnitTesting;

namespace ContractTest
{
    [TestClass]
    public class TestBufferWriter
    {
        [TestMethod]
        public void TestWriteByte()
        {
            var writer = new BufferWriter();
            writer.WriteByte(1);
            Assert.AreEqual("01", writer.GetBytes().ToHexString());
        }

        [TestMethod]
        public void TestWriteBytes()
        {
            var writer = new BufferWriter();
            writer.WriteBytes(new byte[] { 1, 2, 3, 4 });
            Assert.AreEqual("01020304", writer.GetBytes().ToHexString());
        }

        [TestMethod]
        public void TestWriteVarUint()
        {
            var writer = new BufferWriter();
            writer.WriteVarUint(0xf0);
            Assert.AreEqual("f0", writer.GetBytes().ToHexString());
            writer = new BufferWriter();
            writer.WriteVarUint(0xfd);
            Assert.AreEqual("fdfd00", writer.GetBytes().ToHexString());
            writer = new BufferWriter();
            writer.WriteVarUint(0xff);
            Assert.AreEqual("fdff00", writer.GetBytes().ToHexString());
            writer = new BufferWriter();
            writer.WriteVarUint(0xffff);
            Assert.AreEqual("fdffff", writer.GetBytes().ToHexString());
            writer = new BufferWriter();
            writer.WriteVarUint(0xffffffff);
            Assert.AreEqual("feffffffff", writer.GetBytes().ToHexString());
            writer = new BufferWriter();
            writer.WriteVarUint(0xffffffffffffffff);
            Assert.AreEqual("ffffffffffffffffff", writer.GetBytes().ToHexString());
        }

        [TestMethod]
        public void TestWriteVarBytes()
        {
            var writer = new BufferWriter();
            writer.WriteVarBytes(new byte[] { 1, 2, 3 });
            Assert.AreEqual("03010203", writer.GetBytes().ToHexString());
        }

        [TestMethod]
        public void WriteRead()
        {
            var writer = new BufferWriter();
            writer.WriteVarUint(65536);
            var reader = new BufferReader(writer.GetBytes());
            Assert.AreEqual((ulong)65536, reader.ReadVarUint());
        }

        [TestMethod]
        public void TestResize()
        {
            var writer = new BufferWriter();
            var b = new byte[200];
            writer.WriteBytes(b);
            writer.WriteBytes(b);
            Assert.AreEqual(400, writer.GetBytes().Length);
        }
    }
}

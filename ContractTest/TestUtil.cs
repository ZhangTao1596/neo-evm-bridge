using ManageContract;
using Microsoft.VisualStudio.TestTools.UnitTesting;

namespace ContractTest
{
    [TestClass]
    public class TestUtil
    {
        [TestMethod]
        public void TestUint16()
        {
            var b = BitConverter.GetBytes(UInt16.MaxValue);
            var n = Util.UInt16FromLittleEndian(b);
            Assert.AreEqual(UInt16.MaxValue, n);
            b = BitConverter.GetBytes(UInt16.MaxValue / 2);
            n = Util.UInt16FromLittleEndian(b);
            Assert.AreEqual(UInt16.MaxValue / 2, n);

            b = Util.UInt16ToLittleEndian(UInt16.MaxValue);
            n = BitConverter.ToUInt16(b);
            Assert.AreEqual(UInt16.MaxValue, n);
        }

        [TestMethod]
        public void TestUint32()
        {
            var b = BitConverter.GetBytes(UInt32.MaxValue);
            var n = Util.UInt32FromLittleEndian(b);
            Assert.AreEqual(UInt32.MaxValue, n);
            b = BitConverter.GetBytes(UInt32.MaxValue / 2);
            n = Util.UInt32FromLittleEndian(b);
            Assert.AreEqual(UInt32.MaxValue / 2, n);

            b = Util.UInt32ToLittleEndian(UInt32.MaxValue);
            n = BitConverter.ToUInt32(b);
            Assert.AreEqual(UInt32.MaxValue, n);
        }

        [TestMethod]
        public void TestUint64()
        {
            var b = BitConverter.GetBytes(UInt64.MaxValue);
            var n = Util.UInt64FromLittleEndian(b);
            Assert.AreEqual(UInt64.MaxValue, n);
            b = BitConverter.GetBytes(UInt64.MaxValue / 2);
            n = Util.UInt64FromLittleEndian(b);
            Assert.AreEqual(UInt64.MaxValue / 2, n);

            b = Util.UInt64ToLittleEndian(UInt64.MaxValue);
            n = BitConverter.ToUInt64(b);
            Assert.AreEqual(UInt64.MaxValue, n);
        }
    }
}

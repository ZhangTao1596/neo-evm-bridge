using Bridge;
using Microsoft.VisualStudio.TestTools.UnitTesting;

namespace ContractTest
{
    [TestClass]
    public class TestUtil
    {
        [TestMethod]
        public void TestArrayCopy()
        {
            var a = new byte[] { 1, 2, 3 };
            var b = new byte[4];
            Util.ArrayCopy(b, 1, a, 0, a.Length);
            Assert.AreEqual(1, b[1]);
            Assert.AreEqual(3, b[3]);
            Util.ArrayCopy(b, 1, a, 1, 2);
            Assert.AreEqual(2, b[1]);
            Assert.AreEqual(3, b[2]);
        }

        [TestMethod]
        public void BytesAppend()
        {
            var a = new byte[] { 1, 2 };
            a = Util.BytesAppend(a, 3);
            Assert.AreEqual(3, a.Length);
            Assert.AreEqual(3, a[2]);
        }

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

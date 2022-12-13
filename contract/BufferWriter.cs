using System;
using Neo;

namespace ManageContract
{
    public class BufferWriter
    {
        private byte[] buffer = new byte[1024];
        private int offset = 0;

        private void ResizeBuffer()
        {
            var newBuffer = new byte[buffer.Length * 2];
            Util.ArrayCopy(buffer, 0, newBuffer, 0, buffer.Length);
            buffer = newBuffer;
        }

        public void WriteByte(byte b)
        {
            WriteBytes(new byte[] { b });
        }

        public void WriteBytes(byte[] b)
        {
            while (offset + b.Length > buffer.Length)
                ResizeBuffer();
            Util.ArrayCopy(b, 0, buffer, offset, b.Length);
            offset += b.Length;
        }

        public void WriteVarUint(UInt64 value)
        {
            if (value < 0xfd)
            {
                WriteByte((byte)value);
                return;
            }
            if (value < 0xffff)
            {
                WriteByte(0xfd);
                WriteBytes(Util.UInt16ToLittleEndian((UInt16)value));
                return;
            }
            if (value < 0xffffffff)
            {
                WriteByte(0xfe);
                WriteBytes(Util.UInt32ToLittleEndian((UInt32)value));
                return;
            }
            WriteByte(0xff);
            WriteBytes(Util.UInt64ToLittleEndian(value));
        }

        public void WriteVarBytes(byte[] b)
        {
            WriteVarUint((UInt64)b.Length);
            WriteBytes(b);
        }

        public void WriteUint256(UInt256 hash)
        {
            WriteBytes((byte[])hash);
        }

        public void WriteUint160(UInt160 hash)
        {
            WriteBytes((byte[])hash);
        }

        public byte[] GetBytes()
        {
            return buffer[..offset];
        }
    }
}

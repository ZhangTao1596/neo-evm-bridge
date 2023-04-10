using System;
using Neo;

namespace Bridge
{
    public class BufferReader
    {
        private readonly byte[] buffer;
        private int offset;

        public BufferReader(byte[] b)
        {
            buffer = b;
            offset = 0;
        }

        public byte ReadByte()
        {
            if (buffer.Length <= offset)
                throw new Exception("[BufferReader] unexpected EOF");
            return buffer[offset++];
        }

        public byte[] ReadBytes(int n)
        {
            if (buffer.Length < offset + n)
                throw new Exception("[BufferReader] unexpected EOF");
            var value = buffer[offset..(offset + n)];
            offset += n;
            return value;
        }

        public UInt16 ReadUint16()
        {
            if (buffer.Length < offset + 2)
                throw new Exception("[BufferReader] unexpected EOF");
            var value = Util.UInt16FromLittleEndian(buffer[offset..(offset + 2)]);
            offset += 2;
            return value;
        }

        public UInt32 ReadUint32()
        {
            if (buffer.Length < offset + 4)
                throw new Exception("[BufferReader] unexpected EOF");
            var value = Util.UInt32FromLittleEndian(buffer[offset..(offset + 4)]);
            offset += 4;
            return value;
        }

        public UInt64 ReadUint64()
        {
            if (buffer.Length < offset + 8)
                throw new Exception("[BufferReader] unexpected EOF");
            var value = Util.UInt64FromLittleEndian(buffer[offset..(offset + 8)]);
            offset += 8;
            return value;
        }

        public UInt64 ReadVarUint()
        {
            var b = ReadByte();
            return b switch
            {
                0xfd => ReadUint16(),
                0xfe => ReadUint32(),
                0xff => ReadUint64(),
                _ => b,
            };
        }

        public UInt160 ReadUint160()
        {
            var b = ReadBytes(20);
            return (UInt160)b;
        }

        public UInt256 ReadUint256()
        {
            var b = ReadBytes(32);
            return (UInt256)b;
        }

        public byte[] ReadVarBytes()
        {
            var len = (int)ReadVarUint();
            if (buffer.Length < offset + len)
                throw new Exception("[BufferReader] unexpect EOF");
            return ReadBytes(len);
        }

        public byte[] Readed()
        {
            return buffer[..offset];
        }
    }
}

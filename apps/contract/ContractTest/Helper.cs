using System.Globalization;
using System.Text;

namespace ContractTest
{
    public static class Helper
    {
        public static byte[] HexToBytes(this string value)
        {
            if (value == null || value.Length == 0)
                return Array.Empty<byte>();
            if (value.Length % 2 == 1)
                throw new FormatException();
            byte[] result = new byte[value.Length / 2];
            for (int i = 0; i < result.Length; i++)
                result[i] = byte.Parse(value.Substring(i * 2, 2), NumberStyles.AllowHexSpecifier);
            return result;
        }

        public static string ToHexString(this byte[] value)
        {
            StringBuilder sb = new();
            foreach (byte b in value)
                sb.AppendFormat("{0:x2}", b);
            return sb.ToString();
        }
    }
}

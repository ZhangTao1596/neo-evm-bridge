namespace EvmLayerContract
{
    public static class Helper
    {
        public static byte[] BytesAppend(byte[] a, byte x)
        {
            var r = new byte[a.Length + 1];
            int i = 0;
            for (; i < a.Length; i++)
                r[i] = a[i];
            r[i] = x;
            return r;
        }

        public static byte[] BytesConcat(byte[] a, byte[] b)
        {
            var r = new byte[a.Length + b.Length];
            int i = 0;
            for (; i < a.Length; i++)
                r[i] = a[i];
            for (; i < a.Length + b.Length; i++)
                r[i] = b[i - a.Length];
            return r;
        }
    }
}

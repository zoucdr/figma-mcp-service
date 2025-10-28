using UnityEngine;

namespace Figma2UGUI
{
    /// <summary>
    /// 颜色工具类
    /// </summary>
    public static class ColorUtils
    {
        /// <summary>
        /// 将十六进制颜色字符串转换为Unity颜色
        /// </summary>
        public static Color HexToColor(string hex)
        {
            if (string.IsNullOrEmpty(hex))
                return Color.white;

            if (hex.StartsWith("#"))
                hex = hex.Substring(1);

            if (hex.Length == 6)
                hex += "FF"; // 添加完全不透明的Alpha值

            if (hex.Length != 8)
                return Color.white;

            int r = int.Parse(hex.Substring(0, 2), System.Globalization.NumberStyles.HexNumber);
            int g = int.Parse(hex.Substring(2, 2), System.Globalization.NumberStyles.HexNumber);
            int b = int.Parse(hex.Substring(4, 2), System.Globalization.NumberStyles.HexNumber);
            int a = int.Parse(hex.Substring(6, 2), System.Globalization.NumberStyles.HexNumber);

            return new Color(r / 255f, g / 255f, b / 255f, a / 255f);
        }

        /// <summary>
        /// 将Unity颜色转换为十六进制字符串
        /// </summary>
        public static string ColorToHex(Color color)
        {
            int r = Mathf.RoundToInt(color.r * 255f);
            int g = Mathf.RoundToInt(color.g * 255f);
            int b = Mathf.RoundToInt(color.b * 255f);
            int a = Mathf.RoundToInt(color.a * 255f);

            return string.Format("#{0:X2}{1:X2}{2:X2}{3:X2}", r, g, b, a);
        }

        /// <summary>
        /// 调整颜色亮度
        /// </summary>
        /// <param name="color">原始颜色</param>
        /// <param name="factor">亮度因子，大于1变亮，小于1变暗</param>
        public static Color AdjustBrightness(FColor color, float factor)
        {
            return new Color(
                Mathf.Clamp01(color.r * factor),
                Mathf.Clamp01(color.g * factor),
                Mathf.Clamp01(color.b * factor),
                color.a
            );
        }

        /// <summary>
        /// 创建渐变颜色
        /// </summary>
        public static Color Lerp(Color a, Color b, float t)
        {
            return Color.Lerp(a, b, t);
        }

        /// <summary>
        /// 获取颜色的HSV值
        /// </summary>
        public static void RGBToHSV(Color color, out float h, out float s, out float v)
        {
            Color.RGBToHSV(color, out h, out s, out v);
        }

        /// <summary>
        /// 从HSV值创建颜色
        /// </summary>
        public static Color HSVToRGB(float h, float s, float v)
        {
            return Color.HSVToRGB(h, s, v);
        }
    }
}

using System;
using System.Collections.Generic;
using UnityEngine;
using UnityEngine.UI;
using TMPro;
using UnityEditor;

namespace Figma2UGUI
{
    /// <summary>
    /// 文本处理工具类，提供文本相关的通用处理方法
    /// </summary>
    public class TextProcessor
    {
        /// <summary>
        /// 将Figma文本对齐方式转换为Unity Text对齐方式
        /// </summary>
        public static TextAnchor FigmaAlignmentToUnityTextAnchor(string horizontalAlignment, string verticalAlignment)
        {
            // 默认为中间对齐
            TextAnchor alignment = TextAnchor.MiddleCenter;

            // 水平对齐
            if (string.IsNullOrEmpty(horizontalAlignment))
                horizontalAlignment = "CENTER";

            // 垂直对齐
            if (string.IsNullOrEmpty(verticalAlignment))
                verticalAlignment = "CENTER";

            // 组合水平和垂直对齐
            if (horizontalAlignment.ToUpper() == "LEFT")
            {
                if (verticalAlignment.ToUpper() == "TOP")
                    alignment = TextAnchor.UpperLeft;
                else if (verticalAlignment.ToUpper() == "CENTER")
                    alignment = TextAnchor.MiddleLeft;
                else if (verticalAlignment.ToUpper() == "BOTTOM")
                    alignment = TextAnchor.LowerLeft;
            }
            else if (horizontalAlignment.ToUpper() == "CENTER")
            {
                if (verticalAlignment.ToUpper() == "TOP")
                    alignment = TextAnchor.UpperCenter;
                else if (verticalAlignment.ToUpper() == "CENTER")
                    alignment = TextAnchor.MiddleCenter;
                else if (verticalAlignment.ToUpper() == "BOTTOM")
                    alignment = TextAnchor.LowerCenter;
            }
            else if (horizontalAlignment.ToUpper() == "RIGHT")
            {
                if (verticalAlignment.ToUpper() == "TOP")
                    alignment = TextAnchor.UpperRight;
                else if (verticalAlignment.ToUpper() == "CENTER")
                    alignment = TextAnchor.MiddleRight;
                else if (verticalAlignment.ToUpper() == "BOTTOM")
                    alignment = TextAnchor.LowerRight;
            }

            return alignment;
        }

        /// <summary>
        /// 将Figma文本对齐方式转换为TextMeshPro对齐方式
        /// </summary>
        public static TextAlignmentOptions FigmaAlignmentToTMPAlignment(string horizontalAlignment, string verticalAlignment)
        {
            int alignment = 0;

            // 处理垂直对齐
            if (string.IsNullOrEmpty(verticalAlignment))
                verticalAlignment = "CENTER";

            alignment += (verticalAlignment.ToUpper() == "TOP" ? 1 : 0) << 8;
            alignment += (verticalAlignment.ToUpper() == "CENTER" ? 1 : 0) << 9;
            alignment += (verticalAlignment.ToUpper() == "BOTTOM" ? 1 : 0) << 10;

            // 处理水平对齐
            if (string.IsNullOrEmpty(horizontalAlignment))
                horizontalAlignment = "CENTER";

            alignment += (horizontalAlignment.ToUpper() == "LEFT" ? 1 : 0) << 0;
            alignment += (horizontalAlignment.ToUpper() == "CENTER" ? 1 : 0) << 1;
            alignment += (horizontalAlignment.ToUpper() == "RIGHT" ? 1 : 0) << 2;
            alignment += (horizontalAlignment.ToUpper() == "JUSTIFIED" ? 1 : 0) << 3;

            return (TextAlignmentOptions)alignment;
        }

        /// <summary>
        /// 将Figma字体样式转换为Unity字体样式
        /// </summary>
        public static FontStyle FigmaFontStyleToUnityFontStyle(int fontWeight, string textDecoration)
        {
            FontStyle fontStyle = FontStyle.Normal;

            // 处理粗体
            if (fontWeight >= 700) // 700及以上通常被视为Bold
            {
                fontStyle = FontStyle.Bold;
            }

            // 处理斜体（Figma可能没有直接提供斜体信息，这里作为扩展）
            if (textDecoration != null && textDecoration.ToUpper() == "ITALIC")
            {
                if (fontStyle == FontStyle.Bold)
                    fontStyle = FontStyle.BoldAndItalic;
                else
                    fontStyle = FontStyle.Italic;
            }

            return fontStyle;
        }

        /// <summary>
        /// 将Figma字体样式转换为TextMeshPro字体样式
        /// </summary>
        public static FontStyles FigmaFontStyleToTMPFontStyle(int fontWeight, string textDecoration, string textCase)
        {
            FontStyles fontStyle = FontStyles.Normal;

            // 处理粗体
            if (fontWeight >= 700)
            {
                fontStyle |= FontStyles.Bold;
            }

            // 处理下划线和删除线
            if (!string.IsNullOrEmpty(textDecoration))
            {
                if (textDecoration.ToUpper() == "UNDERLINE")
                    fontStyle |= FontStyles.Underline;
                else if (textDecoration.ToUpper() == "STRIKETHROUGH")
                    fontStyle |= FontStyles.Strikethrough;
            }

            // 处理大小写
            if (!string.IsNullOrEmpty(textCase))
            {
                if (textCase.ToUpper() == "UPPER")
                    fontStyle |= FontStyles.UpperCase;
                else if (textCase.ToUpper() == "LOWER")
                    fontStyle |= FontStyles.LowerCase;
                else if (textCase.ToUpper() == "SMALL_CAPS")
                    fontStyle |= FontStyles.SmallCaps;
            }

            return fontStyle;
        }

        /// <summary>
        /// 获取或添加Text组件到GameObject
        /// </summary>
        public static Text GetOrAddTextComponent(GameObject gameObject)
        {
            // 保存当前RectTransform的属性
            RectTransform rectTransform = gameObject.GetComponent<RectTransform>();
            Vector2 sizeDelta = rectTransform.sizeDelta;
            Vector2 anchoredPosition = rectTransform.anchoredPosition;

            // 获取或添加Text组件
            Text text = gameObject.GetComponent<Text>();
            if (text == null)
                text = gameObject.AddComponent<Text>();

            // 恢复RectTransform属性（防止添加组件时改变尺寸）
            rectTransform.sizeDelta = sizeDelta;
            rectTransform.anchoredPosition = anchoredPosition;

            return text;
        }

        /// <summary>
        /// 获取或添加TextMeshProUGUI组件到GameObject
        /// </summary>
        public static TextMeshProUGUI GetOrAddTMPComponent(GameObject gameObject)
        {
            // 保存当前RectTransform的属性
            RectTransform rectTransform = gameObject.GetComponent<RectTransform>();
            Vector2 sizeDelta = rectTransform.sizeDelta;
            Vector2 anchoredPosition = rectTransform.anchoredPosition;

            // 获取或添加TextMeshProUGUI组件
            TextMeshProUGUI tmp = gameObject.GetComponent<TextMeshProUGUI>();
            if (tmp == null)
                tmp = gameObject.AddComponent<TextMeshProUGUI>();

            // 恢复RectTransform属性（防止添加组件时改变尺寸）
            rectTransform.sizeDelta = sizeDelta;
            rectTransform.anchoredPosition = anchoredPosition;

            return tmp;
        }

        /// <summary>
        /// 应用Figma样式到Text组件
        /// </summary>
        public static void ApplyFigmaStyleToText(Text text, Style style, float scale, UIImportSetting settings)
        {
            if (text == null || style == null)
                return;

            // 设置字体大小
            text.fontSize = Mathf.RoundToInt(style.fontSize * scale);

            // 设置字体
            Font font = settings.GetMappedFont(style.fontFamily);
            if (font != null)
            {
                text.font = font;
            }

            // 设置字体样式
            text.fontStyle = FigmaFontStyleToUnityFontStyle(style.fontWeight, style.textDecoration);

            // 设置行高
            if (style.lineHeightPx > 0)
            {
                text.lineSpacing = style.lineHeightPx / style.fontSize;
            }

            // 设置对齐方式
            text.alignment = FigmaAlignmentToUnityTextAnchor(style.textAlignHorizontal, style.textAlignVertical);

            // 设置其他属性
            text.supportRichText = true;
            text.horizontalOverflow = HorizontalWrapMode.Wrap;
            text.verticalOverflow = VerticalWrapMode.Truncate;
        }

        /// <summary>
        /// 应用Figma样式到TextMeshPro组件
        /// </summary>
        public static void ApplyFigmaStyleToTMP(TextMeshProUGUI tmp, Style style, float scale, UIImportSetting settings)
        {
            if (tmp == null || style == null)
                return;

            // 设置字体大小
            tmp.fontSize = style.fontSize * scale;

            // 设置字体
            TMP_FontAsset fontAsset = settings.GetMappedTMPFont(style.fontFamily);
            if (fontAsset != null)
            {
                tmp.font = fontAsset;
            }

            // 设置字体样式
            tmp.fontStyle = FigmaFontStyleToTMPFontStyle(style.fontWeight, style.textDecoration, style.textCase);

            // 设置行高
            if (style.lineHeightPx > 0)
            {
                tmp.lineSpacing = (style.lineHeightPx / style.fontSize - 1) * 50; // TMP使用不同的行间距计算方式
            }

            // 设置对齐方式
            tmp.alignment = FigmaAlignmentToTMPAlignment(style.textAlignHorizontal, style.textAlignVertical);

            // 设置其他属性
            tmp.richText = true;
            tmp.enableWordWrapping = true;
        }
    }
}

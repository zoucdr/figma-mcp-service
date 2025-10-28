using System;
using UnityEngine;

namespace Figma2UGUI
{
    /// <summary>
    /// 组件类型常量
    /// </summary>
    public static class ComponentType
    {
        public const string NONE = "None";
        public const string BUTTON = "Button";
        public const string TOGGLE = "Toggle";
        public const string SLIDER = "Slider";
        public const string INPUTFIELD = "InputField";
        public const string SCROLLVIEW = "ScrollView";
        public const string DROPDOWN = "Dropdown";
    }

    /// <summary>
    /// 图片类型常量
    /// </summary>
    public static class ImageType
    {
        public const string SIMPLE = "Simple";
        public const string SLICED = "Sliced";
        public const string FILLED = "Filled";
        public const string TILED = "Tiled";
    }

    /// <summary>
    /// 存储UGUI特定的修改参数
    /// </summary>
    [Serializable]
    public class UIModify
    {
        // 图片相关
        public string imagePath;

        // 组件类型
        public string componentType = ComponentType.NONE;

        // 图片类型
        public string imageType = ImageType.SIMPLE;
    }
}
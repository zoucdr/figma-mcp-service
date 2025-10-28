using System;
using System.Collections.Generic;
using UnityEngine;
using UnityEditor;

namespace Figma2UGUI
{
    /// <summary>
    /// Figma到UGUI导入的全局设置
    /// </summary>
    [FilePath("ProjectSettings/Figma2UGUISettings.asset", FilePathAttribute.Location.ProjectFolder)]
    public class UIImportSetting : ScriptableSingleton<UIImportSetting>
    {
        public static UIImportSetting Instance => instance;
        [Serializable]
        public class FontMapping
        {
            public string figmaFontName;
            public Font unityFont;
            public TMPro.TMP_FontAsset tmpFont;
            public bool useTMPFont = false;
        }

        [Serializable]
        public class ImageFolderMapping
        {
            public string folderName;
            public string folderPath;
        }

        [Header("字体映射")]
        public List<FontMapping> fontMappings = new List<FontMapping>();

        [Header("图片文件夹")]
        public List<ImageFolderMapping> imageFolders = new List<ImageFolderMapping>();

        [Header("大图Texture文件夹")]
        public List<string> largeTextureFolders = new List<string>();

        [Header("默认设置")]
        public Font defaultFont;
        public TMPro.TMP_FontAsset defaultTMPFont;
        public bool useTextMeshPro = true;

        [Header("图片设置")]
        public bool compressTextures = true;
        public bool generateMipMaps = false;
        public FilterMode textureFilterMode = FilterMode.Bilinear;
        public TextureImporterFormat textureFormat = TextureImporterFormat.Automatic;

        [Header("界面设置")]
        public float defaultScale = 1.0f;
        public bool createPrefab = true;
        public string prefabSavePath = "Assets/Prefabs/UI";

        [Header("图片处理")]
        public bool mergeDuplicateImages = true;
        public bool useImageNameFromFigma = true;

        /// <summary>
        /// 保存设置
        /// </summary>
        public void Save()
        {
            Save(true);
            AssetDatabase.SaveAssets();
        }

        /// <summary>
        /// 重置为默认设置
        /// </summary>
        public void ResetToDefaults()
        {
            fontMappings.Clear();
            imageFolders.Clear();
            largeTextureFolders.Clear();
            defaultFont = null;
            defaultTMPFont = null;
            useTextMeshPro = true;
            compressTextures = true;
            generateMipMaps = false;
            textureFilterMode = FilterMode.Bilinear;
            textureFormat = TextureImporterFormat.Automatic;
            defaultScale = 1.0f;
            createPrefab = true;
            prefabSavePath = "Assets/Prefabs/UI";
            mergeDuplicateImages = true;
            useImageNameFromFigma = true;

            Save();
        }

        /// <summary>
        /// 获取匹配的Unity字体
        /// </summary>
        public Font GetMappedFont(string figmaFontName)
        {
            if (string.IsNullOrEmpty(figmaFontName))
                return defaultFont;

            foreach (var mapping in fontMappings)
            {
                if (mapping.figmaFontName.Equals(figmaFontName, StringComparison.OrdinalIgnoreCase))
                {
                    return mapping.unityFont;
                }
            }

            return defaultFont;
        }

        /// <summary>
        /// 获取匹配的TextMeshPro字体
        /// </summary>
        public TMPro.TMP_FontAsset GetMappedTMPFont(string figmaFontName)
        {
            if (string.IsNullOrEmpty(figmaFontName))
                return defaultTMPFont;

            foreach (var mapping in fontMappings)
            {
                if (mapping.figmaFontName.Equals(figmaFontName, StringComparison.OrdinalIgnoreCase) && mapping.useTMPFont)
                {
                    return mapping.tmpFont;
                }
            }

            return defaultTMPFont;
        }
    }
}
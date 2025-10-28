using System;
using System.Collections.Generic;
using System.IO;
using System.Threading.Tasks;
using UnityEditor;
using UnityEngine;

namespace Figma2UGUI
{
    /// <summary>
    /// 资源管理器，用于管理和处理导入的资源
    /// </summary>
    public class AssetManager
    {
        private UIImportSetting _settings;
        private ImageProcessor _imageProcessor;
        private Dictionary<string, UnityEngine.Object> _assetCache = new Dictionary<string, UnityEngine.Object>();

        public AssetManager()
        {
            _settings = UIImportSetting.Instance;
            _imageProcessor = new ImageProcessor();
        }

        /// <summary>
        /// 异步处理资源
        /// </summary>
        public async Task ProcessAssetsAsync(string jsonFilePath, string textureFolder, string outputFolder)
        {
            // 确保输出文件夹存在
            if (!Directory.Exists(outputFolder))
            {
                Directory.CreateDirectory(outputFolder);
            }

            // 创建图片输出文件夹
            string imageOutputFolder = Path.Combine(outputFolder, "Images");
            if (!Directory.Exists(imageOutputFolder))
            {
                Directory.CreateDirectory(imageOutputFolder);
            }

            // 处理图片
            await _imageProcessor.ProcessImageFolderAsync(textureFolder, imageOutputFolder);

            // 复制JSON文件到输出文件夹
            string jsonFileName = Path.GetFileName(jsonFilePath);
            string jsonOutputPath = Path.Combine(outputFolder, jsonFileName);
            File.Copy(jsonFilePath, jsonOutputPath, true);

            // 刷新资源数据库
            AssetDatabase.Refresh();
        }

        /// <summary>
        /// 获取图片资源路径
        /// </summary>
        public string GetImageAssetPath(string originalPath)
        {
            return _imageProcessor.GetImageAssetPath(originalPath);
        }

        /// <summary>
        /// 异步加载纹理
        /// </summary>
        public async Task<Texture2D> LoadTextureAsync(string assetPath)
        {
            return await _imageProcessor.LoadTextureAsync(assetPath);
        }

        /// <summary>
        /// 异步加载Sprite
        /// </summary>
        public async Task<Sprite> LoadSpriteAsync(string assetPath)
        {
            return await _imageProcessor.LoadSpriteAsync(assetPath);
        }

        /// <summary>
        /// 异步加载资源
        /// </summary>
        public async Task<T> LoadAssetAsync<T>(string assetPath) where T : UnityEngine.Object
        {
            if (string.IsNullOrEmpty(assetPath))
            {
                return null;
            }

            // 检查缓存
            string cacheKey = $"{typeof(T).Name}:{assetPath}";
            if (_assetCache.TryGetValue(cacheKey, out UnityEngine.Object cachedAsset))
            {
                return cachedAsset as T;
            }

            // 加载资源
            T asset = AssetDatabase.LoadAssetAtPath<T>(assetPath);
            if (asset != null)
            {
                _assetCache[cacheKey] = asset;
            }

            await Task.Yield(); // 让UI有机会更新
            return asset;
        }

        /// <summary>
        /// 创建资源
        /// </summary>
        public void CreateAsset<T>(T asset, string assetPath) where T : UnityEngine.Object
        {
            // 确保文件夹存在
            string directory = Path.GetDirectoryName(assetPath);
            if (!Directory.Exists(directory))
            {
                Directory.CreateDirectory(directory);
            }

            // 创建资源
            AssetDatabase.CreateAsset(asset, assetPath);
            AssetDatabase.SaveAssets();
            AssetDatabase.Refresh();

            // 添加到缓存
            string cacheKey = $"{typeof(T).Name}:{assetPath}";
            _assetCache[cacheKey] = asset;
        }

        /// <summary>
        /// 清除缓存
        /// </summary>
        public void ClearCache()
        {
            _assetCache.Clear();
            _imageProcessor.ClearCache();
        }
    }
}

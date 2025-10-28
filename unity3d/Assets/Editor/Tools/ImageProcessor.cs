using System;
using System.Collections.Generic;
using System.IO;
using System.Threading.Tasks;
using UnityEditor;
using UnityEngine;
using System.Linq;

namespace Figma2UGUI
{
    /// <summary>
    /// 图片处理工具类，用于处理和管理导入的图片资源
    /// </summary>
    public class ImageProcessor
    {
        private UIImportSetting _settings;
        private Dictionary<string, string> _imagePathMapping = new Dictionary<string, string>();
        private Dictionary<string, Texture2D> _textureCache = new Dictionary<string, Texture2D>();
        private Dictionary<string, Sprite> _spriteCache = new Dictionary<string, Sprite>();

        public ImageProcessor()
        {
            _settings = UIImportSetting.Instance;
        }

        /// <summary>
        /// 异步处理图片文件夹
        /// </summary>
        public async Task ProcessImageFolderAsync(string sourceFolderPath, string targetFolderPath)
        {
            if (!Directory.Exists(sourceFolderPath))
            {
                throw new DirectoryNotFoundException($"图片源文件夹不存在: {sourceFolderPath}");
            }

            // 确保目标文件夹存在
            if (!Directory.Exists(targetFolderPath))
            {
                Directory.CreateDirectory(targetFolderPath);
            }

            // 获取所有图片文件
            string[] imageFiles = Directory.GetFiles(sourceFolderPath, "*.*", SearchOption.AllDirectories)
                .Where(file =>
                {
                    string ext = Path.GetExtension(file).ToLower();
                    return ext == ".png" || ext == ".jpg" || ext == ".jpeg";
                })
                .ToArray();

            // 处理每个图片文件
            foreach (string imagePath in imageFiles)
            {
                await ProcessImageAsync(imagePath, targetFolderPath);
                await Task.Yield(); // 让UI有机会更新
            }

            // 刷新资源数据库
            AssetDatabase.Refresh();
        }

        /// <summary>
        /// 异步处理单个图片
        /// </summary>
        private async Task ProcessImageAsync(string imagePath, string targetFolderPath)
        {
            string fileName = Path.GetFileName(imagePath);
            string targetPath = Path.Combine(targetFolderPath, fileName);

            // 检查是否已存在相同图片
            if (_settings.mergeDuplicateImages)
            {
                string existingPath = FindDuplicateImage(imagePath);
                if (!string.IsNullOrEmpty(existingPath))
                {
                    _imagePathMapping[imagePath] = existingPath;
                    return;
                }
            }

            // 复制图片到目标文件夹
            File.Copy(imagePath, targetPath, true);

            // 导入图片为Sprite
            string assetPath = targetPath.Replace(Application.dataPath, "Assets");
            AssetDatabase.ImportAsset(assetPath);

            // 设置图片导入设置
            TextureImporter importer = AssetImporter.GetAtPath(assetPath) as TextureImporter;
            if (importer != null)
            {
                importer.textureType = TextureImporterType.Sprite;
                importer.spriteImportMode = SpriteImportMode.Single;
                importer.mipmapEnabled = _settings.generateMipMaps;
                importer.filterMode = _settings.textureFilterMode;
                importer.textureCompression = _settings.compressTextures ?
                    TextureImporterCompression.Compressed :
                    TextureImporterCompression.Uncompressed;

                // 应用设置
                EditorUtility.SetDirty(importer);
                await Task.Yield(); // 让UI有机会更新
                importer.SaveAndReimport();
            }

            // 记录映射关系
            _imagePathMapping[imagePath] = assetPath;
        }

        /// <summary>
        /// 查找重复图片
        /// </summary>
        private string FindDuplicateImage(string imagePath)
        {
            // 如果已经在映射表中，直接返回
            if (_imagePathMapping.TryGetValue(imagePath, out string mappedPath))
            {
                return mappedPath;
            }

            // 读取图片数据
            byte[] imageData = File.ReadAllBytes(imagePath);

            // 遍历已处理的图片，比较MD5哈希
            foreach (var kvp in _imagePathMapping)
            {
                string existingPath = kvp.Value;
                if (existingPath.StartsWith("Assets"))
                {
                    string fullPath = Path.Combine(Application.dataPath, existingPath.Substring(7));
                    if (File.Exists(fullPath))
                    {
                        byte[] existingData = File.ReadAllBytes(fullPath);
                        if (CompareImageData(imageData, existingData))
                        {
                            return existingPath;
                        }
                    }
                }
            }

            return null;
        }

        /// <summary>
        /// 比较两个图片数据是否相同
        /// </summary>
        private bool CompareImageData(byte[] data1, byte[] data2)
        {
            if (data1.Length != data2.Length)
            {
                return false;
            }

            // 计算MD5哈希
            using (var md5 = System.Security.Cryptography.MD5.Create())
            {
                string hash1 = BitConverter.ToString(md5.ComputeHash(data1));
                string hash2 = BitConverter.ToString(md5.ComputeHash(data2));
                return hash1 == hash2;
            }
        }

        /// <summary>
        /// 获取图片的Unity资源路径
        /// </summary>
        public string GetImageAssetPath(string originalPath)
        {
            if (_imagePathMapping.TryGetValue(originalPath, out string assetPath))
            {
                return assetPath;
            }

            return null;
        }

        /// <summary>
        /// 异步加载纹理
        /// </summary>
        public async Task<Texture2D> LoadTextureAsync(string assetPath)
        {
            if (string.IsNullOrEmpty(assetPath))
            {
                return null;
            }

            // 检查缓存
            if (_textureCache.TryGetValue(assetPath, out Texture2D cachedTexture))
            {
                return cachedTexture;
            }

            // 加载纹理
            Texture2D texture = AssetDatabase.LoadAssetAtPath<Texture2D>(assetPath);
            if (texture != null)
            {
                _textureCache[assetPath] = texture;
            }

            await Task.Yield(); // 让UI有机会更新
            return texture;
        }

        /// <summary>
        /// 异步加载Sprite
        /// </summary>
        public async Task<Sprite> LoadSpriteAsync(string assetPath)
        {
            if (string.IsNullOrEmpty(assetPath))
            {
                return null;
            }

            // 检查缓存
            if (_spriteCache.TryGetValue(assetPath, out Sprite cachedSprite))
            {
                return cachedSprite;
            }

            // 加载Sprite
            Sprite sprite = AssetDatabase.LoadAssetAtPath<Sprite>(assetPath);
            if (sprite != null)
            {
                _spriteCache[assetPath] = sprite;
            }

            await Task.Yield(); // 让UI有机会更新
            return sprite;
        }

        /// <summary>
        /// 清除缓存
        /// </summary>
        public void ClearCache()
        {
            _textureCache.Clear();
            _spriteCache.Clear();
        }
    }
}

using System;
using System.IO;
using UnityEditor;
using UnityEngine;

namespace Figma2UGUI
{
    /// <summary>
    /// 可序列化的配置数据对象，用于保存和加载配置文件
    /// </summary>
    [Serializable]
    public class ConfigDataObject : ScriptableObject
    {
        public string jsonFilePath = "";
        public string sourceFolder = "";
        public string spriteFolder = "Assets/Arts/Sprites";
        public string textureFolder = "Assets/Arts/Textures";
        public string outputFolder = "Assets/Prefabs/UI";
        public string prefabName = "";
        public float uiScale = 1.0f;
        public bool createPrefab = true;
        public bool overwriteExisting = false;

        /// <summary>
        /// 创建新的配置数据对象
        /// </summary>
        public static ConfigDataObject CreateInstance()
        {
            var instance = ScriptableObject.CreateInstance<ConfigDataObject>();
            return instance;
        }

        /// <summary>
        /// 保存配置到文件
        /// </summary>
        public void SaveToFile(string filePath)
        {
            if (string.IsNullOrEmpty(filePath))
                return;

            try
            {
                // 确保目录存在
                string directory = Path.GetDirectoryName(filePath);
                if (!Directory.Exists(directory))
                {
                    Directory.CreateDirectory(directory);
                }

                // 保存资源
                AssetDatabase.CreateAsset(this, filePath);
                AssetDatabase.SaveAssets();
                AssetDatabase.Refresh();

                Debug.Log($"配置已保存到: {filePath}");
            }
            catch (Exception e)
            {
                Debug.LogError($"保存配置失败: {e.Message}");
            }
        }

        /// <summary>
        /// 从文件加载配置
        /// </summary>
        public static ConfigDataObject LoadFromFile(string filePath)
        {
            if (string.IsNullOrEmpty(filePath) || !File.Exists(filePath))
                return null;

            try
            {
                return AssetDatabase.LoadAssetAtPath<ConfigDataObject>(filePath);
            }
            catch (Exception e)
            {
                Debug.LogError($"加载配置失败: {e.Message}");
                return null;
            }
        }
    }
}

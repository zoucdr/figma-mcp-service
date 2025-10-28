using System;
using System.IO;
using System.Threading.Tasks;
using UnityEditor;
using UnityEngine;
using System.Collections.Generic;

namespace Figma2UGUI
{

    /// <summary>
    /// Figma2UGUI导入窗口
    /// </summary>
    public class UIImportWindow : EditorWindow
    {
        private string _jsonFilePath = "";
        private string _sourceFolder = "";  // 源文件夹，由JSON文件路径决定，不可见
        private string _spriteFolder = "Assets/Arts/Sprites";  // 图片拷贝目标路径
        private string _textureFolder = "Assets/Arts/Textures";  // 处理后的图片存储路径
        private string _prefabFolder = "Assets/Prefabs/UI";
        private string _prefabName = "";  // 自定义预制体名称
        private float _uiScale = 1.0f;
        private bool _createPrefab = true;
        private bool _overwriteExisting = false;
        private Vector2 _scrollPosition;
        private UINode _rootNode;
        private Texture2D _previewTexture;
        private bool _showPreview = false;
        private bool _isProcessing = false;

        // 配置文件相关
        private string _configFilePath = "Assets/ScriptableObjects/UIDefine/Config.asset";
        private ConfigDataObject _configDataObject;
        private bool _showConfigSection = false;

        // PlayerPrefs键名
        private const string PREF_CONFIG = "Figma2UGUI_Config";

        [MenuItem("Window/Figma2UGUI")]
        public static void ShowWindow()
        {
            UIImportWindow window = GetWindow<UIImportWindow>("Figma2UGUI导入");
            window.minSize = new Vector2(450, 600);
            window.Show();
        }

        private void OnEnable()
        {
            // 从PlayerPrefs加载配置
            LoadConfigFromPlayerPrefs();

            // 尝试加载配置文件
            LoadConfigFromFile();
        }

        private void OnGUI()
        {

            // 配置文件操作区域
            _showConfigSection = EditorGUILayout.Foldout(_showConfigSection, "配置文件操作");
            if (_showConfigSection)
            {
                EditorGUILayout.BeginVertical(EditorStyles.helpBox);

                // 配置文件路径
                EditorGUILayout.BeginHorizontal();
                _configFilePath = EditorGUILayout.TextField("配置文件路径", _configFilePath);
                if (GUILayout.Button("浏览", GUILayout.Width(60)))
                {
                    string path = EditorUtility.SaveFilePanelInProject("选择配置文件保存路径", "Config", "asset", "请选择配置文件保存路径");
                    if (!string.IsNullOrEmpty(path))
                    {
                        _configFilePath = path;
                    }
                }
                EditorGUILayout.EndHorizontal();

                // 保存和加载按钮
                EditorGUILayout.BeginHorizontal();
                if (GUILayout.Button("保存配置到文件", GUILayout.Height(25)))
                {
                    SaveConfigToFile();
                }

                if (GUILayout.Button("从文件加载配置", GUILayout.Height(25)))
                {
                    LoadConfigFromFile();
                }
                EditorGUILayout.EndHorizontal();

                EditorGUILayout.EndVertical();
            }

            EditorGUILayout.Space(10);

            _scrollPosition = EditorGUILayout.BeginScrollView(_scrollPosition);
            // JSON文件路径
            EditorGUILayout.LabelField("JSON文件设置", EditorStyles.boldLabel);
            EditorGUILayout.BeginHorizontal();
            EditorGUI.BeginChangeCheck();
            _jsonFilePath = EditorGUILayout.TextField("JSON文件路径", _jsonFilePath);
            if (GUILayout.Button("浏览", GUILayout.Width(60)))
            {
                string path = EditorUtility.OpenFilePanel("选择JSON文件", "", "json");
                if (!string.IsNullOrEmpty(path))
                {
                    _jsonFilePath = path;
                    LoadJsonPreview();
                    SaveConfigToPlayerPrefs();
                }
            }
            if (EditorGUI.EndChangeCheck())
            {
                SaveConfigToPlayerPrefs();
            }
            EditorGUILayout.EndHorizontal();

            // 源文件夹信息（只读）
            EditorGUI.BeginDisabledGroup(string.IsNullOrEmpty(_jsonFilePath));
            _sourceFolder = EditorGUILayout.TextField("源文件夹", _sourceFolder);
            EditorGUI.EndDisabledGroup();

            // 纹理设置
            EditorGUILayout.LabelField("纹理设置", EditorStyles.boldLabel);

            // 图片拷贝目标路径
            EditorGUILayout.BeginHorizontal();
            EditorGUI.BeginChangeCheck();
            _spriteFolder = EditorGUILayout.TextField("精灵目标路径", _spriteFolder);
            if (GUILayout.Button("浏览", GUILayout.Width(60)))
            {
                string path = EditorUtility.OpenFolderPanel("选择图片拷贝路径", "Assets", "");
                if (!string.IsNullOrEmpty(path))
                {
                    string relativePath = "Assets" + path.Substring(Application.dataPath.Length);
                    _spriteFolder = relativePath;
                    SaveConfigToPlayerPrefs();
                }
            }
            if (EditorGUI.EndChangeCheck())
            {
                SaveConfigToPlayerPrefs();
            }
            EditorGUILayout.EndHorizontal();

            // 处理后的图片存储路径
            EditorGUILayout.BeginHorizontal();
            EditorGUI.BeginChangeCheck();
            _textureFolder = EditorGUILayout.TextField("纹理目标路径", _textureFolder);
            if (GUILayout.Button("浏览", GUILayout.Width(60)))
            {
                string path = EditorUtility.OpenFolderPanel("选择纹理目标路径", "Assets", "");
                if (!string.IsNullOrEmpty(path))
                {
                    string relativePath = "Assets" + path.Substring(Application.dataPath.Length);
                    _textureFolder = relativePath;
                    SaveConfigToPlayerPrefs();
                }
            }
            if (EditorGUI.EndChangeCheck())
            {
                SaveConfigToPlayerPrefs();
            }
            EditorGUILayout.EndHorizontal();

            EditorGUILayout.HelpBox("源文件夹由JSON文件路径自动决定", MessageType.Info);

            // 输出设置
            EditorGUILayout.LabelField("输出设置", EditorStyles.boldLabel);

            EditorGUI.BeginChangeCheck();
            _uiScale = EditorGUILayout.Slider("UI缩放", _uiScale, 0.1f, 5f);
            _createPrefab = EditorGUILayout.Toggle("创建预制体", _createPrefab);

            // 只有勾选创建预制体时才显示输出文件夹选项
            if (_createPrefab)
            {
                EditorGUILayout.BeginHorizontal();
                EditorGUI.BeginChangeCheck();
                _prefabFolder = EditorGUILayout.TextField("输出预制体", _prefabFolder);
                if (GUILayout.Button("浏览", GUILayout.Width(60)))
                {
                    string path = EditorUtility.OpenFolderPanel("选择预制体输出路径", "Assets", "");
                    if (!string.IsNullOrEmpty(path))
                    {
                        string relativePath = "Assets" + path.Substring(Application.dataPath.Length);
                        _prefabFolder = relativePath;
                        SaveConfigToPlayerPrefs();
                    }
                }
                if (EditorGUI.EndChangeCheck())
                {
                    SaveConfigToPlayerPrefs();
                }
                EditorGUILayout.EndHorizontal();

                EditorGUI.BeginChangeCheck();
                _prefabName = EditorGUILayout.TextField("预制体名称", _prefabName);
                if (EditorGUI.EndChangeCheck())
                {
                    SaveConfigToPlayerPrefs();
                }

                _overwriteExisting = EditorGUILayout.Toggle("覆盖现有文件", _overwriteExisting);
            }
            if (EditorGUI.EndChangeCheck())
            {
                SaveConfigToPlayerPrefs();
            }

            EditorGUILayout.Space(10);

            // 预览
            if (_rootNode != null)
            {
                _showPreview = EditorGUILayout.Foldout(_showPreview, "JSON预览");
                if (_showPreview)
                {
                    EditorGUILayout.BeginVertical(EditorStyles.helpBox);
                    EditorGUILayout.LabelField($"根节点: {_rootNode.name}");
                    EditorGUILayout.LabelField($"子节点数量: {(_rootNode.children != null ? _rootNode.children.Length : 0)}");

                    if (_previewTexture != null)
                    {
                        GUILayout.Label(_previewTexture, GUILayout.Height(200));
                    }

                    EditorGUILayout.EndVertical();
                }
            }

            EditorGUILayout.Space(10);

            GUI.enabled = !string.IsNullOrEmpty(_jsonFilePath) && !_isProcessing;

            EditorGUILayout.BeginHorizontal();
            if (GUILayout.Button("导入界面", GUILayout.Height(30)))
            {
                ImportUI();
            }

            if (GUILayout.Button("刷新预览", GUILayout.Height(30)))
            {
                LoadJsonPreview();
            }
            EditorGUILayout.EndHorizontal();

            GUI.enabled = true;

            if (_isProcessing)
            {
                EditorGUILayout.HelpBox("正在处理...", MessageType.Info);
                Repaint();
            }

            EditorGUILayout.EndScrollView();
        }

        private void LoadJsonPreview()
        {
            if (string.IsNullOrEmpty(_jsonFilePath) || !File.Exists(_jsonFilePath))
            {
                EditorUtility.DisplayDialog("错误", "JSON文件不存在", "确定");
                return;
            }

            try
            {
                string jsonContent = File.ReadAllText(_jsonFilePath);
                _rootNode = JsonUtility.FromJson<UINode>(jsonContent);

                // 设置源文件夹为JSON文件所在目录
                _sourceFolder = Path.GetDirectoryName(_jsonFilePath);

                // 生成预览图
                GeneratePreview();
            }
            catch (Exception e)
            {
                Debug.LogError($"加载JSON预览失败: {e.Message}");
                EditorUtility.DisplayDialog("错误", $"加载JSON预览失败: {e.Message}", "确定");
            }
        }

        /// <summary>
        /// 生成预览图
        /// </summary>
        private void GeneratePreview()
        {
            if (_rootNode == null)
                return;

            // 创建一个简单的预览图，显示节点层次结构
            int width = 400;
            int height = 300;

            _previewTexture = new Texture2D(width, height);
            Color[] colors = new Color[width * height];

            // 填充背景色
            for (int i = 0; i < colors.Length; i++)
            {
                colors[i] = new Color(0.2f, 0.2f, 0.2f, 1.0f);
            }

            _previewTexture.SetPixels(colors);

            // 绘制节点结构
            DrawNodeStructure(_rootNode, 0, 0, width, height);

            _previewTexture.Apply();
        }

        /// <summary>
        /// 递归绘制节点结构
        /// </summary>
        private void DrawNodeStructure(UINode node, int level, int startY, int width, int height)
        {
            if (node == null)
                return;

            // 每个节点的高度
            int nodeHeight = 20;
            int indent = level * 10;
            int y = startY + level * nodeHeight;

            // 确保在纹理范围内
            if (y >= height)
                return;

            // 绘制节点矩形
            Color nodeColor = GetNodeColor(node.type);
            DrawRect(_previewTexture, indent, y, width - indent * 2, nodeHeight, nodeColor);

            // 递归绘制子节点
            if (node.children != null)
            {
                for (int i = 0; i < node.children.Length; i++)
                {
                    DrawNodeStructure(node.children[i], level + 1, startY, width, height);
                }
            }
        }

        /// <summary>
        /// 根据节点类型获取颜色
        /// </summary>
        private Color GetNodeColor(string nodeType)
        {
            switch (nodeType?.ToLower())
            {
                case "frame":
                    return new Color(0.2f, 0.6f, 0.9f, 1.0f);
                case "group":
                    return new Color(0.9f, 0.6f, 0.2f, 1.0f);
                case "text":
                    return new Color(0.2f, 0.9f, 0.2f, 1.0f);
                case "rectangle":
                case "ellipse":
                case "polygon":
                    return new Color(0.9f, 0.2f, 0.2f, 1.0f);
                default:
                    return new Color(0.7f, 0.7f, 0.7f, 1.0f);
            }
        }

        /// <summary>
        /// 在纹理上绘制矩形
        /// </summary>
        private void DrawRect(Texture2D texture, int x, int y, int width, int height, Color color)
        {
            for (int i = 0; i < width; i++)
            {
                for (int j = 0; j < height; j++)
                {
                    int pixelX = x + i;
                    int pixelY = y + j;

                    // 确保在纹理范围内
                    if (pixelX >= 0 && pixelX < texture.width && pixelY >= 0 && pixelY < texture.height)
                    {
                        texture.SetPixel(pixelX, pixelY, color);
                    }
                }
            }
        }

        private async void ImportUI()
        {
            if (string.IsNullOrEmpty(_jsonFilePath) || !File.Exists(_jsonFilePath))
            {
                EditorUtility.DisplayDialog("错误", "JSON文件不存在", "确定");
                return;
            }
            if (string.IsNullOrEmpty(_sourceFolder) || !Directory.Exists(_sourceFolder))
            {
                EditorUtility.DisplayDialog("错误", "源文件夹不存在，请先选择有效的JSON文件", "确定");
                return;
            }

            // 确保图片拷贝目标路径存在
            if (!string.IsNullOrEmpty(_spriteFolder) && !Directory.Exists(_spriteFolder))
            {
                Directory.CreateDirectory(_spriteFolder);
            }

            // 确保处理后图片路径存在
            if (!string.IsNullOrEmpty(_textureFolder) && !Directory.Exists(_textureFolder))
            {
                Directory.CreateDirectory(_textureFolder);
            }
            if (_createPrefab && string.IsNullOrEmpty(_prefabFolder))
            {
                EditorUtility.DisplayDialog("错误", "预制体输出路径不能为空", "确定");
                return;
            }
            try
            {
                _isProcessing = true;

                // 保存当前配置到PlayerPrefs
                SaveConfigToPlayerPrefs();

                // 如果需要创建预制体，确保输出文件夹存在
                if (_createPrefab && !Directory.Exists(_prefabFolder))
                {
                    Directory.CreateDirectory(_prefabFolder);
                }

                // 读取JSON
                string jsonContent = File.ReadAllText(_jsonFilePath);
                UINode rootNode = JsonUtility.FromJson<UINode>(jsonContent);

                if (rootNode == null)
                {
                    throw new Exception("JSON解析失败");
                }

                // 调用UGUIGenerater生成界面
                UGUIGenerater generater = new UGUIGenerater();
                await generater.GenerateUIAsync(rootNode, _sourceFolder, _spriteFolder, _textureFolder, _prefabFolder, _uiScale, _createPrefab, _overwriteExisting, _prefabName);

                _isProcessing = false;
                EditorUtility.DisplayDialog("成功", "界面导入成功", "确定");
            }
            catch (Exception e)
            {
                _isProcessing = false;
                Debug.LogException(new Exception($"导入界面失败: {e.Message}", e));
                EditorUtility.DisplayDialog("错误", $"导入界面失败: {e.Message}", "确定");
            }
        }
        /// <summary>
        /// 将当前配置保存到PlayerPrefs
        /// </summary>
        private void SaveConfigToPlayerPrefs()
        {
            // 创建配置对象并序列化为JSON
            var configData = new ConfigData
            {
                jsonFilePath = _jsonFilePath,
                sourceFolder = _sourceFolder,
                spriteFolder = _spriteFolder,
                textureFolder = _textureFolder,
                outputFolder = _prefabFolder,
                prefabName = _prefabName,
                uiScale = _uiScale,
                createPrefab = _createPrefab,
                overwriteExisting = _overwriteExisting
            };

            // 序列化为JSON并保存
            string configJson = JsonUtility.ToJson(configData);
            PlayerPrefs.SetString(PREF_CONFIG, configJson);
            PlayerPrefs.Save();
        }

        /// <summary>
        /// 保存配置到文件
        /// </summary>
        private void SaveConfigToFile()
        {
            if (string.IsNullOrEmpty(_configFilePath))
            {
                EditorUtility.DisplayDialog("错误", "请先设置配置文件路径", "确定");
                return;
            }

            try
            {
                // 创建新的配置数据对象
                _configDataObject = ConfigDataObject.CreateInstance();

                // 设置属性
                _configDataObject.jsonFilePath = _jsonFilePath;
                _configDataObject.sourceFolder = _sourceFolder;
                _configDataObject.spriteFolder = _spriteFolder;
                _configDataObject.textureFolder = _textureFolder;
                _configDataObject.outputFolder = _prefabFolder;
                _configDataObject.prefabName = _prefabName;
                _configDataObject.uiScale = _uiScale;
                _configDataObject.createPrefab = _createPrefab;
                _configDataObject.overwriteExisting = _overwriteExisting;

                // 保存到文件
                _configDataObject.SaveToFile(_configFilePath);

                EditorUtility.DisplayDialog("成功", $"配置已保存到: {_configFilePath}", "确定");
            }
            catch (Exception e)
            {
                EditorUtility.DisplayDialog("错误", $"保存配置失败: {e.Message}", "确定");
                Debug.LogException(e);
            }
        }

        /// <summary>
        /// 从PlayerPrefs加载配置
        /// </summary>
        private void LoadConfigFromPlayerPrefs()
        {
            if (PlayerPrefs.HasKey(PREF_CONFIG))
            {
                string configJson = PlayerPrefs.GetString(PREF_CONFIG);
                try
                {
                    var configData = JsonUtility.FromJson<ConfigData>(configJson);
                    if (configData != null)
                    {
                        _jsonFilePath = configData.jsonFilePath;
                        _sourceFolder = configData.sourceFolder;
                        _spriteFolder = configData.spriteFolder;
                        _textureFolder = configData.textureFolder;
                        _prefabFolder = configData.outputFolder;
                        _prefabName = configData.prefabName ?? "";
                        _uiScale = configData.uiScale;
                        _createPrefab = configData.createPrefab;
                        _overwriteExisting = configData.overwriteExisting;
                    }
                }
                catch (Exception e)
                {
                    Debug.LogWarning($"从JSON加载配置失败: {e.Message}");
                }
            }
        }

        /// <summary>
        /// 从文件加载配置
        /// </summary>
        private void LoadConfigFromFile()
        {
            if (string.IsNullOrEmpty(_configFilePath))
                return;

            try
            {
                _configDataObject = ConfigDataObject.LoadFromFile(_configFilePath);

                if (_configDataObject != null)
                {
                    // 将配置数据应用到窗口
                    _jsonFilePath = _configDataObject.jsonFilePath;
                    _sourceFolder = _configDataObject.sourceFolder;
                    _spriteFolder = _configDataObject.spriteFolder;
                    _textureFolder = _configDataObject.textureFolder;
                    _prefabFolder = _configDataObject.outputFolder;
                    _prefabName = _configDataObject.prefabName;
                    _uiScale = _configDataObject.uiScale;
                    _createPrefab = _configDataObject.createPrefab;
                    _overwriteExisting = _configDataObject.overwriteExisting;

                    Debug.Log($"从文件加载配置成功: {_configFilePath}");

                    // 加载JSON预览
                    if (!string.IsNullOrEmpty(_jsonFilePath) && File.Exists(_jsonFilePath))
                    {
                        LoadJsonPreview();
                    }
                }
                else
                {
                    Debug.Log($"配置文件不存在或为空: {_configFilePath}");
                }
            }
            catch (Exception e)
            {
                Debug.LogWarning($"从文件加载配置失败: {e.Message}");
            }
        }

        /// <summary>
        /// 用于序列化的配置数据类
        /// </summary>
        [Serializable]
        private class ConfigData
        {
            public string jsonFilePath = "";
            public string sourceFolder = "";
            public string spriteFolder = "Assets/Sprites/UI";
            public string textureFolder = "";
            public string outputFolder = "Assets/UI";
            public string prefabName = "";
            public float uiScale = 1.0f;
            public bool createPrefab = true;
            public bool overwriteExisting = false;
        }
    }
}

using System.Collections.Generic;
using UnityEditor;
using UnityEngine;
using UnityEngine.UIElements;

namespace Figma2UGUI
{
    /// <summary>
    /// 在Project Settings中添加Figma2UGUI设置界面
    /// </summary>
    public class UIImportSettingProvider : SettingsProvider
    {
        private SerializedObject _serializedObject;
        private SerializedProperty _fontMappingsProperty;
        private SerializedProperty _imageFoldersProperty;
        private SerializedProperty _largeTextureFoldersProperty;
        private SerializedProperty _defaultFontProperty;
        private SerializedProperty _defaultTMPFontProperty;
        private SerializedProperty _useTextMeshProProperty;
        private SerializedProperty _compressTexturesProperty;
        private SerializedProperty _generateMipMapsProperty;
        private SerializedProperty _textureFilterModeProperty;
        private SerializedProperty _textureFormatProperty;
        private SerializedProperty _defaultScaleProperty;
        private SerializedProperty _createPrefabProperty;
        private SerializedProperty _prefabSavePathProperty;
        private SerializedProperty _mergeDuplicateImagesProperty;
        private SerializedProperty _useImageNameFromFigmaProperty;

        private UIImportSetting _settings;
        private Editor _settingsEditor;
        private int _selectedTab = 0;
        private readonly string[] _tabNames = { "字体设置", "图片设置", "界面设置" };

        private const string SettingsPath = "Project/Figma2UGUI";

        public UIImportSettingProvider() : base(SettingsPath, SettingsScope.Project)
        {
            keywords = new HashSet<string>(new[] { "Figma", "UGUI", "Import", "UI", "Font", "Image" });
        }

        public override void OnActivate(string searchContext, VisualElement rootElement)
        {
            _settings = UIImportSetting.Instance;
            _serializedObject = new SerializedObject(_settings);
            _fontMappingsProperty = _serializedObject.FindProperty("fontMappings");
            _imageFoldersProperty = _serializedObject.FindProperty("imageFolders");
            _largeTextureFoldersProperty = _serializedObject.FindProperty("largeTextureFolders");
            _defaultFontProperty = _serializedObject.FindProperty("defaultFont");
            _defaultTMPFontProperty = _serializedObject.FindProperty("defaultTMPFont");
            _useTextMeshProProperty = _serializedObject.FindProperty("useTextMeshPro");
            _compressTexturesProperty = _serializedObject.FindProperty("compressTextures");
            _generateMipMapsProperty = _serializedObject.FindProperty("generateMipMaps");
            _textureFilterModeProperty = _serializedObject.FindProperty("textureFilterMode");
            _textureFormatProperty = _serializedObject.FindProperty("textureFormat");
            _defaultScaleProperty = _serializedObject.FindProperty("defaultScale");
            _createPrefabProperty = _serializedObject.FindProperty("createPrefab");
            _prefabSavePathProperty = _serializedObject.FindProperty("prefabSavePath");
            _mergeDuplicateImagesProperty = _serializedObject.FindProperty("mergeDuplicateImages");
            _useImageNameFromFigmaProperty = _serializedObject.FindProperty("useImageNameFromFigma");
            Editor.CreateCachedEditor(_settings, null, ref _settingsEditor);
        }

        public override void OnGUI(string searchContext)
        {
            if (_serializedObject == null)
            {
                return;
            }

            _serializedObject.Update();

            EditorGUILayout.Space();
            EditorGUILayout.LabelField("Figma2UGUI 导入设置", EditorStyles.boldLabel);
            EditorGUILayout.Space();

            // 绘制Tab栏
            _selectedTab = GUILayout.Toolbar(_selectedTab, _tabNames);

            EditorGUILayout.Space();

            // 根据选中的Tab显示不同设置
            switch (_selectedTab)
            {
                case 0:
                    DrawFontSettings();
                    break;
                case 1:
                    DrawImageSettings();
                    break;
                case 2:
                    DrawUISettings();
                    break;
            }

            // 自动应用修改
            if (_serializedObject.ApplyModifiedProperties())
            {
                // 自动保存设置
                EditorUtility.SetDirty(_settings);
                _settings.Save();
            }
        }

        /// <summary>
        /// 绘制字体设置
        /// </summary>
        private void DrawFontSettings()
        {
            EditorGUILayout.LabelField("字体映射设置", EditorStyles.boldLabel);
            EditorGUI.indentLevel++;

            EditorGUILayout.PropertyField(_useTextMeshProProperty, new GUIContent("使用TextMeshPro"));
            EditorGUILayout.PropertyField(_defaultFontProperty, new GUIContent("默认字体"));

            if (_useTextMeshProProperty.boolValue)
            {
                EditorGUILayout.PropertyField(_defaultTMPFontProperty, new GUIContent("默认TMP字体"));
            }

            EditorGUILayout.PropertyField(_fontMappingsProperty, new GUIContent("字体映射列表"), true);

            EditorGUI.indentLevel--;
        }

        /// <summary>
        /// 绘制图片设置
        /// </summary>
        private void DrawImageSettings()
        {
            // 图片文件夹设置
            EditorGUILayout.LabelField("图片文件夹设置", EditorStyles.boldLabel);
            EditorGUI.indentLevel++;

            EditorGUILayout.PropertyField(_imageFoldersProperty, new GUIContent("图片文件夹列表"), true);
            EditorGUILayout.PropertyField(_largeTextureFoldersProperty, new GUIContent("大图Texture文件夹列表"), true);
            EditorGUILayout.HelpBox("大图Texture文件夹会在加载图片时优先检查，如果找到匹配图片则使用，否则使用选择的路径中的图片", MessageType.Info);
            EditorGUILayout.PropertyField(_mergeDuplicateImagesProperty, new GUIContent("合并重复图片"));
            EditorGUILayout.PropertyField(_useImageNameFromFigmaProperty, new GUIContent("使用Figma图片名称"));

            EditorGUI.indentLevel--;
            EditorGUILayout.Space();

            // 图片导入设置
            EditorGUILayout.LabelField("图片导入设置", EditorStyles.boldLabel);
            EditorGUI.indentLevel++;

            EditorGUILayout.PropertyField(_compressTexturesProperty, new GUIContent("压缩纹理"));
            EditorGUILayout.PropertyField(_generateMipMapsProperty, new GUIContent("生成MipMaps"));
            EditorGUILayout.PropertyField(_textureFilterModeProperty, new GUIContent("过滤模式"));
            EditorGUILayout.PropertyField(_textureFormatProperty, new GUIContent("纹理格式"));

            EditorGUI.indentLevel--;
        }

        /// <summary>
        /// 绘制界面设置
        /// </summary>
        private void DrawUISettings()
        {
            EditorGUILayout.LabelField("界面生成设置", EditorStyles.boldLabel);
            EditorGUI.indentLevel++;

            EditorGUILayout.PropertyField(_defaultScaleProperty, new GUIContent("默认缩放"));
            EditorGUILayout.PropertyField(_createPrefabProperty, new GUIContent("创建预制体"));

            if (_createPrefabProperty.boolValue)
            {
                EditorGUILayout.PropertyField(_prefabSavePathProperty, new GUIContent("预制体保存路径"));
            }

            EditorGUI.indentLevel--;
        }

        [SettingsProvider]
        public static SettingsProvider CreateProvider()
        {
            return new UIImportSettingProvider();
        }
    }
}

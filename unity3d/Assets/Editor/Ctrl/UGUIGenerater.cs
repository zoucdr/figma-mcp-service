using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using UnityEditor;
using UnityEngine;
using UnityEngine.UI;
using TMPro;

namespace Figma2UGUI
{
    /// <summary>
    /// UGUI界面生成器，负责将UINode转换为UGUI界面
    /// </summary>
    public class UGUIGenerater
    {
        private Dictionary<string, Texture2D> _loadedTextures = new Dictionary<string, Texture2D>();
        private Dictionary<string, Sprite> _loadedSprites = new Dictionary<string, Sprite>();
        private Dictionary<GameObject, UINode> _gameObjectToNodeMap = new Dictionary<GameObject, UINode>();
        private UIImportSetting _settings;


        /// <summary>
        /// 获取GameObject关联的UINode
        /// </summary>
        /// <param name="gameObject">要查询的GameObject</param>
        /// <returns>关联的UINode，如果不存在则返回null</returns>
        public UINode GetUINode(GameObject gameObject)
        {
            if (gameObject == null || !_gameObjectToNodeMap.ContainsKey(gameObject))
                return null;

            return _gameObjectToNodeMap[gameObject];
        }

        public UGUIGenerater()
        {
            _settings = UIImportSetting.Instance;
        }

        /// <summary>
        /// 异步生成UI界面
        /// </summary>
        public async Task GenerateUIAsync(UINode rootNode, string sourceFolder, string spriteFolder, string textureFolder, string outputFolder, float scale, bool createPrefab, bool overwriteExisting, string prefabName = "")
        {

            // 将源文件夹中的图片复制到Sprite文件夹
            await CopyImagesToSpriteFolderAsync(sourceFolder, spriteFolder);

            // 加载所有纹理
            await LoadTexturesAsync(sourceFolder, spriteFolder, textureFolder);

            // 先查找场景中的Canvas
            Canvas canvas = GameObject.FindObjectOfType<Canvas>();
            GameObject rootGO = null;
            RectTransform rootRect = null;

            if (canvas != null)
            {
                rootGO = canvas.gameObject;
                rootRect = rootGO.GetComponent<RectTransform>();
            }
            if (!rootRect)
            {
                // 创建根GameObject
                rootGO = new GameObject("Canvas");
                rootRect = rootGO.AddComponent<RectTransform>();
                canvas = rootGO.AddComponent<Canvas>();
                canvas.renderMode = RenderMode.ScreenSpaceOverlay;
                rootGO.AddComponent<CanvasScaler>();
                rootGO.AddComponent<GraphicRaycaster>();
            }

            // 设置根节点的尺寸
            rootRect.sizeDelta = rootNode.absoluteBoundingBox.GetSize();

            // 递归生成UI
            await GenerateUIElementsAsync(rootNode, rootGO, scale);

            // 创建预制体
            if (createPrefab)
            {
                // 使用自定义预制体名称或默认使用根节点名称
                string prefabFileName = !string.IsNullOrEmpty(prefabName) ? prefabName : rootNode.name;
                string prefabPath = Path.Combine(outputFolder, $"{prefabFileName}.prefab");

                // 确保文件夹存在
                string directory = Path.GetDirectoryName(prefabPath);
                if (!Directory.Exists(directory))
                {
                    Directory.CreateDirectory(directory);
                }

                // 检查是否存在同名预制体
                if (File.Exists(prefabPath) && !overwriteExisting)
                {
                    Debug.LogError($"预制体已存在: {prefabPath}，请启用覆盖选项或使用不同的名称");
                    return;
                }
                // 不创建新对象，直接获得第一个子物体作为预制体根节点
                if (rootGO.transform.childCount == 0)
                {
                    Debug.LogError("Canvas下没有任何子节点，无法生成预制体");
                    return;
                }
                GameObject prefabRoot = rootGO.transform.GetChild(0).gameObject;
                // 在生成Prefab前，重置Prefab根节点到(0,0)，防止Canvas下Transform错乱
                RectTransform prefabRootRect = prefabRoot.GetComponent<RectTransform>();
                if (prefabRootRect != null)
                {
                    prefabRootRect.anchoredPosition = Vector2.zero;
                    prefabRootRect.localPosition = Vector3.zero;
                }

                // 创建预制体，并获取保存后的Prefab资源 
                // 使用SaveAsPrefabAsset而不是SaveAsPrefabAssetAndConnect，以避免删除场景中的对象
                GameObject prefabAsset = PrefabUtility.SaveAsPrefabAsset(
                    prefabRoot,
                    prefabPath
                );
                Debug.Log($"已创建预制体: {prefabPath}");
            }

            // 清理资源
            _loadedTextures.Clear();
            _loadedSprites.Clear();
            _gameObjectToNodeMap.Clear();
        }


        /// <summary>
        /// 将源文件夹中的图片复制到Sprite文件夹
        /// </summary>
        private async Task CopyImagesToSpriteFolderAsync(string sourceFolder, string spriteFolder)
        {
            if (string.IsNullOrEmpty(sourceFolder) || !Directory.Exists(sourceFolder) ||
                string.IsNullOrEmpty(spriteFolder))
            {
                return;
            }

            // 确保目标文件夹存在
            if (!Directory.Exists(spriteFolder))
            {
                Directory.CreateDirectory(spriteFolder);
            }

            // 获取所有图片文件
            string[] imageFiles = Directory.GetFiles(sourceFolder, "*.*", SearchOption.AllDirectories)
                .Where(file =>
                {
                    string ext = Path.GetExtension(file).ToLower();
                    return ext == ".png" || ext == ".jpg" || ext == ".jpeg";
                })
                .ToArray();

            // 复制图片到目标文件夹
            foreach (string imagePath in imageFiles)
            {
                string fileName = Path.GetFileName(imagePath);
                string destPath = Path.Combine(spriteFolder, fileName);

                // 如果目标文件已存在且启用了合并重复图片，则跳过
                if (File.Exists(destPath) && _settings.mergeDuplicateImages)
                {
                    continue;
                }
                else if (File.Exists(destPath))
                {
                    // 如果不合并重复图片，则使用唯一文件名
                    string fileNameWithoutExt = Path.GetFileNameWithoutExtension(imagePath);
                    string extension = Path.GetExtension(imagePath);
                    fileName = $"{fileNameWithoutExt}_{Guid.NewGuid().ToString().Substring(0, 8)}{extension}";
                    destPath = Path.Combine(spriteFolder, fileName);
                }

                // 复制文件
                File.Copy(imagePath, destPath, true);

                // 让UI有机会更新
                await Task.Yield();
            }

            // 刷新资源数据库，确保Unity识别新文件
            AssetDatabase.Refresh();
        }

        /// <summary>
        /// 异步加载纹理
        /// </summary>
        private async Task LoadTexturesAsync(string sourceFolder, string spriteFolder, string textureFolder)
        {
            // 首先检查大图文件夹列表
            List<string> foldersToCheck = new List<string>();

            // 添加大图文件夹列表
            if (_settings.largeTextureFolders != null && _settings.largeTextureFolders.Count > 0)
            {
                foldersToCheck.AddRange(_settings.largeTextureFolders);
            }

            // 添加源文件夹
            if (!string.IsNullOrEmpty(sourceFolder) && Directory.Exists(sourceFolder))
            {
                foldersToCheck.Add(sourceFolder);
            }

            // 添加Sprite文件夹
            if (!string.IsNullOrEmpty(spriteFolder) && Directory.Exists(spriteFolder))
            {
                foldersToCheck.Add(spriteFolder);
            }

            // 添加纹理文件夹
            if (!string.IsNullOrEmpty(textureFolder) && Directory.Exists(textureFolder))
            {
                foldersToCheck.Add(textureFolder);
            }

            // 如果没有有效的文件夹，则返回
            if (foldersToCheck.Count == 0)
            {
                return;
            }

            // 收集所有图片文件
            List<string> allImageFiles = new List<string>();
            foreach (string folder in foldersToCheck)
            {
                if (Directory.Exists(folder))
                {
                    string[] imageFiles = Directory.GetFiles(folder, "*.*", SearchOption.AllDirectories)
                        .Where(file =>
                        {
                            string ext = Path.GetExtension(file).ToLower();
                            return ext == ".png" || ext == ".jpg" || ext == ".jpeg";
                        })
                        .ToArray();

                    allImageFiles.AddRange(imageFiles);
                }
            }

            // 去重（如果有同名文件，优先使用大图文件夹中的）
            Dictionary<string, string> uniqueImageFiles = new Dictionary<string, string>();
            foreach (string imagePath in allImageFiles)
            {
                string fileName = Path.GetFileName(imagePath);

                // 如果文件名已存在，检查优先级
                if (uniqueImageFiles.ContainsKey(fileName))
                {
                    // 检查现有文件是否来自大图文件夹
                    bool existingFromLargeFolder = _settings.largeTextureFolders.Any(folder =>
                        uniqueImageFiles[fileName].StartsWith(folder));

                    // 检查新文件是否来自大图文件夹
                    bool newFromLargeFolder = _settings.largeTextureFolders.Any(folder =>
                        imagePath.StartsWith(folder));

                    // 如果新文件来自大图文件夹，而现有文件不是，则替换
                    if (newFromLargeFolder && !existingFromLargeFolder)
                    {
                        uniqueImageFiles[fileName] = imagePath;
                    }
                }
                else
                {
                    uniqueImageFiles[fileName] = imagePath;
                }
            }

            // 使用唯一的图片文件列表
            string[] uniqueImages = uniqueImageFiles.Values.ToArray();

            foreach (string imagePath in uniqueImages)
            {
                string fileName = Path.GetFileName(imagePath);

                // 检查是否已加载
                if (_loadedTextures.ContainsKey(fileName))
                {
                    if (_settings.mergeDuplicateImages)
                    {
                        continue;
                    }
                    else
                    {
                        fileName = Path.GetFileNameWithoutExtension(imagePath) + "_" + Guid.NewGuid().ToString().Substring(0, 8) + Path.GetExtension(imagePath);
                    }
                }

                // 检查文件是否是Unity资源
                bool isAssetFile = imagePath.StartsWith("Assets/") || imagePath.StartsWith(Application.dataPath);
                Texture2D texture;

                if (isAssetFile)
                {
                    // 如果是Unity资源，使用AssetDatabase加载
                    string assetPath = imagePath;
                    if (imagePath.StartsWith(Application.dataPath))
                    {
                        // 转换为相对于Assets的路径
                        assetPath = "Assets" + imagePath.Substring(Application.dataPath.Length);
                    }

                    texture = AssetDatabase.LoadAssetAtPath<Texture2D>(assetPath);
                    if (texture == null)
                    {
                        // 如果加载失败，尝试直接加载文件
                        byte[] fileData = File.ReadAllBytes(imagePath);
                        texture = new Texture2D(2, 2);
                        texture.LoadImage(fileData);
                    }
                }
                else
                {
                    // 直接从文件加载
                    byte[] fileData = File.ReadAllBytes(imagePath);
                    texture = new Texture2D(2, 2);
                    texture.LoadImage(fileData);
                }

                _loadedTextures[fileName] = texture;

                // 创建Sprite
                Sprite sprite = Sprite.Create(
                    texture,
                    new Rect(0, 0, texture.width, texture.height),
                    new Vector2(0.5f, 0.5f)
                );

                _loadedSprites[fileName] = sprite;

                // 让UI有机会更新
                await Task.Yield();
            }
        }

        /// <summary>
        /// 异步生成UI元素
        /// </summary>
        private async Task GenerateUIElementsAsync(UINode node, GameObject parent, float scale)
        {
            if (node == null)
            {
                Debug.LogError("node is null");
                return;
            }
            GameObject nodeGO = new GameObject(node.name);
            nodeGO.transform.SetParent(parent.transform, false);

            RectTransform rectTransform = nodeGO.AddComponent<RectTransform>();

            // 设置RectTransform属性
            if (node.uiModify == null)
            {
                node.uiModify = new UIModify();
            }

            // 根据absoluteBoundingBox设置默认值
            Vector2 anchoredPosition = Vector2.zero;
            Vector2 sizeDelta = Vector2.zero;

            if (node.absoluteBoundingBox != null)
            {
                // 获取尺寸
                sizeDelta = node.absoluteBoundingBox.GetSize();

                // 计算锚点位置
                // 检查是否为顶层元素（直接在Canvas下）
                if (parent.GetComponent<Canvas>() != null)
                {
                    // 顶层元素计算
                    // 获取Canvas的尺寸
                    RectTransform canvasRect = parent.GetComponent<RectTransform>();
                    float canvasWidth = canvasRect.sizeDelta.x;
                    float canvasHeight = canvasRect.sizeDelta.y;

                    // Figma坐标转换为Unity UGUI坐标
                    // X坐标：anchored_position_x = figma_x - (canvas_width/2) + (element_width/2)
                    float x = node.absoluteBoundingBox.x - (canvasWidth / 2) + (sizeDelta.x / 2);

                    // Y坐标：anchored_position_y = (canvas_height/2) - figma_y - (element_height/2)
                    float y = (canvasHeight / 2) - node.absoluteBoundingBox.y - (sizeDelta.y / 2);

                    anchoredPosition = new Vector2(x, y);
                }
                else
                {
                    // 嵌套元素计算
                    // 获取父元素的RectTransform和Figma位置信息
                    RectTransform parentRect = parent.GetComponent<RectTransform>();
                    UINode parentNode = null;
                    if (_gameObjectToNodeMap.ContainsKey(parent))
                    {
                        parentNode = _gameObjectToNodeMap[parent];
                    }

                    if (parentNode != null && parentNode.absoluteBoundingBox != null)
                    {
                        float parentWidth = parentRect.sizeDelta.x;
                        float parentHeight = parentRect.sizeDelta.y;
                        float parentFigmaX = parentNode.absoluteBoundingBox.x;
                        float parentFigmaY = parentNode.absoluteBoundingBox.y;

                        // X坐标：anchored_position_x = (figma_x - parent_figma_x) - (parent_width/2) + (element_width/2)
                        float x = (node.absoluteBoundingBox.x - parentFigmaX) - (parentWidth / 2) + (sizeDelta.x / 2);

                        // Y坐标：anchored_position_y = (parent_figma_y - figma_y) + (parent_height/2) - (element_height/2)
                        float y = (parentFigmaY - node.absoluteBoundingBox.y) + (parentHeight / 2) - (sizeDelta.y / 2);

                        anchoredPosition = new Vector2(x, y);
                    }
                    else
                    {
                        // 如果无法获取父节点信息，则使用默认计算方式
                        Vector2 figmaPosition = new Vector2(node.absoluteBoundingBox.x, node.absoluteBoundingBox.y);
                        anchoredPosition = figmaPosition;
                    }
                }
            }

            // 设置默认锚点和轴心点
            rectTransform.anchorMin = new Vector2(0.5f, 0.5f);
            rectTransform.anchorMax = new Vector2(0.5f, 0.5f);
            rectTransform.pivot = new Vector2(0.5f, 0.5f);
            rectTransform.anchoredPosition = anchoredPosition * scale;
            rectTransform.sizeDelta = sizeDelta * scale;

            // 保存当前的大小和位置信息（用于其他方法中可能需要的计算）
            LayoutElement layoutElement = nodeGO.AddComponent<LayoutElement>();
            layoutElement.preferredWidth = sizeDelta.x;
            layoutElement.preferredHeight = sizeDelta.y;

            // 存储节点引用到字典
            _gameObjectToNodeMap[nodeGO] = node;

            // 根据节点类型创建不同的UI组件
            if (node.type == "TEXT" || node.characters != null && !string.IsNullOrEmpty(node.characters))
            {
                // 创建文本组件
                if (_settings.useTextMeshPro)
                {
                    CreateTMPText(node, nodeGO, scale);
                }
                else
                {
                    CreateText(node, nodeGO, scale);
                }
            }
            else if (node.fills != null && node.fills.Length > 0)
            {
                // 创建图片组件
                CreateImage(node, nodeGO);
            }
            else
            {
                // 创建普通Panel
                CreatePanel(node, nodeGO);
            }

            // 处理特殊组件类型
            ProcessComponentType(node, nodeGO);

            // 递归处理子节点
            if (node.children != null)
            {
                foreach (UINode childNode in node.children)
                {
                    await GenerateUIElementsAsync(childNode, nodeGO, scale);

                    // 让UI有机会更新
                    await Task.Yield();
                }
            }
        }

        /// <summary>
        /// 创建普通文本
        /// </summary>
        private void CreateText(UINode node, GameObject gameObject, float scale)
        {
            // 获取或添加Text组件
            Text text = TextProcessor.GetOrAddTextComponent(gameObject);
            text.text = node.characters;

            // 应用Figma样式到Text组件
            if (node.style != null)
            {
                TextProcessor.ApplyFigmaStyleToText(text, node.style, scale, _settings);
            }

            // 设置文本颜色
            if (node.fills != null && node.fills.Length > 0 && node.fills[0].color != null)
            {
                text.color = node.fills[0].color.ToColor();
            }
            else
            {
                text.color = Color.black;
            }
        }

        /// <summary>
        /// 创建TextMeshPro文本
        /// </summary>
        private void CreateTMPText(UINode node, GameObject gameObject, float scale)
        {
            // 获取或添加TextMeshProUGUI组件
            TextMeshProUGUI text = TextProcessor.GetOrAddTMPComponent(gameObject);
            text.text = node.characters;

            // 应用Figma样式到TextMeshPro组件
            if (node.style != null)
            {
                TextProcessor.ApplyFigmaStyleToTMP(text, node.style, scale, _settings);
            }

            // 设置文本颜色
            if (node.fills != null && node.fills.Length > 0 && node.fills[0].color != null)
            {
                text.color = node.fills[0].color.ToColor();
            }
            else
            {
                text.color = Color.black;
            }
        }

        /// <summary>
        /// 创建图片
        /// </summary>
        private void CreateImage(UINode node, GameObject gameObject)
        {
            Image image = gameObject.AddComponent<Image>();

            // 查找对应的Sprite
            string imagePath = null;
            if (node.uiModify != null && !string.IsNullOrEmpty(node.uiModify.imagePath))
            {
                imagePath = node.uiModify.imagePath;
            }
            else if (node.fills != null && node.fills.Length > 0 && !string.IsNullOrEmpty(node.fills[0].imageRef))
            {
                imagePath = node.fills[0].imageRef;
            }

            if (!string.IsNullOrEmpty(imagePath) && _loadedSprites.TryGetValue(Path.GetFileName(imagePath), out Sprite sprite))
            {
                image.sprite = sprite;

                // 设置图片类型
                if (node.uiModify != null)
                {
                    switch (node.uiModify.imageType)
                    {
                        case ImageType.SIMPLE:
                            image.type = Image.Type.Simple;
                            break;
                        case ImageType.SLICED:
                            image.type = Image.Type.Sliced;
                            // 注意：border属性已被删除，需要从node中计算
                            Vector4 border = new Vector4(5, 5, 5, 5); // 默认值
                            image.sprite = CreateSlicedSprite(sprite, border);
                            break;
                        case ImageType.FILLED:
                            image.type = Image.Type.Filled;
                            break;
                        case ImageType.TILED:
                            image.type = Image.Type.Tiled;
                            break;
                    }
                }
            }
            else
            {
                // 检查是否有渐变填充
                if (node.fills != null && node.fills.Length > 0)
                {
                    Fill fill = node.fills[0];

                    // 处理渐变类型
                    if (fill.type != null && fill.type.StartsWith("GRADIENT_") &&
                        fill.gradientHandlePositions != null && fill.gradientHandlePositions.Length >= 2 &&
                        fill.gradientStops != null && fill.gradientStops.Length >= 2)
                    {
                        // 使用GradientEffectManager应用渐变效果
                        GradientEffectManager.Instance.ApplyGradientEffect(gameObject, fill);
                    }
                    // 使用纯色
                    else if (fill.color != null)
                    {
                        image.color = fill.color.ToColor();
                    }
                    else if (node.backgroundColor != null)
                    {
                        image.color = node.backgroundColor.ToColor();
                    }
                    else
                    {
                        image.color = Color.white;
                    }
                }
                else if (node.backgroundColor != null)
                {
                    image.color = node.backgroundColor.ToColor();
                }
                else
                {
                    image.color = Color.white;
                }

                // 如果有圆角，创建圆角图片
                float cornerRadius = 0;
                // 从Figma节点获取圆角信息，这里简化处理
                if (node.absoluteBoundingBox != null)
                {
                    // 可以从Figma的effects中获取圆角信息
                    // 这里使用默认值0
                    cornerRadius = 0;
                }

                if (cornerRadius > 0 && image.sprite == null)
                {
                    Vector2 size = node.absoluteBoundingBox != null ?
                        node.absoluteBoundingBox.GetSize() :
                        new Vector2(100, 100);

                    Texture2D roundedTexture = CreateRoundedRectTexture(
                        (int)size.x,
                        (int)size.y,
                        (int)cornerRadius,
                        image.color
                    );

                    Sprite roundedSprite = Sprite.Create(
                        roundedTexture,
                        new Rect(0, 0, roundedTexture.width, roundedTexture.height),
                        new Vector2(0.5f, 0.5f)
                    );

                    image.sprite = roundedSprite;
                }
            }
        }

        /// <summary>
        /// 创建面板
        /// </summary>
        private void CreatePanel(UINode node, GameObject gameObject)
        {
            Image image = gameObject.AddComponent<Image>();

            // 检查是否有渐变填充
            if (node.fills != null && node.fills.Length > 0)
            {
                Fill fill = node.fills[0];

                // 处理渐变类型
                if (fill.type != null && fill.type.StartsWith("GRADIENT_") &&
                    fill.gradientHandlePositions != null && fill.gradientHandlePositions.Length >= 2 &&
                    fill.gradientStops != null && fill.gradientStops.Length >= 2)
                {
                    // 使用GradientEffectManager应用渐变效果
                    GradientEffectManager.Instance.ApplyGradientEffect(gameObject, fill);
                }
                // 使用纯色
                else if (fill.color != null)
                {
                    image.color = fill.color.ToColor();
                }
                else if (node.backgroundColor != null)
                {
                    image.color = node.backgroundColor.ToColor();
                }
                else
                {
                    image.color = Color.white;
                }
            }
            else if (node.backgroundColor != null)
            {
                image.color = node.backgroundColor.ToColor();
            }
            else
            {
                image.color = Color.white;
            }

            // 如果有圆角，创建圆角图片
            float cornerRadius = 0;
            // 从Figma节点获取圆角信息，这里简化处理
            if (node.absoluteBoundingBox != null)
            {
                // 可以从Figma的effects中获取圆角信息
                // 这里使用默认值0
                cornerRadius = 0;
            }

            if (cornerRadius > 0 && image.sprite == null)
            {
                Vector2 size = node.absoluteBoundingBox != null ?
                    node.absoluteBoundingBox.GetSize() :
                    new Vector2(100, 100);

                Texture2D roundedTexture = CreateRoundedRectTexture(
                    (int)size.x,
                    (int)size.y,
                    (int)cornerRadius,
                    image.color
                );

                Sprite roundedSprite = Sprite.Create(
                    roundedTexture,
                    new Rect(0, 0, roundedTexture.width, roundedTexture.height),
                    new Vector2(0.5f, 0.5f)
                );

                image.sprite = roundedSprite;
            }
        }

        /// <summary>
        /// 处理组件类型
        /// </summary>
        private void ProcessComponentType(UINode node, GameObject gameObject)
        {
            if (node.uiModify == null || node.uiModify.componentType == ComponentType.NONE)
            {
                return;
            }

            switch (node.uiModify.componentType)
            {
                case ComponentType.BUTTON:
                    Button button = gameObject.AddComponent<Button>();
                    ColorBlock colors = button.colors;

                    // 设置默认颜色
                    button.colors = colors;
                    break;

                case ComponentType.TOGGLE:
                    gameObject.AddComponent<Toggle>();
                    break;

                case ComponentType.SLIDER:
                    gameObject.AddComponent<Slider>();
                    break;

                case ComponentType.INPUTFIELD:
                    if (_settings.useTextMeshPro)
                    {
                        gameObject.AddComponent<TMP_InputField>();
                    }
                    else
                    {
                        gameObject.AddComponent<InputField>();
                    }
                    break;

                case ComponentType.SCROLLVIEW:
                    gameObject.AddComponent<ScrollRect>();
                    break;

                case ComponentType.DROPDOWN:
                    if (_settings.useTextMeshPro)
                    {
                        gameObject.AddComponent<TMP_Dropdown>();
                    }
                    else
                    {
                        gameObject.AddComponent<Dropdown>();
                    }
                    break;
            }
        }

        /// <summary>
        /// 创建圆角矩形纹理
        /// </summary>
        private Texture2D CreateRoundedRectTexture(int width, int height, int radius, Color color)
        {
            Texture2D texture = new Texture2D(width, height, TextureFormat.RGBA32, false);

            // 填充透明背景
            Color[] pixels = new Color[width * height];
            for (int i = 0; i < pixels.Length; i++)
            {
                pixels[i] = Color.clear;
            }
            texture.SetPixels(pixels);

            // 填充中心区域
            for (int y = radius; y < height - radius; y++)
            {
                for (int x = 0; x < width; x++)
                {
                    texture.SetPixel(x, y, color);
                }
            }

            // 填充左右区域
            for (int y = 0; y < height; y++)
            {
                for (int x = radius; x < width - radius; x++)
                {
                    texture.SetPixel(x, y, color);
                }
            }

            // 填充四个角
            for (int y = 0; y < radius; y++)
            {
                for (int x = 0; x < radius; x++)
                {
                    float distance = Vector2.Distance(new Vector2(x, y), new Vector2(radius, radius));
                    if (distance <= radius)
                    {
                        texture.SetPixel(x, y, color); // 左下
                        texture.SetPixel(width - x - 1, y, color); // 右下
                        texture.SetPixel(x, height - y - 1, color); // 左上
                        texture.SetPixel(width - x - 1, height - y - 1, color); // 右上
                    }
                }
            }

            texture.Apply();
            return texture;
        }

        /// <summary>
        /// 创建九宫格Sprite
        /// </summary>
        private Sprite CreateSlicedSprite(Sprite originalSprite, Vector4 border)
        {
            return Sprite.Create(
                originalSprite.texture,
                originalSprite.rect,
                originalSprite.pivot,
                originalSprite.pixelsPerUnit,
                0,
                SpriteMeshType.FullRect,
                border
            );
        }
    }
}

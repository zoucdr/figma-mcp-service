using System.Collections.Generic;
using UnityEngine;
using UnityEngine.UI;

namespace Figma2UGUI
{
    /// <summary>
    /// 渐变效果管理器，用于创建和管理渐变效果
    /// </summary>
    public class GradientEffectManager
    {
        // 单例实例
        private static GradientEffectManager _instance;
        public static GradientEffectManager Instance
        {
            get
            {
                if (_instance == null)
                {
                    _instance = new GradientEffectManager();
                }
                return _instance;
            }
        }

        /// <summary>
        /// 根据填充类型应用渐变效果
        /// </summary>
        public void ApplyGradientEffect(GameObject gameObject, Fill fill)
        {
            if (fill == null || gameObject == null)
            {
                return;
            }

            // 移除现有的渐变效果
            RemoveGradientEffects(gameObject);

            // 根据填充类型添加相应的渐变效果
            switch (fill.type)
            {
                case "GRADIENT_LINEAR":
                    ApplyLinearGradient(gameObject, fill);
                    break;
                case "GRADIENT_RADIAL":
                    ApplyRadialGradient(gameObject, fill);
                    break;
                case "GRADIENT_DIAMOND":
                    ApplyDiamondGradient(gameObject, fill);
                    break;
                case "GRADIENT_ANGULAR":
                    ApplyAngularGradient(gameObject, fill);
                    break;
            }
        }

        /// <summary>
        /// 应用线性渐变效果
        /// </summary>
        private void ApplyLinearGradient(GameObject gameObject, Fill fill)
        {
            if (fill.gradientHandlePositions == null || fill.gradientHandlePositions.Length < 2 ||
                fill.gradientStops == null || fill.gradientStops.Length < 2)
            {
                return;
            }

            // 添加线性渐变效果组件
            LinearGradient linearGradient = gameObject.AddComponent<LinearGradient>();

            // 设置渐变控制点
            Vector2 start = fill.gradientHandlePositions[0].ToVector2();
            Vector2 end = fill.gradientHandlePositions[1].ToVector2();
            linearGradient.StartPoint = start;
            linearGradient.EndPoint = end;

            // 设置渐变色停点
            SetGradientStops(linearGradient, fill.gradientStops);
        }

        /// <summary>
        /// 应用径向渐变效果
        /// </summary>
        private void ApplyRadialGradient(GameObject gameObject, Fill fill)
        {
            if (fill.gradientHandlePositions == null || fill.gradientHandlePositions.Length < 3 ||
                fill.gradientStops == null || fill.gradientStops.Length < 2)
            {
                return;
            }

            // 添加径向渐变效果组件
            RadialGradient radialGradient = gameObject.AddComponent<RadialGradient>();

            // 设置渐变控制点
            Vector2 center = fill.gradientHandlePositions[0].ToVector2();
            Vector2 radius1 = fill.gradientHandlePositions[1].ToVector2() - center;
            Vector2 radius2 = fill.gradientHandlePositions[2].ToVector2() - center;

            radialGradient.Center = center;
            radialGradient.Radius1 = radius1;
            radialGradient.Radius2 = radius2;

            // 设置渐变色停点
            SetGradientStops(radialGradient, fill.gradientStops);
        }

        /// <summary>
        /// 应用菱形渐变效果
        /// </summary>
        private void ApplyDiamondGradient(GameObject gameObject, Fill fill)
        {
            if (fill.gradientHandlePositions == null || fill.gradientHandlePositions.Length < 3 ||
                fill.gradientStops == null || fill.gradientStops.Length < 2)
            {
                return;
            }

            // 添加菱形渐变效果组件
            DiamondGradient diamondGradient = gameObject.AddComponent<DiamondGradient>();

            // 设置渐变控制点
            Vector2 center = fill.gradientHandlePositions[0].ToVector2();
            Vector2 axis1 = fill.gradientHandlePositions[1].ToVector2() - center;
            Vector2 axis2 = fill.gradientHandlePositions[2].ToVector2() - center;

            diamondGradient.Center = center;
            diamondGradient.Axis1 = axis1;
            diamondGradient.Axis2 = axis2;

            // 设置渐变色停点
            SetGradientStops(diamondGradient, fill.gradientStops);
        }

        /// <summary>
        /// 应用角度渐变效果
        /// </summary>
        private void ApplyAngularGradient(GameObject gameObject, Fill fill)
        {
            if (fill.gradientHandlePositions == null || fill.gradientHandlePositions.Length < 3 ||
                fill.gradientStops == null || fill.gradientStops.Length < 2)
            {
                return;
            }

            // 添加角度渐变效果组件
            AngularGradient angularGradient = gameObject.AddComponent<AngularGradient>();

            // 设置渐变控制点
            Vector2 center = fill.gradientHandlePositions[0].ToVector2();
            Vector2 direction1 = fill.gradientHandlePositions[1].ToVector2() - center;
            Vector2 direction2 = fill.gradientHandlePositions[2].ToVector2() - center;

            angularGradient.Center = center;
            angularGradient.Direction1 = direction1.normalized;
            angularGradient.Direction2 = direction2.normalized;

            // 设置渐变色停点
            SetGradientStops(angularGradient, fill.gradientStops);
        }

        /// <summary>
        /// 设置渐变色停点
        /// </summary>
        private void SetGradientStops(BaseGradient gradient, GradientStops[] stops)
        {
            if (stops == null || stops.Length < 2)
            {
                return;
            }

            // 最多支持8个色停
            int count = Mathf.Min(stops.Length, 8);

            // 创建色停数组
            GradientColorKey[] colorKeys = new GradientColorKey[count];

            // 设置色停位置和颜色
            for (int i = 0; i < count; i++)
            {
                Color color = stops[i].color.ToColor();
                colorKeys[i] = new GradientColorKey(stops[i].position, color);
            }

            // 设置渐变色停点
            gradient.SetColorKeys(colorKeys);
        }

        /// <summary>
        /// 移除所有渐变效果
        /// </summary>
        private void RemoveGradientEffects(GameObject gameObject)
        {
            // 移除线性渐变
            LinearGradient linearGradient = gameObject.GetComponent<LinearGradient>();
            if (linearGradient != null)
            {
                Object.DestroyImmediate(linearGradient);
            }

            // 移除径向渐变
            RadialGradient radialGradient = gameObject.GetComponent<RadialGradient>();
            if (radialGradient != null)
            {
                Object.DestroyImmediate(radialGradient);
            }

            // 移除菱形渐变
            DiamondGradient diamondGradient = gameObject.GetComponent<DiamondGradient>();
            if (diamondGradient != null)
            {
                Object.DestroyImmediate(diamondGradient);
            }

            // 移除角度渐变
            AngularGradient angularGradient = gameObject.GetComponent<AngularGradient>();
            if (angularGradient != null)
            {
                Object.DestroyImmediate(angularGradient);
            }
        }
    }
}

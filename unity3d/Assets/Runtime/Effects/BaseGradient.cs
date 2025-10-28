using System.Collections.Generic;
using UnityEngine;
using UnityEngine.UI;

namespace Figma2UGUI
{
    /// <summary>
    /// 渐变色停点
    /// </summary>
    [System.Serializable]
    public struct GradientColorKey
    {
        public float position;
        public Color color;

        public GradientColorKey(float position, Color color)
        {
            this.position = position;
            this.color = color;
        }
    }

    /// <summary>
    /// 基础渐变效果类
    /// </summary>
    [RequireComponent(typeof(Graphic))]
    public abstract class BaseGradient : BaseMeshEffect
    {
        [SerializeField]
        protected List<GradientColorKey> m_ColorKeys = new List<GradientColorKey>();

        /// <summary>
        /// 设置渐变色停点
        /// </summary>
        public void SetColorKeys(GradientColorKey[] colorKeys)
        {
            m_ColorKeys.Clear();

            if (colorKeys != null)
            {
                m_ColorKeys.AddRange(colorKeys);
            }

            // 确保图形组件更新
            if (graphic != null)
            {
                graphic.SetVerticesDirty();
            }
        }

        /// <summary>
        /// 获取指定位置的颜色
        /// </summary>
        protected Color EvaluateColor(float position)
        {
            if (m_ColorKeys.Count == 0)
            {
                return Color.white;
            }

            if (m_ColorKeys.Count == 1)
            {
                return m_ColorKeys[0].color;
            }

            // 确保位置在0-1范围内
            position = Mathf.Clamp01(position);

            // 找到位置所在的色停区间
            for (int i = 0; i < m_ColorKeys.Count - 1; i++)
            {
                if (position >= m_ColorKeys[i].position && position <= m_ColorKeys[i + 1].position)
                {
                    // 计算在区间内的插值因子
                    float t = Mathf.InverseLerp(m_ColorKeys[i].position, m_ColorKeys[i + 1].position, position);

                    // 返回插值颜色
                    return Color.Lerp(m_ColorKeys[i].color, m_ColorKeys[i + 1].color, t);
                }
            }

            // 如果位置小于第一个色停，返回第一个色停的颜色
            if (position < m_ColorKeys[0].position)
            {
                return m_ColorKeys[0].color;
            }

            // 如果位置大于最后一个色停，返回最后一个色停的颜色
            return m_ColorKeys[m_ColorKeys.Count - 1].color;
        }

        /// <summary>
        /// 计算渐变位置并应用到顶点颜色
        /// </summary>
        public override void ModifyMesh(VertexHelper vh)
        {
            if (!IsActive() || vh.currentVertCount == 0 || m_ColorKeys.Count < 2)
            {
                return;
            }

            // 获取所有顶点
            List<UIVertex> vertices = new List<UIVertex>();
            vh.GetUIVertexStream(vertices);

            // 修改顶点颜色
            for (int i = 0; i < vertices.Count; i++)
            {
                UIVertex vertex = vertices[i];

                // 计算顶点的渐变位置 (0-1)
                float gradientPosition = CalculateGradientPosition(vertex.position);

                // 根据渐变位置获取颜色
                Color color = EvaluateColor(gradientPosition);

                // 应用颜色到顶点，保留原始Alpha
                vertex.color = new Color(color.r, color.g, color.b, vertex.color.a * color.a);

                // 更新顶点
                vertices[i] = vertex;
            }

            // 清空并重新填充顶点辅助器
            vh.Clear();
            vh.AddUIVertexTriangleStream(vertices);
        }

        /// <summary>
        /// 计算顶点在渐变中的位置 (0-1)
        /// 子类必须实现此方法
        /// </summary>
        protected abstract float CalculateGradientPosition(Vector3 vertexPosition);
    }
}

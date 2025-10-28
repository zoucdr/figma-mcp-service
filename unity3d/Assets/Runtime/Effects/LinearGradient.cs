using UnityEngine;
using UnityEngine.UI;

namespace Figma2UGUI
{
    /// <summary>
    /// 线性渐变效果
    /// </summary>
    [AddComponentMenu("UI/Effects/Linear Gradient")]
    public class LinearGradient : BaseGradient
    {
        [SerializeField]
        private Vector2 m_StartPoint = new Vector2(0, 0);

        [SerializeField]
        private Vector2 m_EndPoint = new Vector2(1, 0);

        /// <summary>
        /// 渐变起点
        /// </summary>
        public Vector2 StartPoint
        {
            get { return m_StartPoint; }
            set
            {
                m_StartPoint = value;
                if (graphic != null)
                {
                    graphic.SetVerticesDirty();
                }
            }
        }

        /// <summary>
        /// 渐变终点
        /// </summary>
        public Vector2 EndPoint
        {
            get { return m_EndPoint; }
            set
            {
                m_EndPoint = value;
                if (graphic != null)
                {
                    graphic.SetVerticesDirty();
                }
            }
        }

        /// <summary>
        /// 计算顶点在线性渐变中的位置 (0-1)
        /// </summary>
        protected override float CalculateGradientPosition(Vector3 vertexPosition)
        {
            // 将顶点位置转换为局部空间中的2D坐标
            Vector2 point = new Vector2(vertexPosition.x, vertexPosition.y);

            // 计算方向向量
            Vector2 direction = m_EndPoint - m_StartPoint;

            // 计算顶点到起点的向量
            Vector2 toPoint = point - m_StartPoint;

            // 计算投影长度
            float projectionLength = Vector2.Dot(toPoint, direction.normalized);

            // 计算总长度
            float totalLength = direction.magnitude;

            // 计算相对位置 (0-1)
            float position = projectionLength / totalLength;

            // 确保位置在0-1范围内
            return Mathf.Clamp01(position);
        }
    }
}

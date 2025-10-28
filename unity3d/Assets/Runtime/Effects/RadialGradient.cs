using UnityEngine;
using UnityEngine.UI;

namespace Figma2UGUI
{
    /// <summary>
    /// 径向渐变效果
    /// </summary>
    [AddComponentMenu("UI/Effects/Radial Gradient")]
    public class RadialGradient : BaseGradient
    {
        [SerializeField]
        private Vector2 m_Center = new Vector2(0.5f, 0.5f);

        [SerializeField]
        private Vector2 m_Radius1 = new Vector2(0.5f, 0);

        [SerializeField]
        private Vector2 m_Radius2 = new Vector2(0, 0.5f);

        /// <summary>
        /// 渐变中心点
        /// </summary>
        public Vector2 Center
        {
            get { return m_Center; }
            set
            {
                m_Center = value;
                if (graphic != null)
                {
                    graphic.SetVerticesDirty();
                }
            }
        }

        /// <summary>
        /// 渐变半径1
        /// </summary>
        public Vector2 Radius1
        {
            get { return m_Radius1; }
            set
            {
                m_Radius1 = value;
                if (graphic != null)
                {
                    graphic.SetVerticesDirty();
                }
            }
        }

        /// <summary>
        /// 渐变半径2
        /// </summary>
        public Vector2 Radius2
        {
            get { return m_Radius2; }
            set
            {
                m_Radius2 = value;
                if (graphic != null)
                {
                    graphic.SetVerticesDirty();
                }
            }
        }

        /// <summary>
        /// 计算顶点在径向渐变中的位置 (0-1)
        /// </summary>
        protected override float CalculateGradientPosition(Vector3 vertexPosition)
        {
            // 将顶点位置转换为局部空间中的2D坐标
            Vector2 point = new Vector2(vertexPosition.x, vertexPosition.y);

            // 计算顶点到中心的向量
            Vector2 toPoint = point - m_Center;

            // 计算在两个半径方向上的投影长度
            float progress1 = CalcProgress(toPoint, m_Radius1);
            float progress2 = CalcProgress(toPoint, m_Radius2);

            // 计算径向距离
            float distance = Mathf.Sqrt(progress1 * progress1 + progress2 * progress2);

            // 确保位置在0-1范围内
            return Mathf.Clamp01(distance);
        }

        /// <summary>
        /// 计算向量在指定方向上的投影比例
        /// </summary>
        private float CalcProgress(Vector2 point, Vector2 direction)
        {
            if (direction.sqrMagnitude < 0.0001f)
            {
                return 0;
            }

            float dot = Vector2.Dot(point, direction);
            return ((dot / direction.sqrMagnitude) * direction).magnitude / direction.magnitude;
        }
    }
}

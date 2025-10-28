using UnityEngine;
using UnityEngine.UI;

namespace Figma2UGUI
{
    /// <summary>
    /// 菱形渐变效果
    /// </summary>
    [AddComponentMenu("UI/Effects/Diamond Gradient")]
    public class DiamondGradient : BaseGradient
    {
        [SerializeField]
        private Vector2 m_Center = new Vector2(0.5f, 0.5f);

        [SerializeField]
        private Vector2 m_Axis1 = new Vector2(0.5f, 0);

        [SerializeField]
        private Vector2 m_Axis2 = new Vector2(0, 0.5f);

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
        /// 菱形轴1
        /// </summary>
        public Vector2 Axis1
        {
            get { return m_Axis1; }
            set
            {
                m_Axis1 = value;
                if (graphic != null)
                {
                    graphic.SetVerticesDirty();
                }
            }
        }

        /// <summary>
        /// 菱形轴2
        /// </summary>
        public Vector2 Axis2
        {
            get { return m_Axis2; }
            set
            {
                m_Axis2 = value;
                if (graphic != null)
                {
                    graphic.SetVerticesDirty();
                }
            }
        }

        /// <summary>
        /// 计算顶点在菱形渐变中的位置 (0-1)
        /// </summary>
        protected override float CalculateGradientPosition(Vector3 vertexPosition)
        {
            // 将顶点位置转换为局部空间中的2D坐标
            Vector2 point = new Vector2(vertexPosition.x, vertexPosition.y);

            // 计算顶点到中心的向量
            Vector2 toPoint = point - m_Center;

            // 计算在两个轴向上的投影长度
            float progress1 = CalcProgress(toPoint, m_Axis1);
            float progress2 = CalcProgress(toPoint, m_Axis2);

            // 菱形渐变是两个投影长度的和
            float distance = progress1 + progress2;

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
            return Mathf.Abs(((dot / direction.sqrMagnitude) * direction).magnitude / direction.magnitude);
        }
    }
}
